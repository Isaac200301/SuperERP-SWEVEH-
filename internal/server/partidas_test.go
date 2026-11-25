package server_test

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
)

// R5 (V-07, HU-105, ADR-0040): cotización por partidas con detalle interno. Datos Ficticio.

// quotedOrder deja una orden en diagnóstico, lista para cotizar, y devuelve la ruta de su cotización.
func quotedOrder(t *testing.T, c *client, phone, plate string) string {
	t.Helper()
	r := c.must(http.StatusCreated, c.do("POST", "/api/v1/customers", map[string]any{"celular": phone, "nombre": "Cliente " + plate}))
	c.must(http.StatusCreated, c.do("POST", "/api/v1/vehicles", map[string]any{"placa": plate, "clienteId": nested(r.body, "cliente", "id"),
		"cilindraje": "1600", "turbo": false, "energia": "gasolina", "transmision": "manual"}))
	id := c.must(http.StatusCreated, c.do("POST", "/api/v1/orders", map[string]any{"placa": plate, "sintoma": "Restauración", "estadoInicial": "recibido"})).str("id")
	c.must(http.StatusOK, c.do("POST", "/api/v1/orders/"+id+"/transition", map[string]any{"estado": "en_diagnostico", "version": r2v(c, id)}))
	return "/api/v1/orders/" + id + "/quote"
}

// qv es la versión de la cotización que tiene la pantalla (If-Match de las ediciones, R-07).
func qv(c *client, q string) float64 {
	return cot(c.must(http.StatusOK, c.do("GET", q, nil))).body["version"].(float64)
}

