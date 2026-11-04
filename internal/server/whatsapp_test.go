package server_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"sweveh/internal/messaging/adapters/out/whatsapp/meta"
	messagingapp "sweveh/internal/messaging/app"
	"sweveh/internal/messaging/domain"
	"sweveh/internal/platform/config"
	"sweveh/internal/platform/testdb"
	"sweveh/internal/wire"
)

// HU-85: un número nuevo recibe la autorización con el nombre del taller antes de cualquier proceso; "Acepto"
// guarda texto, versión, canal, id del mensaje y hora, y procesa el mensaje original; "No acepto" deja la
// conversación para una persona y nada pasa por el agente.
func TestWhatsApp_ConsentBeforeAnything(t *testing.T) {
	e := newEnv(t)
	owner := e.register("3105550400", "Pablo Rodríguez")
	w := e.whatsapp(t, owner, nil)

	w.write("3001234400", "Laura Gómez", "Hola, ¿a cómo el cambio de aceite?")
	first := w.last("3001234400")
	if !strings.Contains(first["cuerpo"].(string), "aceptas el tratamiento de tus datos") ||
		!strings.Contains(first["cuerpo"].(string), "Taller de Pablo") ||
		!hasPrefix(buttons(first), "consent:si") || !hasPrefix(buttons(first), "consent:no") {
		t.Fatalf("autorización: %v", first)
	}
	if len(w.drafts()) != 0 {
		t.Fatal("el mensaje pasó por el agente antes de la autorización")
	}
	// Mientras no responda, otro mensaje no vuelve a preguntar ni pasa por el agente.
	w.write("3001234400", "Laura Gómez", "¿Hola?")
	if n := len(w.sent("3001234400")); n != 1 || len(w.drafts()) != 0 {
		t.Fatalf("sin respuesta a la autorización: %d enviados, %d borradores", n, len(w.drafts()))
	}

	w.press("3001234400", "Laura Gómez", "consent:si", "Acepto")
	if !strings.Contains(w.last("3001234400")["cuerpo"].(string), "Ya quedó registrada tu autorización") {
		t.Fatalf("gracias: %v", w.last("3001234400"))
	}
	c := owner.must(http.StatusOK, owner.do("GET", "/api/v1/customers/by-phone/3001234400", nil)).body
	if c["autorizacionEstado"] != "autorizada" || c["nombre"] != "Laura Gómez" {
		t.Fatalf("cliente: %v", c)
	}
	conv, _ := w.conversation("3001234400")
	if conv["clienteId"] != c["id"] {
		t.Fatalf("la conversación no quedó ligada al cliente: %v", conv)
	}
	// Se procesó el mensaje original (pregunta de precio → borrador para que el dueño apruebe, RF-18).
	ds := w.drafts()
	if len(ds) != 1 || !strings.Contains(ds[0]["mensajeCliente"].(string), "cambio de aceite") {
		t.Fatalf("borradores tras aceptar: %v", ds)
	}
	var texto, version, canal, ref string
	err := e.db.Tx.Do(testdb.As(tallerOf(t, owner)), func(ctx context.Context) error {
		return queryRow(ctx, `SELECT texto, version_politica, canal, coalesce(ref, '') FROM customers.autorizacion
			WHERE estado = 'autorizada'`, &texto, &version, &canal, &ref)
	})
	if err != nil || texto == "" || version == "" || canal != "whatsapp" || ref == "" {
		t.Fatalf("registro de la autorización: %q %q %q %q %v", texto, version, canal, ref, err)
	}

	// "No acepto": modo humano, aviso y nada pasa por el agente después.
	w.write("3001234401", "Jorge Peña", "Buenas")
	w.press("3001234401", "Jorge Peña", "consent:no", "No acepto")
	conv, _ = w.conversation("3001234401")
	if conv["modo"] != "humano" || !strings.Contains(w.last("3001234401")["cuerpo"].(string), "una persona del taller te atiende") {
		t.Fatalf("no acepto: %v %v", conv, w.last("3001234401"))
	}
	before := len(w.sent("3001234401"))
	w.write("3001234401", "Jorge Peña", "¿A cómo la alineación?")
	if len(w.sent("3001234401")) != before || len(w.drafts()) != 1 {
		t.Fatal("tras rechazar, el mensaje pasó por el agente")
	}
}

