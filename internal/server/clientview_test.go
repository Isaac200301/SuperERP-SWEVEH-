package server_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"sweveh/internal/shared"
)

// R7 (V-10, HU-109, HU-29, HU-86, ADR-0043): la cotización en pantalla y en PDF. Datos Ficticio.

// rawGet trae una ruta sin interpretar el cuerpo: sirve para el PDF y para buscar texto en el JSON tal cual sale.
func (c *client) rawGet(path string) (int, http.Header, []byte) {
	c.e.t.Helper()
	req, _ := http.NewRequest("GET", c.e.srv.URL+path, nil)
	req.Header.Set("X-Requested-With", "sweveh")
	res, err := c.http.Do(req)
	if err != nil {
		c.e.t.Fatalf("GET %s: %v", path, err)
	}
	defer func() { _ = res.Body.Close() }()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, res.Header, b
}

// Cosas internas que jamás deben llegar al cliente, ni en pantalla ni en el PDF.
const (
	secretDetailRef = "7771777"              // valor de referencia del detalle interno
	secretExpense   = "GASTO-INTERNO-XYZ"    // descripción de un gasto
	secretNote      = "NOTA-INTERNA-XYZ"     // nota de una evidencia de la orden
	secretDetail    = "Pastillas delanteras" // detalle de la partida: solo con "Mostrar detalle al cliente"
)

// partidaOrder deja una orden en diagnóstico con una partida (480.000, con detalle interno) y un servicio de lista (60.000).
func partidaOrder(t *testing.T, c *client, phone, plate string) (orderPath string) {
	t.Helper()
	svc := c.must(http.StatusCreated, c.do("POST", "/api/v1/services", map[string]any{"nombre": "Alineación " + plate, "tipoVehiculo": "carro", "precio": 60000})).str("id")
	q := quotedOrder(t, c, phone, plate)
	pid := lines(c.must(http.StatusOK, c.do("POST", q+"/lines", map[string]any{"tipo": "partida", "descripcion": "Frenos completos", "cantidad": 1, "precioUnitario": 480000})))[0]["id"].(string)
	c.must(http.StatusOK, c.do("POST", q+"/lines", map[string]any{"tipo": "servicio", "refId": svc, "cantidad": 1}))
	c.must(http.StatusOK, c.do("PUT", q+"/lines/"+pid+"/detail", map[string]any{"version": qv(c, q), "detalle": []map[string]any{
		{"descripcion": secretDetail, "cantidad": 2, "valorRef": 7771777}, {"descripcion": "Rectificar discos"}}}))
	return strings.TrimSuffix(q, "/quote")
}

