package server_test

import (
	"net/http"
	"testing"
)

// HU-20: pasos de uno en uno, puertas con código estable, versión optimista e historial con usuario y hora.
func TestFlow_TransitionsHistoryAndVersion(t *testing.T) {
	e := newEnv(t)
	c := e.register("3105550120", "Marta Ríos")

	r := c.must(http.StatusCreated, c.do("POST", "/api/v1/customers", map[string]any{"celular": "3005550120", "nombre": "Jorge Peña"}))
	clienteID := nested(r.body, "cliente", "id").(string)
	c.must(http.StatusCreated, c.do("POST", "/api/v1/vehicles", map[string]any{
		"placa": "QWE321", "clienteId": clienteID, "cilindraje": "1600", "turbo": false,
	}))
	r = c.must(http.StatusCreated, c.do("POST", "/api/v1/orders", map[string]any{"placa": "QWE321", "sintoma": "Vibra", "estadoInicial": "recibido"}))
	id, version := r.str("id"), r.body["version"]
	path := "/api/v1/orders/" + id + "/transition"

	// Criterio 1: no se salta estados.
	r = c.do("POST", path, map[string]any{"estado": "listo", "version": version})
	if r.status != http.StatusUnprocessableEntity || r.str("code") != "salto_de_estado" {
		t.Fatalf("salto: %d %v", r.status, r.body)
	}
	r = c.must(http.StatusOK, c.do("POST", path, map[string]any{"estado": "en_diagnostico", "version": version}))
	if r.str("estado") != "en_diagnostico" {
		t.Fatalf("avance: %v", r.body)
	}
	// Versión vieja → 409 (concurrencia optimista, R-07).
	r = c.do("POST", path, map[string]any{"estado": "recibido", "version": version})
	if r.status != http.StatusConflict || r.str("code") != "version_conflicto" {
		t.Fatalf("versión vieja: %d %v", r.status, r.body)
	}
	// Sin cotización enviada no pasa a Esperando aprobación.
	r = c.do("POST", path, map[string]any{"estado": "esperando_aprobacion", "version": r2v(c, id)})
	if r.status != http.StatusUnprocessableEntity || r.str("code") != "falta_cotizacion" {
		t.Fatalf("sin cotización: %d %v", r.status, r.body)
	}
	// Devolverse un paso sí se puede.
	c.must(http.StatusOK, c.do("POST", path, map[string]any{"estado": "recibido", "version": r2v(c, id)}))

	// Criterio 3: cada cambio queda con estado anterior, nuevo, usuario y hora.
	r = c.must(http.StatusOK, c.do("GET", "/api/v1/orders/"+id, nil))
	hist, _ := r.body["historial"].([]any)
	var pasos []string
	for _, h := range hist {
		m := h.(map[string]any)
		if m["usuarioId"] == nil || m["ocurridoEn"] == nil {
			t.Fatalf("historial sin usuario u hora: %v", m)
		}
		desde, _ := m["desde"].(string)
		pasos = append(pasos, desde+">"+m["hacia"].(string))
	}
	want := []string{">recibido", "recibido>en_diagnostico", "en_diagnostico>recibido"}
	if len(pasos) != len(want) {
		t.Fatalf("historial: %v", pasos)
	}
	for i := range want {
		if pasos[i] != want[i] {
			t.Fatalf("historial: %v, esperaba %v", pasos, want)
		}
	}
}

// r2v lee la versión vigente de la orden.
func r2v(c *client, id string) any {
	return c.must(http.StatusOK, c.do("GET", "/api/v1/orders/"+id, nil)).body["version"]
}