// HU-48: Recibido → AU-01 y Listo → AU-03 con los datos reales; si la orden pasa dos veces por Listo, el aviso
// no se repite (criterio 2). HU-41: apagada, no sale nada.
func TestWhatsApp_StateNoticesOncePerOrder(t *testing.T) {
	e := newEnv(t)
	owner := e.register("3105550410", "Pablo Rodríguez")
	w := e.whatsapp(t, owner, nil)
	cid := w.customer("3001234410", "Laura Gómez")
	id, _ := w.orderFor(cid, "AVI410")

	w.move(id, "recibido", nil)
	au1 := w.sentBy("3001234410", "AU-01")
	if len(au1) != 1 || !strings.Contains(au1[0]["cuerpo"].(string), "Recibimos tu") || !strings.Contains(au1[0]["cuerpo"].(string), "AVI410") ||
		au1[0]["ordenId"] != id {
		t.Fatalf("AU-01: %v", au1)
	}
	w.move(id, "en_diagnostico", nil)
	q := "/api/v1/orders/" + id + "/quote"
	owner.must(http.StatusOK, owner.do("POST", q+"/lines", map[string]any{"tipo": "servicio", "descripcion": "Mano de obra", "cantidad": 1, "precioUnitario": 50000}))
	owner.must(http.StatusOK, owner.do("POST", q+"/send", nil))
	w.pump()
	owner.must(http.StatusOK, owner.do("POST", q+"/response", map[string]any{"decision": "aprobar", "canal": "mostrador"}))
	w.pump()
	w.move(id, "listo", nil)
	au3 := w.sentBy("3001234410", "AU-03")
	if len(au3) != 1 || !strings.Contains(au3[0]["cuerpo"].(string), "está listo") || !strings.Contains(au3[0]["cuerpo"].(string), "$") {
		t.Fatalf("AU-03: %v", au3)
	}
	// Se devuelve y vuelve a Listo: no se repite. Tampoco AU-01 si vuelve a pasar por Recibido.
	w.move(id, "en_reparacion", nil)
	w.move(id, "listo", nil)
	if n := len(w.sentBy("3001234410", "AU-03")); n != 1 {
		t.Fatalf("AU-03 salió %d veces", n)
	}

	// HU-41: con AU-03 apagada no sale nada en otra orden.
	owner.must(http.StatusOK, owner.do("PATCH", "/api/v1/automations/AU-03", map[string]any{"activa": false}))
	id2, _ := w.orderFor(cid, "AVI411")
	w.move(id2, "recibido", nil)
	w.move(id2, "en_diagnostico", nil)
	q2 := "/api/v1/orders/" + id2 + "/quote"
	owner.must(http.StatusOK, owner.do("POST", q2+"/lines", map[string]any{"tipo": "servicio", "descripcion": "Revisión", "cantidad": 1, "precioUnitario": 30000}))
	owner.must(http.StatusOK, owner.do("POST", q2+"/send", nil))
	owner.must(http.StatusOK, owner.do("POST", q2+"/response", map[string]any{"decision": "aprobar", "canal": "mostrador"}))
	w.pump()
	w.move(id2, "listo", nil)
	if n := len(w.sentBy("3001234410", "AU-03")); n != 1 {
		t.Fatalf("AU-03 apagada y salió (%d)", n)
	}
}