func TestClientView_WhatTheCustomerSeesAndWhatNot(t *testing.T) {
	e := newPINEnv(t)
	c := e.registerPIN("3105551101", "Servitaller El Pistón")
	base := partidaOrder(t, c, "3005551102", "CLI102")
	q := base + "/quote"

	// Borrador, sin "Mostrar detalle": se puede mostrar y compartir, pero el detalle interno no sale.
	status, _, body := c.rawGet(q + "/client-view")
	if status != http.StatusOK {
		t.Fatalf("client-view: %d %s", status, body)
	}
	s := string(body)
	for _, no := range []string{secretDetail, "Rectificar", secretDetailRef, "valorRef"} {
		if strings.Contains(s, no) {
			t.Fatalf("el detalle interno llegó al cliente (%q): %s", no, s)
		}
	}
	v := c.must(http.StatusOK, c.do("GET", q+"/client-view", nil))
	doc := v.body["documento"].(map[string]any)
	cq := doc["cotizacion"].(map[string]any)
	if doc["taller"] != "Servitaller El Pistón" || !strings.HasPrefix(doc["orden"].(string), "OS-") || doc["placa"] != "CLI102" ||
		cq["estado"] != "borrador" || cq["total"] != float64(540000) || len(cq["lineas"].([]any)) != 2 {
		t.Fatalf("documento: %v", doc)
	}
	if l := cq["lineas"].([]any)[0].(map[string]any); l["tipo"] != "partida" || l["detalle"] != nil || l["precioUnitario"] != float64(480000) {
		t.Fatalf("partida: %v", l)
	}

	// "Mostrar detalle al cliente" encendido: el detalle sale, pero sin el valor de referencia interno.
	c.must(http.StatusOK, c.do("PATCH", q, map[string]any{"mostrarDetalle": true, "version": qv(c, q)}))
	_, _, body = c.rawGet(q + "/client-view")
	s = string(body)
	if !strings.Contains(s, secretDetail) || !strings.Contains(s, "Rectificar discos") {
		t.Fatalf("con detalle encendido debía salir: %s", s)
	}
	if strings.Contains(s, secretDetailRef) || strings.Contains(s, "valorRef") {
		t.Fatalf("el valor de referencia es interno: %s", s)
	}

	// Enviada y aprobada en el mostrador; luego se anotan un gasto y una nota interna que el cliente no debe ver.
	c.must(http.StatusOK, c.do("POST", q+"/send", nil))
	c.must(http.StatusOK, c.do("POST", q+"/response", map[string]any{"decision": "aprobar", "canal": "mostrador"}))
	c.must(http.StatusCreated, c.do("POST", base+"/expenses", map[string]any{"id": shared.NewID().String(), "descripcion": secretExpense, "valor": 99999}))
	c.must(http.StatusCreated, c.uploadPhotos(base+"/evidence", map[string]string{"nota": secretNote}, 1))

	status, _, body = c.rawGet(q + "/client-view")
	s = string(body)
	if status != http.StatusOK {
		t.Fatalf("client-view aprobada: %d", status)
	}
	for _, no := range []string{secretExpense, secretNote, "99999", "gast", "refId", "precioLibre", "sinStock", "horas", "version", "nota"} {
		if strings.Contains(s, no) {
			t.Fatalf("lo interno llegó al cliente (%q): %s", no, s)
		}
	}
	if !strings.Contains(s, `"estado":"aprobada"`) || !strings.Contains(s, `"vencida":false`) {
		t.Fatalf("estado: %s", s)
	}

	// El PDF: responde application/pdf, con nombre, sin caché, con el total y sin lo interno.
	status, h, pdf := c.rawGet(q + ".pdf")
	if status != http.StatusOK || h.Get("Content-Type") != "application/pdf" || h.Get("Cache-Control") != "no-store" {
		t.Fatalf("pdf: %d %v", status, h)
	}
	if cd := h.Get("Content-Disposition"); !strings.HasPrefix(cd, `attachment; filename="cotizacion-OS-`) || !strings.HasSuffix(cd, `.pdf"`) {
		t.Fatalf("nombre del archivo: %q", cd)
	}
	p := string(pdf)
	for _, want := range []string{"%PDF-1.4", "Servitaller El Pist\\363n", "Frenos completos", "$ 540.000", "Firma del cliente", "CLI102", secretDetail} {
		if !strings.Contains(p, want) {
			t.Errorf("el PDF no trae %q", want)
		}
	}
	for _, no := range []string{secretExpense, secretNote, secretDetailRef, "99999", "gast"} {
		if strings.Contains(p, no) {
			t.Errorf("lo interno llegó al PDF: %q", no)
		}
	}
}

// Sin "Mostrar detalle al cliente" el PDF tampoco trae el detalle, y un borrador lo dice.
func TestClientView_PDFHidesDetailAndMarksDraft(t *testing.T) {
	e := newPINEnv(t)
	c := e.registerPIN("3105551103", "Servitaller El Pistón")
	q := partidaOrder(t, c, "3005551104", "CLI104") + "/quote"
	status, _, pdf := c.rawGet(q + ".pdf")
	if status != http.StatusOK {
		t.Fatalf("pdf: %d", status)
	}
	if p := string(pdf); strings.Contains(p, secretDetail) || strings.Contains(p, "Rectificar") || !strings.Contains(p, "BORRADOR") {
		t.Fatal("el PDF de un borrador sin detalle encendido trae el detalle o no dice que es borrador")
	}
}