func TestPartidas_CRUDDetailAndTotals(t *testing.T) {
	e := newEnv(t)
	c := e.register("3105550510", "Rosa Pérez")
	svc := c.must(http.StatusCreated, c.do("POST", "/api/v1/services", map[string]any{"nombre": "Alineación", "tipoVehiculo": "carro", "precio": 60000})).str("id")
	q := quotedOrder(t, c, "3005550511", "PAR511")

	// Sin valor no hay partida: el valor lo pone el taller (RF-17).
	if r := c.do("POST", q+"/lines", map[string]any{"tipo": "partida", "descripcion": "Frenos completos", "cantidad": 1}); r.str("code") != "partida_valor" {
		t.Fatalf("partida sin valor: %d %v", r.status, r.body)
	}
	ls := lines(c.must(http.StatusOK, c.do("POST", q+"/lines", map[string]any{"tipo": "partida", "descripcion": "Frenos completos", "cantidad": 1, "precioUnitario": 480000})))
	pid := ls[0]["id"].(string)
	if ls[0]["tipo"] != "partida" || ls[0]["precioLibre"] != true || len(ls[0]["detalle"].([]any)) != 0 {
		t.Fatalf("partida: %v", ls[0])
	}
	c.must(http.StatusOK, c.do("POST", q+"/lines", map[string]any{"tipo": "servicio", "refId": svc, "cantidad": 1}))

	// Detalle interno: no cambia el total (480.000 + 60.000; IVA incluido por defecto).
	if r := c.do("PUT", q+"/lines/"+pid+"/detail", map[string]any{"detalle": []map[string]any{}}); r.str("code") != "version_requerida" {
		t.Fatalf("sin versión: %d %v", r.status, r.body)
	}
	raw := c.must(http.StatusOK, c.do("PUT", q+"/lines/"+pid+"/detail", map[string]any{"version": qv(c, q), "detalle": []map[string]any{
		{"descripcion": "Pastillas delanteras", "cantidad": 2, "valorRef": 130000}, {"descripcion": "Rectificar discos"}}}))
	r := cot(raw)
	if r.body["total"] != float64(540000) {
		t.Fatalf("el detalle no debía cambiar el total: %v", r.body)
	}
	det := lines(raw)[0]["detalle"].([]any)
	if len(det) != 2 || det[1].(map[string]any)["cantidad"] != float64(1) || det[0].(map[string]any)["valorRef"] != float64(130000) {
		t.Fatalf("detalle: %v", det)
	}
	if r := c.do("PUT", q+"/lines/"+lines(raw)[1]["id"].(string)+"/detail", map[string]any{"version": qv(c, q), "detalle": []map[string]any{{"descripcion": "x"}}}); r.str("code") != "no_es_partida" {
		t.Fatalf("detalle en un servicio: %d %v", r.status, r.body)
	}

	// Nombre y valor se cambian; descuento con el tope de siempre.
	if r := c.do("PATCH", q+"/lines/"+pid, map[string]any{"descripcion": "Sin versión"}); r.status != http.StatusConflict {
		t.Fatalf("renombrar sin versión: %d %v", r.status, r.body)
	}
	raw = c.must(http.StatusOK, c.do("PATCH", q+"/lines/"+pid, map[string]any{"descripcion": "Frenos completos y líquido", "precioUnitario": 500000, "version": qv(c, q)}))
	if l := lines(raw)[0]; l["descripcion"] != "Frenos completos y líquido" || l["precioUnitario"] != float64(500000) || len(l["detalle"].([]any)) != 2 {
		t.Fatalf("cambio: %v", l)
	}
	r = cot(c.must(http.StatusOK, c.do("PATCH", q+"/lines/"+pid, map[string]any{"descuentoPct": 10})))
	if r.body["total"] != float64(510000) { // 500.000 − 50.000 + 60.000
		t.Fatalf("descuento: %v", r.body)
	}
	is4xx(t, "descuento sobre el tope", c.do("PATCH", q+"/lines/"+pid, map[string]any{"descuentoPct": 11}))

	// "Mostrar detalle al cliente": se guarda y queda en la bitácora.
	r = cot(c.must(http.StatusOK, c.do("PATCH", q, map[string]any{"mostrarDetalle": true, "version": qv(c, q)})))
	if r.body["mostrarDetalle"] != true {
		t.Fatalf("mostrar detalle: %v", r.body)
	}
	again := c.must(http.StatusOK, c.do("GET", q, nil))
	if cot(again).body["mostrarDetalle"] != true || len(lines(again)[0]["detalle"].([]any)) != 2 {
		t.Fatalf("al leer de nuevo: %v", again.body)
	}
	var n int
	if err := e.asTaller(tallerOf(t, c), `SELECT count(*) FROM audit.evento WHERE accion IN ('cotizacion.detalle_partida', 'cotizacion.mostrar_detalle')`, &n); err != nil || n != 2 {
		t.Fatalf("bitácora: %d %v", n, err)
	}

	// Enviada, ya no se edita (lo que vio el cliente no cambia; para más, la adicional de HU-31).
	c.must(http.StatusOK, c.do("POST", q+"/send", nil))
	is4xx(t, "detalle enviada", c.do("PUT", q+"/lines/"+pid+"/detail", map[string]any{"version": qv(c, q), "detalle": []map[string]any{}}))
	is4xx(t, "mostrar detalle enviada", c.do("PATCH", q, map[string]any{"mostrarDetalle": false, "version": qv(c, q)}))

	// Aislamiento (CA-10): otro taller no ve la cotización ni el detalle.
	other := e.register("3105550512", "Otro Taller")
	if r := other.do("GET", q, nil); r.status != http.StatusNotFound {
		t.Fatalf("otro taller ve la cotización: %d", r.status)
	}
	var visible int
	if err := e.asTaller(tallerOf(t, other), `SELECT count(*) FROM quotes.partida_detalle`, &visible); err != nil || visible != 0 {
		t.Fatalf("otro taller ve %d renglones del detalle (%v)", visible, err)
	}
	var mine int
	if err := e.asTaller(tallerOf(t, c), `SELECT count(*) FROM quotes.partida_detalle`, &mine); err != nil || mine != 2 {
		t.Fatalf("detalle propio: %d %v", mine, err)
	}
}