// HU-95: sin autorización no sale AU-01 y el dueño ve por qué. HU-47: fuera de la ventana de 24 h tampoco.
func TestWhatsApp_NoConsentOrClosedWindowSkips(t *testing.T) {
	e := newEnv(t)
	owner := e.register("3105550420", "Pablo Rodríguez")
	w := e.whatsapp(t, owner, nil)

	// Cliente registrado en el mostrador, sin autorización.
	r := owner.must(http.StatusCreated, owner.do("POST", "/api/v1/customers", map[string]any{"celular": "3001234420", "nombre": "Cliente Importado"}))
	id, _ := w.orderFor(nested(r.body, "cliente", "id").(string), "SIN420")
	w.move(id, "recibido", nil)
	runs := w.runsOf("AU-01")
	if len(runs) != 1 || runs[0]["resultado"] != "omitido" || !strings.Contains(*strOrNil(runs[0]["detalle"]), "no ha autorizado") {
		t.Fatalf("HU-95: %v", runs)
	}
	if len(w.sent("3001234420")) != 0 {
		t.Fatal("se le escribió a un cliente sin autorización")
	}

	// Cliente autorizado que escribió hace 25 h: fuera de la ventana.
	cid := w.customer("3001234421", "Laura Gómez")
	id2, _ := w.orderFor(cid, "VEN421")
	e.clk.Advance(25 * time.Hour)
	w.owner = e.login("3105550420") // la sesión dura 10 minutos
	w.move(id2, "recibido", nil)
	var closed map[string]any
	for _, x := range w.runsOf("AU-01") {
		if x["ordenId"] == id2 {
			closed = x
		}
	}
	if closed == nil || closed["resultado"] != "omitido" || !strings.Contains(*strOrNil(closed["detalle"]), "24 h") {
		t.Fatalf("HU-47: %v", closed)
	}
	if len(w.sentBy("3001234421", "AU-01")) != 0 {
		t.Fatal("salió AU-01 fuera de la ventana")
	}
}

func strOrNil(v any) *string {
	s, _ := v.(string)
	return &s
}

// HU-28 / HU-29 / HU-33: la cotización sale con Aprobar / Tengo dudas; "Aprobar" desde WhatsApp deja la orden
// En reparación y descuenta el repuesto; pulsarlo otra vez no descuenta dos veces; "Tengo dudas" pasa a una persona.
func TestWhatsApp_ApproveQuoteFromButton(t *testing.T) {
	e := newEnv(t)
	owner := e.register("3105550430", "Pablo Rodríguez")
	w := e.whatsapp(t, owner, nil)
	cid := w.customer("3001234430", "Laura Gómez")
	id, part := w.orderFor(cid, "COT430")
	w.move(id, "recibido", nil)
	w.move(id, "en_diagnostico", nil)
	q := "/api/v1/orders/" + id + "/quote"
	owner.must(http.StatusOK, owner.do("POST", q+"/lines", map[string]any{"tipo": "repuesto", "refId": part, "cantidad": 2}))
	owner.must(http.StatusOK, owner.do("POST", q+"/send", nil))
	w.pump()

	au2 := w.sentBy("3001234430", "AU-02")
	if len(au2) != 1 || !strings.Contains(au2[0]["cuerpo"].(string), "Filtro de aceite") || !strings.Contains(au2[0]["cuerpo"].(string), "¿La apruebas?") ||
		!hasPrefix(buttons(au2[0]), "quote:aprobar:"+id) || !hasPrefix(buttons(au2[0]), "quote:dudas:"+id) {
		t.Fatalf("AU-02: %v", au2)
	}
	w.press("3001234430", "Laura Gómez", "quote:aprobar:"+id, "Aprobar")
	o := owner.must(http.StatusOK, owner.do("GET", "/api/v1/orders/"+id, nil)).body
	cot := owner.must(http.StatusOK, owner.do("GET", q, nil)).body["cotizacion"].(map[string]any)
	stock := partStock(t, owner, "FA-COT430")
	if o["estado"] != "en_reparacion" || cot["estado"] != "aprobada" || cot["canalRespuesta"] != "whatsapp" {
		t.Fatalf("aprobada por WhatsApp: orden %v, cotización %v", o["estado"], cot)
	}
	if !strings.Contains(w.last("3001234430")["cuerpo"].(string), "Quedó aprobada") {
		t.Fatalf("respuesta: %v", w.last("3001234430"))
	}
	if stock != 3 {
		t.Fatalf("existencia tras aprobar 2 de 5: %v", stock)
	}

	// Pulsar Aprobar otra vez (el cliente insiste): ni error que trabe el worker ni doble descuento.
	w.press("3001234430", "Laura Gómez", "quote:aprobar:"+id, "Aprobar")
	if again := partStock(t, owner, "FA-COT430"); again != stock {
		t.Fatalf("doble descuento: %v → %v", stock, again)
	}
	if o := owner.must(http.StatusOK, owner.do("GET", "/api/v1/orders/"+id, nil)).body; o["estado"] != "en_reparacion" {
		t.Fatalf("la segunda pulsación movió la orden: %v", o["estado"])
	}

	// "Tengo dudas" en otra cotización: la conversación pasa a una persona y la cotización sigue esperando.
	id2, _ := w.orderFor(cid, "DUD431")
	w.move(id2, "recibido", nil)
	w.move(id2, "en_diagnostico", nil)
	q2 := "/api/v1/orders/" + id2 + "/quote"
	owner.must(http.StatusOK, owner.do("POST", q2+"/lines", map[string]any{"tipo": "servicio", "descripcion": "Revisión", "cantidad": 1, "precioUnitario": 30000}))
	owner.must(http.StatusOK, owner.do("POST", q2+"/send", nil))
	w.pump()
	w.press("3001234430", "Laura Gómez", "quote:dudas:"+id2, "Tengo dudas")
	conv, _ := w.conversation("3001234430")
	o2 := owner.must(http.StatusOK, owner.do("GET", "/api/v1/orders/"+id2, nil)).body
	if conv["modo"] != "humano" || o2["estado"] != "esperando_aprobacion" || !strings.Contains(w.last("3001234430")["cuerpo"].(string), "una persona del taller") {
		t.Fatalf("tengo dudas: %v %v %v", conv["modo"], o2["estado"], w.last("3001234430"))
	}
	_ = part
}

