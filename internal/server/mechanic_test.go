package server_test

// HU-23 y HU-81: el mecánico entra con su celular, ve sus órdenes y no ve dinero ni configuración (RF-21).
// Datos Ficticio.

import (
	"net/http"
	"strings"
	"testing"
)

// login entra con un celular que ya es usuario de un taller.
func (e *env) login(phone10 string) *client {
	e.t.Helper()
	c := e.client()
	c.must(http.StatusAccepted, c.do("POST", "/api/v1/auth/otp", map[string]any{"celular": phone10}))
	c.must(http.StatusOK, c.do("POST", "/api/v1/auth/otp/verify", map[string]any{"celular": phone10, "codigo": e.otp.Last("+57" + phone10)}))
	return c
}

// V-05 (ADR-0038, cambia RF-21 y HU-81): un solo panel. El mecánico ve sus órdenes, precios y dinero de la orden, da
// descuentos y cancela con motivo; no ve los KPI ni la configuración (Equipo, Mi taller, lista de precios).
func TestMechanic_SeesOwnOrdersWithoutMoneyOrSettings(t *testing.T) {
	e := newEnv(t)
	owner := e.register("3105550170", "Felipe Duque")
	r := owner.must(http.StatusCreated, owner.do("POST", "/api/v1/users", map[string]any{"celular": "3005550171", "nombre": "Andrés Mecánico", "rol": "mecanico"}))
	mecID := r.str("id")
	owner.must(http.StatusCreated, owner.do("POST", "/api/v1/parts", map[string]any{
		"referencia": "PF-01", "nombre": "Pastillas de freno", "tipoVehiculo": "carro", "costo": 40000, "precioVenta": 65000, "existencia": 3, "minimo": 1,
	}))
	_, _, asignada := seedOrder(t, owner, "3005550172", "QAZ111")
	_, _, otra := seedOrder(t, owner, "3005550173", "QAZ222")
	owner.must(http.StatusOK, owner.do("PATCH", "/api/v1/orders/"+asignada+"/assignment", map[string]any{"mecanicoId": mecID}))

	mec := e.login("3005550171")
	me := mec.must(http.StatusOK, mec.do("GET", "/api/v1/me", nil))
	perms, _ := me.body["permisos"].([]any)
	if containsAny(perms, "user:manage", "kpi:read", "workshop:manage", "service:manage", "data_request:manage", "customer:merge", "import:run", "audit:read") {
		t.Fatalf("el mecánico tiene permisos de más: %v", perms)
	}
	for _, need := range []string{"money:read", "quote:edit", "quote:discount", "order:cancel", "customer:read_all", "order:create"} {
		if !containsAny(perms, need) {
			t.Fatalf("al mecánico le falta %s: %v", need, perms)
		}
	}

	// HU-23 criterio 1: con órdenes asignadas ve solo las suyas.
	r = mec.must(http.StatusOK, mec.do("GET", "/api/v1/orders/mine", nil))
	ordenes, _ := r.body["ordenes"].([]any)
	if len(ordenes) != 1 || ordenes[0].(map[string]any)["id"] != asignada {
		t.Fatalf("mis órdenes: %v", r.body)
	}
	// HU-23 criterio 2: el dueño no tiene asignadas → ve todas las abiertas.
	r = owner.must(http.StatusOK, owner.do("GET", "/api/v1/orders/mine", nil))
	if l, _ := r.body["ordenes"].([]any); len(l) != 2 {
		t.Fatalf("todas las abiertas (sin asignar %s): %v", otra, r.body)
	}

	// Precio de venta sí, costo de compra no (part:cost sigue siendo del dueño).
	r = mec.must(http.StatusOK, mec.do("GET", "/api/v1/parts", nil))
	p := r.body["repuestos"].([]any)[0].(map[string]any)
	if _, ok := p["costo"]; ok || p["precioVenta"] != float64(65000) {
		t.Fatalf("repuesto visto por el mecánico: %v", p)
	}

	// La configuración y los números responden "sin permiso" (403 en el servidor).
	svc := owner.must(http.StatusCreated, owner.do("POST", "/api/v1/services", map[string]any{"nombre": "Alineación", "tipoVehiculo": "carro"})).str("id")
	for _, c := range []struct{ method, path string }{
		{"GET", "/api/v1/users"},
		{"PUT", "/api/v1/workshop/settings"},
		{"POST", "/api/v1/workshop/onboarding"},
		{"PUT", "/api/v1/workshop/schedule"},
		{"POST", "/api/v1/services"},
		{"PATCH", "/api/v1/services/" + svc},
		{"POST", "/api/v1/parts"},
		{"GET", "/api/v1/dashboard"},
	} {
		if r := mec.do(c.method, c.path, map[string]any{}); r.status != http.StatusForbidden {
			t.Errorf("%s %s: esperaba 403, obtuve %d", c.method, c.path, r.status)
		}
	}
	// Cancelar sí, con motivo obligatorio (V-05).
	if r := mec.do("POST", "/api/v1/orders/"+asignada+"/cancel", map[string]any{"motivo": ""}); r.status < 400 || r.status == http.StatusForbidden {
		t.Fatalf("cancelar sin motivo: %d %v", r.status, r.body)
	}
	mec.must(http.StatusOK, mec.do("POST", "/api/v1/orders/"+asignada+"/cancel", map[string]any{"motivo": "El cliente no volvió", "version": r2v(mec, asignada)}))
}

