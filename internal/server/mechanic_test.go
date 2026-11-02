package server_test

// HU-23 y HU-81: el mecánico entra con su celular, ve sus órdenes y no ve dinero ni configuración (RF-21).
// Datos Ficticio.

import (
	"net/http"
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
	if perms, _ := me.body["permisos"].([]any); containsAny(perms, "money:read", "user:manage", "kpi:read") {
		t.Fatalf("el mecánico tiene permisos de más: %v", perms)
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

	// HU-81 criterio 1: precio de venta sí, costo no.
	r = mec.must(http.StatusOK, mec.do("GET", "/api/v1/parts", nil))
	p := r.body["repuestos"].([]any)[0].(map[string]any)
	if _, ok := p["costo"]; ok || p["precioVenta"] != float64(65000) {
		t.Fatalf("repuesto visto por el mecánico: %v", p)
	}

	// HU-81 criterio 2: la configuración responde "sin permiso".
	for _, c := range []struct{ method, path string }{
		{"GET", "/api/v1/users"},
		{"POST", "/api/v1/workshop/onboarding"},
		{"PUT", "/api/v1/workshop/schedule"},
		{"POST", "/api/v1/services"},
		{"POST", "/api/v1/parts"},
		{"POST", "/api/v1/orders/" + asignada + "/cancel"},
	} {
		if r := mec.do(c.method, c.path, map[string]any{}); r.status != http.StatusForbidden {
			t.Errorf("%s %s: esperaba 403, obtuve %d", c.method, c.path, r.status)
		}
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