// flaky es un canal que falla según fail: definitivo (número inválido) o pasajero (límite de tasa).
type flaky struct {
	fail      int // cuántos envíos más fallan
	retryable bool
	sent      int
}

type chErr struct{ retry bool }

func (e chErr) Error() string   { return "whatsapp: respuesta 400 (código 131026)" }
func (e chErr) Retryable() bool { return e.retry }

func (f *flaky) send() (string, error) {
	if f.fail > 0 {
		f.fail--
		return "", chErr{f.retryable}
	}
	f.sent++
	return fmt.Sprintf("wamid.flaky.%d", f.sent), nil
}
func (f *flaky) SendText(context.Context, string, string, string) (string, error) { return f.send() }
func (f *flaky) SendButtons(context.Context, string, string, string, []domain.Button) (string, error) {
	return f.send()
}
func (f *flaky) SendTemplate(context.Context, string, string, messagingapp.Template) (string, error) {
	return f.send()
}
func (f *flaky) SendImage(context.Context, string, string, messagingapp.Image) (string, error) {
	return f.send()
}

// HU-46: si el envío falla del todo, el mensaje y la ejecución quedan fallidos, el dueño lo ve en "Hoy tienes
// pendiente" y la orden sigue funcionando (RNF-13). Si la falla es pasajera, se reintenta y sale.
func TestWhatsApp_FailedDeliveryIsVisibleAndRetried(t *testing.T) {
	e := newEnv(t)
	owner := e.register("3105550440", "Pablo Rodríguez")
	ch := &flaky{}
	w := e.whatsapp(t, owner, ch)
	cid := w.customer("3001234440", "Laura Gómez")

	// Falla definitiva en AU-01.
	ch.fail, ch.retryable = 1, false
	id, _ := w.orderFor(cid, "FAL440")
	w.move(id, "recibido", nil)
	au1 := w.sentBy("3001234440", "AU-01")
	if len(au1) != 1 || au1[0]["estado"] != "fallido" || au1[0]["error"] == nil {
		t.Fatalf("mensaje fallido: %v", au1)
	}
	if r := w.runsOf("AU-01"); len(r) != 1 || r[0]["resultado"] != "fallido" {
		t.Fatalf("ejecución: %v", r)
	}
	alert := false
	for _, p := range owner.must(http.StatusOK, owner.do("GET", "/api/v1/today", nil)).body["pendientes"].([]any) {
		m := p.(map[string]any)
		if m["tipo"] == "aviso_fallido" && m["ordenId"] == id {
			alert = true
		}
	}
	if !alert {
		t.Fatal("el dueño no ve la alerta del aviso que no salió")
	}
	w.move(id, "en_diagnostico", nil) // la orden sigue

	// Falla pasajera en AU-03 de otra orden: se reintenta con el retroceso del outbox y sale.
	id2, _ := w.orderFor(cid, "REI441")
	w.move(id2, "recibido", nil)
	w.move(id2, "en_diagnostico", nil)
	q := "/api/v1/orders/" + id2 + "/quote"
	owner.must(http.StatusOK, owner.do("POST", q+"/lines", map[string]any{"tipo": "servicio", "descripcion": "Revisión", "cantidad": 1, "precioUnitario": 30000}))
	owner.must(http.StatusOK, owner.do("POST", q+"/send", nil))
	owner.must(http.StatusOK, owner.do("POST", q+"/response", map[string]any{"decision": "aprobar", "canal": "mostrador"}))
	w.pump()
	ch.fail, ch.retryable = 2, true
	w.move(id2, "listo", nil)
	if au3 := w.sentBy("3001234440", "AU-03"); len(au3) != 1 || au3[0]["estado"] != "pendiente" {
		t.Fatalf("tras la primera falla pasajera debía seguir pendiente: %v", au3)
	}
	for range 2 { // dos reintentos (retroceso exponencial del outbox: 1 s, 2 s…)
		e.clk.Advance(30 * time.Second)
		w.pump()
	}
	w.owner = e.login("3105550440") // la sesión de 10 minutos ya pasó con tantos pasos
	if au3 := w.sentBy("3001234440", "AU-03"); len(au3) != 1 || au3[0]["estado"] != "enviado" {
		t.Fatalf("tras reintentar debía salir: %v", au3)
	}
}

