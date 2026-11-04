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

// Estados opcionales de punta a punta: apagados no existen para el taller; activados, la orden pasa por ellos.
func TestSettings_OptionalStatesInTheFlow(t *testing.T) {
	e := newEnv(t)
	c := e.register("3105550350", "Pablo Rodríguez")
	id, _ := orderInDiagnosis(t, c, "EST351", 2)
	tr := func(to string, extra map[string]any) resp {
		body := map[string]any{"estado": to, "version": r2v(c, id)}
		for k, v := range extra {
			body[k] = v
		}
		return c.do("POST", "/api/v1/orders/"+id+"/transition", body)
	}
	q := "/api/v1/orders/" + id + "/quote"
	c.must(http.StatusOK, c.do("POST", q+"/lines", map[string]any{"tipo": "servicio", "descripcion": "Mano de obra", "cantidad": 1, "precioUnitario": 50000}))
	c.must(http.StatusOK, c.do("POST", q+"/send", nil))
	c.must(http.StatusOK, c.do("POST", q+"/response", map[string]any{"decision": "aprobar", "canal": "mostrador"}))

	if r := tr("esperando_repuesto", map[string]any{"motivo": "Kit"}); r.str("code") != "estado_no_activo" {
		t.Fatalf("apagado: %d %v", r.status, r.body)
	}
	s := c.must(http.StatusOK, c.do("GET", "/api/v1/workshop/settings", nil)).body
	s["usarEsperandoRepuesto"], s["usarControlCalidad"] = true, true
	c.must(http.StatusOK, c.do("PUT", "/api/v1/workshop/settings", s))

	is4xx(t, "sin motivo", tr("esperando_repuesto", nil))
	c.must(http.StatusOK, tr("esperando_repuesto", map[string]any{"motivo": "Llega el kit de embrague el jueves"}))
	c.must(http.StatusOK, tr("en_reparacion", nil))
	if r := tr("listo", nil); r.str("code") != "salto_de_estado" {
		t.Fatalf("con control de calidad no se salta a Listo: %d %v", r.status, r.body)
	}
	c.must(http.StatusOK, tr("control_calidad", nil))
	if r := c.must(http.StatusOK, tr("listo", nil)); r.str("estado") != "listo" {
		t.Fatalf("listo: %v", r.body)
	}
	hist := c.must(http.StatusOK, c.do("GET", "/api/v1/orders/"+id, nil)).body["historial"].([]any)
	var pasos []string
	for _, h := range hist {
		pasos = append(pasos, h.(map[string]any)["hacia"].(string))
	}
	want := "esperando_repuesto en_reparacion control_calidad listo"
	if got := pasos[len(pasos)-4:]; got[0]+" "+got[1]+" "+got[2]+" "+got[3] != want {
		t.Fatalf("historial: %v", pasos)
	}
}

