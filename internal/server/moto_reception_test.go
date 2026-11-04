package server_test

import (
	"net/http"
	"slices"
	"testing"
)

// HU-18: la moto pide sus 5 lados (con tablero, mínimo 4 fotos) y acepta "no aplica" en combustible.
// Lo que deja el cliente (casco, documentos) va como nota con foto (decisión de Pablo, 3-nov).
func TestMotoReception_SidesAndFuelNotApplicable(t *testing.T) {
	e := newEnv(t)
	c := e.register("3105550390", "Pablo Rodríguez")
	r := c.must(http.StatusCreated, c.do("POST", "/api/v1/customers", map[string]any{"celular": "3005550390", "nombre": "Andrés Moto"}))
	c.must(http.StatusCreated, c.do("POST", "/api/v1/vehicles", map[string]any{"placa": "MTO39F", "clienteId": nested(r.body, "cliente", "id"),
		"cilindraje": "150", "energia": "gasolina", "ciclo": "4t", "refrigeracion": "aire", "transmision": "manual"}))
	id := c.must(http.StatusCreated, c.do("POST", "/api/v1/orders", map[string]any{"placa": "MTO39F", "sintoma": "Frena mal"})).str("id")
	base := "/api/v1/orders/" + id

	cl := c.must(http.StatusOK, c.do("GET", base+"/checklist", nil)).body
	pedidos, _ := cl["ladosPedidos"].([]any)
	if cl["tipo"] != "moto" || len(pedidos) != 5 || !slices.Contains(pedidos, any("tablero")) {
		t.Fatalf("lados de moto: %v", cl)
	}

	// Mínimo 4 de los 5 lados: con 3 fotos no se recibe; con 4 (sin tablero) sí.
	for _, lado := range []string{"izquierdo", "derecho", "frente"} {
		c.must(http.StatusCreated, c.upload(base+"/evidence", map[string]string{"lado": lado}, jpg))
	}
	c.must(http.StatusOK, c.do("PUT", base+"/checklist", map[string]any{
		"kilometraje": 18000, "combustible": "no_aplica",
	}))
	is4xx(t, "moto con 3 fotos", c.do("POST", base+"/transition", map[string]any{"estado": "recibido", "version": r2v(c, id)}))
	c.must(http.StatusCreated, c.upload(base+"/evidence", map[string]string{"lado": "trasera"}, jpg))
	c.must(http.StatusOK, c.do("POST", base+"/transition", map[string]any{"estado": "recibido", "version": r2v(c, id)}))

	cl = c.must(http.StatusOK, c.do("GET", base+"/checklist", nil)).body
	if cl["combustible"] != "no_aplica" || cl["completadoEn"] == nil {
		t.Fatalf("recepción de la moto: %v", cl)
	}
}
