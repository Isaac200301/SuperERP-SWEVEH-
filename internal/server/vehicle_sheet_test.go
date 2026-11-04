package server_test

// Ficha del vehículo (decisión del 3-nov-2026) de punta a punta: registro, compuerta de Recibido y
// completar la ficha con If-Match. Datos Ficticio.

import (
	"net/http"
	"testing"
)

func TestVehicleSheet_RegisterGateAndComplete(t *testing.T) {
	e := newEnv(t)
	c := e.register("3105550150", "Diana Mejía")
	r := c.must(http.StatusCreated, c.do("POST", "/api/v1/customers", map[string]any{"celular": "3005550150", "nombre": "Jorge Pérez"}))
	cliente := nested(r.body, "cliente", "id").(string)

	// Carro diésel turbo automático 4x4: ficha completa, se puede recibir de una vez.
	r = c.must(http.StatusCreated, c.do("POST", "/api/v1/vehicles", map[string]any{
		"placa": "FTR123", "clienteId": cliente, "cilindraje": "2.8 L", "energia": "diesel", "aspiracion": "turbo",
		"transmision": "automatica", "traccion": "4x4", "carroceria": "pickup", "color": "Blanco", "llanta": "265/65 r17",
	}))
	v := r.body["vehiculo"].(map[string]any)
	if v["motorPendiente"] != false || v["turbo"] != true || v["cilindrajeCc"] != float64(2800) || v["llanta"] != "265/65 R17" {
		t.Fatalf("carro diésel: %v", v)
	}
	c.must(http.StatusCreated, c.do("POST", "/api/v1/orders", map[string]any{"placa": "FTR123", "sintoma": "Revisión", "estadoInicial": "recibido"}))

	// Moto con lo que no le aplica → 400; moto 2T por aire sin caja → pendiente y no se recibe.
	r = c.do("POST", "/api/v1/vehicles", map[string]any{"placa": "XVN43F", "clienteId": cliente, "traccion": "4x4"})
	if r.status != http.StatusBadRequest || r.str("code") != "traccion_invalida" {
		t.Fatalf("moto con tracción: %d %v", r.status, r.body)
	}
	r = c.must(http.StatusCreated, c.do("POST", "/api/v1/vehicles", map[string]any{
		"placa": "XVN43F", "clienteId": cliente, "cilindraje": "125", "energia": "gasolina", "ciclo": "2t", "refrigeracion": "aire",
	}))
	moto := r.body["vehiculo"].(map[string]any)
	if moto["motorPendiente"] != true || moto["aspiracion"] != "atmosferico" {
		t.Fatalf("moto sin caja: %v", moto)
	}
	r = c.do("POST", "/api/v1/orders", map[string]any{"placa": "XVN43F", "sintoma": "No prende", "estadoInicial": "recibido"})
	if r.status != http.StatusUnprocessableEntity || r.str("code") != "motor_pendiente" {
		t.Fatalf("moto pendiente no debía recibirse: %d %v", r.status, r.body)
	}

	// Completar la ficha con If-Match deja la moto lista.
	g := c.must(http.StatusOK, c.do("GET", "/api/v1/vehicles/"+moto["id"].(string), nil))
	r = c.must(http.StatusOK, c.do("PATCH", "/api/v1/vehicles/"+moto["id"].(string)+"/engine",
		map[string]any{"transmision": "manual", "color": "Negro"}, "If-Match", g.header.Get("ETag")))
	if r.body["motorPendiente"] != false || r.body["ciclo"] != "2t" || r.body["color"] != "Negro" {
		t.Fatalf("moto completa: %v", r.body)
	}

	// Eléctrico: sin cilindraje ni aspiración y aun así completo.
	r = c.must(http.StatusCreated, c.do("POST", "/api/v1/vehicles", map[string]any{
		"placa": "EVA321", "clienteId": cliente, "energia": "electrico", "transmision": "automatica", "cilindraje": "1600",
	}))
	ev := r.body["vehiculo"].(map[string]any)
	if ev["motorPendiente"] != false || ev["cilindrajeCc"] != nil {
		t.Fatalf("eléctrico: %v", ev)
	}
}