// HU-29 y V-10: "No aprobó" con motivo opcional; el motivo queda en la bitácora y no se registra dos veces.
func TestClientView_RejectWithReason(t *testing.T) {
	e := newPINEnv(t)
	c := e.registerPIN("3105551105", "Servitaller El Pistón")
	q := partidaOrder(t, c, "3005551106", "CLI106") + "/quote"
	c.must(http.StatusOK, c.do("POST", q+"/send", nil))

	is4xx(t, "motivo muy largo", c.do("POST", q+"/response", map[string]any{"decision": "rechazar", "canal": "mostrador", "motivo": strings.Repeat("x", 201)}))
	r := cot(c.must(http.StatusOK, c.do("POST", q+"/response", map[string]any{"decision": "rechazar", "canal": "mostrador", "motivo": "  Está muy   caro  "})))
	if r.body["estado"] != "rechazada" {
		t.Fatalf("estado: %v", r.body)
	}
	var n int
	if err := e.asTaller(tallerOf(t, c), `SELECT count(*) FROM audit.evento WHERE accion = 'cotizacion.respuesta' AND despues->>'motivo' = 'Está muy caro'`, &n); err != nil || n != 1 {
		t.Fatalf("bitácora con el motivo: %d %v", n, err)
	}
	// La pantalla del cliente ya no se puede responder otra vez.
	is4xx(t, "responder dos veces", c.do("POST", q+"/response", map[string]any{"decision": "aprobar", "canal": "mostrador"}))
	if got := c.must(http.StatusOK, c.do("GET", q+"/client-view", nil)); nested(got.body, "documento", "cotizacion", "estado") != "rechazada" {
		t.Fatalf("client-view tras rechazar: %v", got.body)
	}
}

// HU-86: la pantalla dice si el cliente ya autorizó sus datos y con qué texto; al autorizar en el mostrador cambia.
func TestClientView_CounterConsent(t *testing.T) {
	e := newPINEnv(t)
	c := e.registerPIN("3105551107", "Servitaller El Pistón")
	q := partidaOrder(t, c, "3005551108", "CLI108") + "/quote"
	c.must(http.StatusOK, c.do("POST", q+"/send", nil))

	a := c.must(http.StatusOK, c.do("GET", q+"/client-view", nil)).body["autorizacion"].(map[string]any)
	if a["estado"] != "pendiente" || !strings.HasPrefix(a["texto"].(string), "Autorizo al taller a tratar mis datos") {
		t.Fatalf("autorización antes: %v", a)
	}
	c.must(http.StatusOK, c.do("POST", "/api/v1/customers/"+a["clienteId"].(string)+"/consent", map[string]any{"acepta": true}))
	if a = c.must(http.StatusOK, c.do("GET", q+"/client-view", nil)).body["autorizacion"].(map[string]any); a["estado"] != "autorizada" {
		t.Fatalf("autorización después: %v", a)
	}
}

// Sin cotización no hay qué mostrar; otro taller no ve la de este (CA-10); sin sesión, 401; el mecánico sí la ve (V-05).
func TestClientView_NotFoundIsolationAndRoles(t *testing.T) {
	e := newPINEnv(t)
	c := e.registerPIN("3105551109", "Servitaller El Pistón")
	empty := strings.TrimSuffix(quotedOrder(t, c, "3005551110", "CLI110"), "/quote") + "/quote"
	if r := c.do("GET", empty+"/client-view", nil); r.status != http.StatusNotFound || r.str("code") != "cotizacion_no_encontrada" {
		t.Fatalf("sin cotización: %d %v", r.status, r.body)
	}
	if status, _, _ := c.rawGet(empty + ".pdf"); status != http.StatusNotFound {
		t.Fatalf("pdf sin cotización: %d", status)
	}

	q := partidaOrder(t, c, "3005551111", "CLI111") + "/quote"
	other := e.registerPIN("3105551112", "Otro Taller")
	if r := other.do("GET", q+"/client-view", nil); r.status != http.StatusNotFound {
		t.Fatalf("otro taller ve la vista del cliente: %d", r.status)
	}
	if status, _, _ := other.rawGet(q + ".pdf"); status != http.StatusNotFound {
		t.Fatalf("otro taller baja el PDF: %d", status)
	}
	if status, _, _ := e.client().rawGet(q + ".pdf"); status != http.StatusUnauthorized {
		t.Fatalf("sin sesión: %d", status)
	}
	if r := e.client().do("GET", q+"/client-view", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("sin sesión: %d", r.status)
	}

	c.must(http.StatusCreated, c.do("POST", "/api/v1/users", map[string]any{"celular": "3005551113", "nombre": "Jhon Mecánico", "rol": "mecanico", "pin": "7391"}))
	mec, r := e.loginPIN("3005551113", "7391")
	if r.status != http.StatusOK {
		t.Fatalf("login del mecánico: %d %v", r.status, r.body)
	}
	mec.must(http.StatusOK, mec.do("GET", q+"/client-view", nil))
	if status, _, _ := mec.rawGet(q + ".pdf"); status != http.StatusOK {
		t.Fatalf("el mecánico baja el PDF: %d", status)
	}
}