// Webhook de Meta: firma válida → 200 y queda en la cola; firma inválida o ausente → 401; verificación de la
// suscripción solo con el token correcto (R-05, R-09).
func TestWhatsApp_WebhookSignature(t *testing.T) {
	e := newEnv(t)
	r := chi.NewRouter()
	wire.Public(e.db.API, config.API{WhatsApp: config.WhatsApp{AppSecret: "secreto", VerifyToken: "tok"}}, e.log)(r)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	body, _ := meta.SimulatedBody("PNID-X", "+573001112233", "Laura", "hola", "", e.clk.Now())
	mac := hmac.New(sha256.New, []byte("secreto"))
	mac.Write(body)
	post := func(sig string) int {
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/webhooks/whatsapp", bytes.NewReader(body))
		if sig != "" {
			req.Header.Set(meta.SignatureHeader, sig)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		return res.StatusCode
	}
	if s := post("sha256=" + hex.EncodeToString(mac.Sum(nil))); s != http.StatusOK {
		t.Fatalf("firma válida: %d", s)
	}
	if s := post("sha256=00"); s != http.StatusUnauthorized {
		t.Fatalf("firma inválida: %d", s)
	}
	if s := post(""); s != http.StatusUnauthorized {
		t.Fatalf("sin firma: %d", s)
	}
	get := func(q string) (int, string) {
		res, err := http.Get(srv.URL + "/webhooks/whatsapp?" + q)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		return res.StatusCode, string(b)
	}
	if s, b := get("hub.mode=subscribe&hub.verify_token=tok&hub.challenge=987"); s != 200 || b != "987" {
		t.Fatalf("verificación: %d %q", s, b)
	}
	if s, _ := get("hub.mode=subscribe&hub.verify_token=otro&hub.challenge=987"); s != http.StatusForbidden {
		t.Fatalf("token malo: %d", s)
	}
}