// R-07 (hallazgo del revisor en R5): una pantalla con datos viejos no pisa lo que guardó otra persona. A y B leen la
// misma versión; B guarda; A recibe 409 y el detalle de B queda.
func TestPartidas_StaleScreenGets409(t *testing.T) {
	e := newEnv(t)
	a := e.register("3105550515", "Rosa Pérez")
	q := quotedOrder(t, a, "3005550516", "PAR516")
	pid := lines(a.must(http.StatusOK, a.do("POST", q+"/lines", map[string]any{"tipo": "partida", "descripcion": "Pintura", "cantidad": 1, "precioUnitario": 700000})))[0]["id"].(string)
	b := e.client()
	copyCookies(t, a, b) // la misma cuenta en otro celular: otra pantalla
	v := qv(a, q)
	b.must(http.StatusOK, b.do("PUT", q+"/lines/"+pid+"/detail", map[string]any{"version": v, "detalle": []map[string]any{{"descripcion": "Lo de B"}}}))
	if r := a.do("PUT", q+"/lines/"+pid+"/detail", map[string]any{"version": v, "detalle": []map[string]any{{"descripcion": "Lo de A"}}}); r.status != http.StatusConflict || r.str("code") != "version_conflicto" {
		t.Fatalf("pantalla vieja: %d %v", r.status, r.body)
	}
	if r := a.do("PATCH", q, map[string]any{"mostrarDetalle": true, "version": v}); r.status != http.StatusConflict {
		t.Fatalf("mostrar detalle con pantalla vieja: %d", r.status)
	}
	if r := a.do("PATCH", q+"/lines/"+pid, map[string]any{"descripcion": "Otro nombre", "version": v}); r.status != http.StatusConflict {
		t.Fatalf("renombrar con pantalla vieja: %d", r.status)
	}
	det := lines(a.must(http.StatusOK, a.do("GET", q, nil)))[0]["detalle"].([]any)
	if len(det) != 1 || det[0].(map[string]any)["descripcion"] != "Lo de B" {
		t.Fatalf("el detalle de B debía quedar: %v", det)
	}
}

// R-07: varias personas guardan el detalle a la vez con la misma versión: gana exactamente una y las demás reciben 409;
// el detalle nunca queda mezclado.
func TestPartidas_ConcurrentDetailEdits(t *testing.T) {
	e := newEnv(t)
	c := e.register("3105550513", "Rosa Pérez")
	q := quotedOrder(t, c, "3005550514", "PAR514")
	pid := lines(c.must(http.StatusOK, c.do("POST", q+"/lines", map[string]any{"tipo": "partida", "descripcion": "Latonería", "cantidad": 1, "precioUnitario": 900000})))[0]["id"].(string)
	var wg sync.WaitGroup
	statuses := make(chan int, 6)
	v := qv(c, q)
	for i := range 6 {
		wg.Go(func() {
			items := []map[string]any{}
			for j := range i + 1 {
				items = append(items, map[string]any{"descripcion": fmt.Sprintf("Paso %d de %d", j+1, i+1)})
			}
			statuses <- c.do("PUT", q+"/lines/"+pid+"/detail", map[string]any{"version": v, "detalle": items}).status
		})
	}
	wg.Wait()
	close(statuses)
	ok := 0
	for s := range statuses {
		switch s {
		case http.StatusOK:
			ok++
		case http.StatusConflict:
		default:
			t.Fatalf("estado inesperado: %d", s)
		}
	}
	if ok != 1 {
		t.Fatalf("debía ganar exactamente una, ganaron %d", ok)
	}
	det := lines(c.must(http.StatusOK, c.do("GET", q, nil)))[0]["detalle"].([]any)
	last := det[len(det)-1].(map[string]any)["descripcion"].(string)
	if want := fmt.Sprintf("Paso %d de %d", len(det), len(det)); last != want {
		t.Fatalf("el detalle quedó mezclado: %v", det)
	}
}
