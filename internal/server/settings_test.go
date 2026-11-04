package server_test

// Parámetros del taller de punta a punta: por defecto sin configurar, guardar con versión, validación,
// permisos (solo el dueño cambia; todos leen) y aislamiento entre talleres. Datos Ficticio.

import (
	"net/http"
	"testing"
)

func TestSettings_DefaultsSaveVersionPermissionsIsolation(t *testing.T) {
	e := newEnv(t)
	c := e.register("3105550330", "Pablo Rodríguez")

	r := c.must(http.StatusOK, c.do("GET", "/api/v1/workshop/settings", nil))
	if r.body["ivaPct"] != float64(19) || r.body["version"] != float64(0) || r.body["fotosMinimas"] != float64(4) || len(r.body["itemsCarro"].([]any)) == 0 {
		t.Fatalf("por defecto: %v", r.body)
	}
	in := r.body
	in["usarEsperandoRepuesto"], in["ivaPct"], in["tarifaHora"], in["vigenciaDias"] = true, 0, 80000, 30
	in["itemsMoto"] = []map[string]any{{"clave": "", "nombre": "Casco"}, {"clave": "", "nombre": "Kit de herramienta"}}
	r = c.must(http.StatusOK, c.do("PUT", "/api/v1/workshop/settings", in))
	if r.body["version"] != float64(1) || r.body["usarEsperandoRepuesto"] != true || r.body["ivaPct"] != float64(0) || r.body["tarifaHora"] != float64(80000) {
		t.Fatalf("guardado: %v", r.body)
	}
	if k := r.body["itemsMoto"].([]any)[1].(map[string]any)["clave"]; k != "kit_de_herramienta" {
		t.Fatalf("clave del ítem: %v", k)
	}
	// Versión vieja → 409; valores fuera de rango → 400.
	if r := c.do("PUT", "/api/v1/workshop/settings", in); r.status != http.StatusConflict {
		t.Fatalf("versión vieja: %d %v", r.status, r.body)
	}
	in["version"], in["vigenciaDias"] = 1, 0
	is4xx(t, "vigencia 0", c.do("PUT", "/api/v1/workshop/settings", in))

	// El mecánico lee pero no cambia.
	c.must(http.StatusCreated, c.do("POST", "/api/v1/users", map[string]any{"celular": "3005550331", "nombre": "Mecánico", "rol": "mecanico"}))
	mec := e.login("3005550331")
	if r := mec.must(http.StatusOK, mec.do("GET", "/api/v1/workshop/settings", nil)); r.body["tarifaHora"] != float64(80000) {
		t.Fatalf("el mecánico debía leer los parámetros: %v", r.body)
	}
	if r := mec.do("PUT", "/api/v1/workshop/settings", in); r.status != http.StatusForbidden {
		t.Fatalf("el mecánico cambia parámetros: %d", r.status)
	}
	// Otro taller sigue con sus valores por defecto.
	other := e.register("3105550339", "Otro Taller")
	if r := other.must(http.StatusOK, other.do("GET", "/api/v1/workshop/settings", nil)); r.body["ivaPct"] != float64(19) || r.body["version"] != float64(0) {
		t.Fatalf("aislamiento: %v", r.body)
	}
}

// La recepción usa los parámetros: ítems del taller con su estado, mínimo de fotos y kilometraje opcional.
func TestSettings_ReceptionUsesWorkshopRules(t *testing.T) {
	e := newEnv(t)
	c := e.register("3105550340", "Pablo Rodríguez")
	s := c.must(http.StatusOK, c.do("GET", "/api/v1/workshop/settings", nil)).body
	s["fotosMinimas"], s["kmObligatorio"] = 1, false
	s["itemsCarro"] = []map[string]any{{"clave": "", "nombre": "Luces"}, {"clave": "", "nombre": "Extintor"}}
	c.must(http.StatusOK, c.do("PUT", "/api/v1/workshop/settings", s))

	_, _, id := seedOrder(t, c, "3005550341", "CHK341")
	base := "/api/v1/orders/" + id
	r := c.must(http.StatusOK, c.do("GET", base+"/checklist", nil))
	its := r.body["items"].([]any)
	if len(its) != 2 || its[1].(map[string]any)["clave"] != "extintor" || r.body["fotosMinimas"] != float64(1) {
		t.Fatalf("checklist con los ítems del taller: %v", r.body)
	}
	c.must(http.StatusOK, c.do("PUT", base+"/checklist", map[string]any{"combustible": "1/2",
		"items": []map[string]any{{"clave": "luces", "estado": "mal", "nota": "stop fundido"}, {"clave": "inventado", "estado": "bien"}}}))
	c.must(http.StatusCreated, c.upload(base+"/evidence", map[string]string{"lado": "frente"}, jpg))
	// Una foto y sin kilometraje alcanza para recibir en este taller.
	c.must(http.StatusOK, c.do("POST", base+"/transition", map[string]any{"estado": "recibido", "version": r2v(c, id)}))

	// Cambiar la lista del taller después no altera la recepción hecha.
	s = c.must(http.StatusOK, c.do("GET", "/api/v1/workshop/settings", nil)).body
	s["itemsCarro"] = []map[string]any{{"clave": "", "nombre": "Solo radio"}}
	c.must(http.StatusOK, c.do("PUT", "/api/v1/workshop/settings", s))
	its = c.must(http.StatusOK, c.do("GET", base+"/checklist", nil)).body["items"].([]any)
	luces := its[0].(map[string]any)
	if len(its) != 2 || luces["estado"] != "mal" || luces["nota"] != "stop fundido" {
		t.Fatalf("la recepción conserva su copia de ítems: %v", its)
	}
}