// Cotización real con los parámetros: IVA aparte, mano de obra por horas, descuento con tope y permiso, vigencia.
func TestSettings_RealQuoteWithVATHoursDiscountValidity(t *testing.T) {
	e := newEnv(t)
	c := e.register("3105550360", "Pablo Rodríguez")
	s := c.must(http.StatusOK, c.do("GET", "/api/v1/workshop/settings", nil)).body
	s["preciosConIva"], s["tarifaHora"], s["descuentoMaxPct"], s["vigenciaDias"] = false, 80000, 10, 15
	c.must(http.StatusOK, c.do("PUT", "/api/v1/workshop/settings", s))
	id, pid := orderInDiagnosis(t, c, "IVA361", 5)
	q := "/api/v1/orders/" + id + "/quote"

	is4xx(t, "horas fuera de cuartos", c.do("POST", q+"/lines", map[string]any{"tipo": "servicio", "descripcion": "Mano de obra", "horas": 1.3}))
	ls := lines(c.must(http.StatusOK, c.do("POST", q+"/lines", map[string]any{"tipo": "servicio", "descripcion": "Mano de obra frenos", "horas": 1.5})))
	if ls[0]["precioUnitario"] != float64(120000) || ls[0]["horas"] != 1.5 {
		t.Fatalf("1,5 h × 80.000: %v", ls[0])
	}
	if r := c.do("PATCH", q+"/lines/"+ls[0]["id"].(string), map[string]any{"precioUnitario": 1}); r.status < 400 {
		t.Fatalf("la mano de obra por horas no se edita a mano: %d", r.status)
	}
	labor := ls[0]["id"].(string)
	is4xx(t, "cambiar a horas fuera de cuartos", c.do("PATCH", q+"/lines/"+labor, map[string]any{"horas": 1.3}))
	ls = lines(c.must(http.StatusOK, c.do("PATCH", q+"/lines/"+labor, map[string]any{"horas": 2})))
	if ls[0]["precioUnitario"] != float64(160000) || ls[0]["horas"] != float64(2) {
		t.Fatalf("2 h × 80.000: %v", ls[0])
	}
	c.must(http.StatusOK, c.do("PATCH", q+"/lines/"+labor, map[string]any{"horas": 1.5}))
	ls = lines(c.must(http.StatusOK, c.do("POST", q+"/lines", map[string]any{"tipo": "repuesto", "refId": pid, "cantidad": 2})))
	part := ls[1]["id"].(string)
	is4xx(t, "horas en un repuesto", c.do("PATCH", q+"/lines/"+part, map[string]any{"horas": 1}))
	is4xx(t, "descuento sobre el máximo", c.do("PATCH", q+"/lines/"+part, map[string]any{"descuentoPct": 11}))
	r := cot(c.must(http.StatusOK, c.do("PATCH", q+"/lines/"+part, map[string]any{"descuentoPct": 10})))
	// 120.000 + 130.000 − 13.000 = 237.000; IVA 19 % = 45.030; total 282.030.
	if r.body["subtotal"] != float64(250000) || r.body["descuento"] != float64(13000) || r.body["iva"] != float64(45030) || r.body["total"] != float64(282030) {
		t.Fatalf("desglose: %v", r.body)
	}

	// Recepción no da descuentos salvo que el taller lo permita; el mecánico nunca.
	c.must(http.StatusCreated, c.do("POST", "/api/v1/users", map[string]any{"celular": "3005550362", "nombre": "Recepción", "rol": "recepcion"}))
	rec := e.login("3005550362")
	if r := rec.do("PATCH", q+"/lines/"+part, map[string]any{"descuentoPct": 5}); r.status != http.StatusForbidden {
		t.Fatalf("recepción sin permiso del taller: %d %v", r.status, r.body)
	}
	s = c.must(http.StatusOK, c.do("GET", "/api/v1/workshop/settings", nil)).body
	s["descuentoRecepcion"] = true
	c.must(http.StatusOK, c.do("PUT", "/api/v1/workshop/settings", s))
	rec.must(http.StatusOK, rec.do("PATCH", q+"/lines/"+part, map[string]any{"descuentoPct": 5}))

	// Enviar fija la vigencia; renovar la vuelve a contar. El IVA quedó copiado aunque el taller lo cambie.
	r = cot(c.must(http.StatusOK, c.do("POST", q+"/send", nil)))
	if r.body["venceEn"] == nil || r.body["vencida"] != false || r.body["ivaPct"] != float64(19) {
		t.Fatalf("enviada: %v", r.body)
	}
	s = c.must(http.StatusOK, c.do("GET", "/api/v1/workshop/settings", nil)).body
	s["ivaPct"] = 0
	c.must(http.StatusOK, c.do("PUT", "/api/v1/workshop/settings", s))
	if r := cot(c.must(http.StatusOK, c.do("POST", q+"/renew", nil))); r.body["ivaPct"] != float64(19) || r.body["venceEn"] == nil {
		t.Fatalf("renovada conserva su IVA: %v", r.body)
	}
}

// Sin cotización no se envía, no se renueva, no se responde ni se cambia un ítem.
func TestSettings_QuoteActionsWithoutQuote(t *testing.T) {
	e := newEnv(t)
	c := e.register("3105550370", "Pablo Rodríguez")
	id, _ := orderInDiagnosis(t, c, "SIN370", 1)
	q := "/api/v1/orders/" + id + "/quote"
	is4xx(t, "enviar sin cotización", c.do("POST", q+"/send", nil))
	is4xx(t, "renovar sin cotización", c.do("POST", q+"/renew", nil))
	is4xx(t, "responder sin cotización", c.do("POST", q+"/response", map[string]any{"decision": "aprobada", "canal": "telefono"}))
	is4xx(t, "cambiar un ítem sin cotización", c.do("PATCH", q+"/lines/00000000-0000-0000-0000-000000000000", map[string]any{"cantidad": 2}))
	is4xx(t, "quitar un ítem sin cotización", c.do("DELETE", q+"/lines/00000000-0000-0000-0000-000000000000", nil))
}
