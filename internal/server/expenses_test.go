package server_test

import (
	"net/http"
	"strings"
	"testing"

	"sweveh/internal/shared"
)

// R6 (V-08, V-09, HU-106, ADR-0040): lo gastado contra lo cotizado. Datos Ficticio.

// approvedOrder deja una orden con una cotización aprobada de 100.000 (servicio de lista, IVA incluido por defecto).
func approvedOrder(t *testing.T, c *client, phone, plate string) (orderPath string) {
	t.Helper()
	svc := c.must(http.StatusCreated, c.do("POST", "/api/v1/services", map[string]any{"nombre": "Revisión " + plate, "tipoVehiculo": "carro", "precio": 100000})).str("id")
	q := quotedOrder(t, c, phone, plate)
	c.must(http.StatusOK, c.do("POST", q+"/lines", map[string]any{"tipo": "servicio", "refId": svc, "cantidad": 1}))
	c.must(http.StatusOK, c.do("POST", q+"/send", nil))
	c.must(http.StatusOK, c.do("POST", q+"/response", map[string]any{"decision": "aprobar", "canal": "mostrador"}))
	return strings.TrimSuffix(q, "/quote")
}

func TestExpenses_AddVoidSummaryAndIdempotency(t *testing.T) {
	e := newEnv(t)
	c := e.register("3105550610", "Rosa Pérez")
	base := approvedOrder(t, c, "3005550611", "GAS611")
	exp := base + "/expenses"

	r := c.must(http.StatusOK, c.do("GET", exp, nil))
	if res := r.body["resumen"].(map[string]any); res["cotizado"] != float64(100000) || res["gastado"] != float64(0) || res["quedan"] != float64(100000) {
		t.Fatalf("resumen inicial: %v", res)
	}
	// Validaciones: sin id, sin valor o con un origen inventado no se anota.
	is4xx(t, "sin id", c.do("POST", exp, map[string]any{"descripcion": "Pastillas", "valor": 40000}))
	is4xx(t, "valor cero", c.do("POST", exp, map[string]any{"id": shared.NewID().String(), "descripcion": "Pastillas", "valor": 0}))
	is4xx(t, "origen", c.do("POST", exp, map[string]any{"id": shared.NewID().String(), "descripcion": "Pastillas", "valor": 1, "origen": "magia"}))

	// Idempotente: el mismo id dos veces anota una sola vez.
	g1 := shared.NewID().String()
	body := map[string]any{"id": g1, "descripcion": "Pastillas delanteras", "valor": 60000}
	c.must(http.StatusCreated, c.do("POST", exp, body))
	r = c.must(http.StatusCreated, c.do("POST", exp, body))
	if res := r.body["resumen"].(map[string]any); res["gastado"] != float64(60000) {
		t.Fatalf("el mismo envío dos veces: %v", res)
	}

	// Con la foto de la factura (una evidencia de la orden); una de otra orden no sirve.
	c.must(http.StatusCreated, c.uploadPhotos(base+"/evidence", map[string]string{"nota": "Factura del almacén"}, 1))
	fotoID := c.must(http.StatusOK, c.do("GET", base+"/evidence", nil)).body["evidencias"].([]any)[0].(map[string]any)["id"].(string)
	g2 := shared.NewID().String()
	r = c.must(http.StatusCreated, c.do("POST", exp, map[string]any{"id": g2, "descripcion": "Discos", "valor": 55000, "origen": "factura", "fotoId": fotoID}))
	if res := r.body["resumen"].(map[string]any); res["gastado"] != float64(115000) || res["quedan"] != float64(-15000) {
		t.Fatalf("se pasó de lo cotizado: %v", res)
	}
	otra := approvedOrder(t, c, "3005550612", "GAS612")
	if r := c.do("POST", otra+"/expenses", map[string]any{"id": shared.NewID().String(), "descripcion": "x", "valor": 1, "fotoId": fotoID}); r.str("code") != "gasto_foto" {
		t.Fatalf("foto de otra orden: %d %v", r.status, r.body)
	}

	// KPI 5: 1 de 1 orden abierta con gastos va por encima de lo aprobado.
	k := kpi(t, c.must(http.StatusOK, c.do("GET", "/api/v1/dashboard", nil)).body, "gastado")
	if k["texto"] != "1 de 1 por encima" || !strings.Contains(k["detalle"].(string), "$115.000 gastado de $100.000 cotizado") {
		t.Fatalf("KPI 5: %v", k)
	}

	// Anular: no se borra, sale de la suma; anular otra vez no cambia nada ni repite la bitácora.
	c.must(http.StatusOK, c.do("POST", exp+"/"+g2+"/void", nil))
	r = c.must(http.StatusOK, c.do("POST", exp+"/"+g2+"/void", nil))
	if res := r.body["resumen"].(map[string]any); res["gastado"] != float64(60000) {
		t.Fatalf("anulado: %v", res)
	}
	list := c.must(http.StatusOK, c.do("GET", exp, nil)).body["gastos"].([]any)
	if len(list) != 2 || list[1].(map[string]any)["anulado"] != true || list[1].(map[string]any)["fotoId"] != fotoID {
		t.Fatalf("gastos: %v", list)
	}
	is4xx(t, "anular uno que no existe", c.do("POST", exp+"/"+shared.NewID().String()+"/void", nil))

	var anotar, anular int
	tid := tallerOf(t, c)
	if err := e.asTaller(tid, `SELECT count(*) FROM audit.evento WHERE accion = 'orden.gasto.anotar'`, &anotar); err != nil || anotar != 2 {
		t.Fatalf("bitácora anotar: %d %v", anotar, err)
	}
	if err := e.asTaller(tid, `SELECT count(*) FROM audit.evento WHERE accion = 'orden.gasto.anular'`, &anular); err != nil || anular != 1 {
		t.Fatalf("bitácora anular: %d %v", anular, err)
	}

	// Aislamiento (CA-10): otro taller no ve ni anota gastos de esta orden.
	other := e.register("3105550613", "Otro Taller")
	if r := other.do("GET", exp, nil); r.status != http.StatusNotFound {
		t.Fatalf("otro taller ve los gastos: %d", r.status)
	}
	if r := other.do("POST", exp, map[string]any{"id": shared.NewID().String(), "descripcion": "x", "valor": 1}); r.status != http.StatusNotFound {
		t.Fatalf("otro taller anota: %d", r.status)
	}
	var visible int
	if err := e.asTaller(tallerOf(t, other), `SELECT count(*) FROM orders.gasto`, &visible); err != nil || visible != 0 {
		t.Fatalf("otro taller ve %d gastos (%v)", visible, err)
	}
}

// V-05 y V-08: el mecánico anota gastos (expense:manage es de los 3 roles); el KPI de lo gastado no lo ve (kpi:read).
func TestExpenses_MechanicAnnotates(t *testing.T) {
	e := newEnv(t)
	owner := e.register("3105550614", "Rosa Pérez")
	owner.must(http.StatusCreated, owner.do("POST", "/api/v1/users", map[string]any{"celular": "3005550615", "nombre": "Jhon Mecánico", "rol": "mecanico"}))
	base := approvedOrder(t, owner, "3005550616", "GAS616")
	mec := e.login("3005550615")
	mec.must(http.StatusCreated, mec.do("POST", base+"/expenses", map[string]any{"id": shared.NewID().String(), "descripcion": "Líquido de frenos", "valor": 25000}))
	if r := mec.do("GET", "/api/v1/dashboard", nil); r.status != http.StatusForbidden {
		t.Fatalf("el mecánico no ve el tablero: %d", r.status)
	}
}