// V-05: un solo panel. El mecánico recibe el mismo "Hoy tienes pendiente" que el dueño (la cotización que espera al
// cliente incluida) y ninguna entrada trae montos: el servidor no los manda a nadie en esta lista.
func TestMechanic_PendingTodayWithoutAmounts(t *testing.T) {
	e := newEnv(t)
	owner := e.register("3105550175", "Felipe Duque")
	owner.must(http.StatusCreated, owner.do("POST", "/api/v1/users", map[string]any{"celular": "3005550176", "nombre": "Andrés Mecánico", "rol": "mecanico"}))
	svc := owner.must(http.StatusCreated, owner.do("POST", "/api/v1/services", map[string]any{"nombre": "Revisión", "tipoVehiculo": "carro", "precio": 85000})).str("id")
	r := owner.must(http.StatusCreated, owner.do("POST", "/api/v1/customers", map[string]any{"autorizacion": acepta, "celular": "3005550177", "nombre": "Ana Ríos"}))
	owner.must(http.StatusCreated, owner.do("POST", "/api/v1/vehicles", map[string]any{"placa": "PEN177", "clienteId": nested(r.body, "cliente", "id"),
		"cilindraje": "1400", "turbo": false, "energia": "gasolina", "transmision": "manual"}))
	id := owner.must(http.StatusCreated, owner.do("POST", "/api/v1/orders", map[string]any{"placa": "PEN177", "sintoma": "Frena mal", "estadoInicial": "recibido"})).str("id")
	owner.must(http.StatusOK, owner.do("POST", "/api/v1/orders/"+id+"/transition", map[string]any{"estado": "en_diagnostico", "version": r2v(owner, id)}))
	q := "/api/v1/orders/" + id + "/quote"
	owner.must(http.StatusOK, owner.do("POST", q+"/lines", map[string]any{"tipo": "servicio", "refId": svc, "cantidad": 1}))
	owner.must(http.StatusOK, owner.do("POST", q+"/send", nil))

	mec := e.login("3005550176")
	list := func(c *client) string {
		var b strings.Builder
		for _, p := range c.must(http.StatusOK, c.do("GET", "/api/v1/today", nil)).body["pendientes"].([]any) {
			m := p.(map[string]any)
			b.WriteString(m["tipo"].(string) + "|" + m["titulo"].(string) + "|" + m["detalle"].(string) + "\n")
		}
		return b.String()
	}
	own, mine := list(owner), list(mec)
	if !strings.Contains(mine, "aprobacion|PEN177 no ha aprobado") || own != mine {
		t.Fatalf("el mecánico debía ver lo mismo que el dueño:\n%s\n---\n%s", own, mine)
	}
	if strings.Contains(mine, "$") || strings.Contains(mine, "85.000") || strings.Contains(mine, "85000") {
		t.Fatalf("la lista no lleva montos: %s", mine)
	}
}

func containsAny(list []any, wanted ...string) bool {
	for _, x := range list {
		for _, w := range wanted {
			if x == w {
				return true
			}
		}
	}
	return false
}
