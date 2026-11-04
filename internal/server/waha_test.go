package server_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"sweveh/internal/platform/config"
	"sweveh/internal/wire"
)

// fakeWAHA imita la API de WAHA: guarda lo que se envía y responde la sesión como vinculada.
type fakeWAHA struct {
	mu     sync.Mutex
	sent   []map[string]any
	status string
	n      int
}

func (f *fakeWAHA) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "clave-waha" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		switch {
		case r.URL.Path == "/api/sendText":
			var b map[string]any
			_ = json.NewDecoder(r.Body).Decode(&b)
			f.sent = append(f.sent, b)
			f.n++
			_, _ = fmt.Fprintf(w, `{"id":"true_%s_OUT%d"}`, b["chatId"], f.n)
		case r.URL.Path == "/api/sessions/default":
			_, _ = fmt.Fprintf(w, `{"name":"default","status":%q}`, f.status)
		case r.URL.Path == "/api/default/auth/qr":
			_, _ = io.WriteString(w, `{"mimetype":"image/png","data":"`+base64.StdEncoding.EncodeToString([]byte("png"))+`"}`)
		default:
			_, _ = io.WriteString(w, `{}`)
		}
	}
}

func (f *fakeWAHA) texts(chat string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, b := range f.sent {
		if b["chatId"] == chat && b["session"] == "default" {
			out = append(out, b["text"].(string))
		}
	}
	return out
}

// ADR-0027: con el número de prueba de WAHA, el cliente escribe desde WhatsApp (webhook firmado), recibe la
// autorización como menú numerado y responde "1"; queda autorizado y su mensaje pasa al agente. La cotización
// se aprueba escribiendo "aprobar". Firma mala → 401. El simulador no se mezcla con el número real.
func TestWAHA_DemoNumber(t *testing.T) {
	f := &fakeWAHA{status: "SCAN_QR_CODE"}
	waha := httptest.NewServer(f.handler(t))
	t.Cleanup(waha.Close)
	cfg := config.API{Env: config.EnvLocal, WhatsApp: config.WhatsApp{Provider: "local",
		WAHA: config.WAHA{URL: waha.URL, APIKey: "clave-waha", WebhookSecret: "secreto-hook", Session: "default"}}}
	e := newEnvWith(t, cfg)
	owner := e.register("3105550600", "Pablo Rodríguez")

	// Mi taller: estado de la sesión con el QR para escanear, y el taller pasa a usar el número de WAHA.
	st := owner.must(http.StatusOK, owner.do("GET", "/api/v1/whatsapp/waha", nil)).body
	if st["configurado"] != true || st["estado"] != "SCAN_QR_CODE" || !strings.HasPrefix(st["qr"].(string), "data:image/png;base64,") {
		t.Fatalf("estado: %v", st)
	}
	owner.must(http.StatusOK, owner.do("POST", "/api/v1/whatsapp/waha/start", nil))
	f.status = "WORKING"
	ch := owner.must(http.StatusOK, owner.do("PUT", "/api/v1/whatsapp/channel", map[string]any{"proveedor": "waha"})).body
	if ch["canal"].(map[string]any)["proveedor"] != "waha" {
		t.Fatalf("canal: %v", ch)
	}
	if r := owner.do("POST", "/api/v1/simulator/inbound", map[string]any{"celular": "3001234600", "texto": "hola"}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("simulador con número real: %d", r.status)
	}

	w := e.whatsapp(t, owner, wire.Channel(cfg.WhatsApp))
	const chat = "573001234600@c.us"
	n := 0
	write := func(text string) {
		t.Helper()
		e.clk.Advance(time.Minute) // WhatsApp manda la hora en segundos: un minuto entre mensajes, como en el simulador
		n++
		body := fmt.Sprintf(`{"event":"message","session":"default","engine":"GOWS","payload":{"id":"false_%s_IN%d",
			"timestamp":%d,"from":%q,"fromMe":false,"body":%q,"hasMedia":false,"_data":{"notifyName":"Laura Gómez"}}}`,
			chat, n, e.clk.Now().Unix(), chat, text)
		if s := postWAHA(t, e, []byte(body), "secreto-hook"); s != http.StatusOK {
			t.Fatalf("webhook: %d", s)
		}
		w.pump()
	}

	write("Hola, ¿a cómo el cambio de aceite?")
	got := f.texts(chat)
	if len(got) != 1 || !strings.Contains(got[0], "aceptas el tratamiento de tus datos") || !strings.Contains(got[0], "Responde con el número:\n1. ") {
		t.Fatalf("autorización por WAHA: %q", got)
	}
	write("1")
	got = f.texts(chat)
	if len(got) != 2 || !strings.Contains(got[1], "Ya quedó registrada tu autorización") {
		t.Fatalf("tras responder 1: %q", got)
	}
	c := owner.must(http.StatusOK, owner.do("GET", "/api/v1/customers/by-phone/3001234600", nil)).body
	if c["autorizacionEstado"] != "autorizada" {
		t.Fatalf("cliente: %v", c)
	}
	if ds := w.drafts(); len(ds) != 1 || !strings.Contains(ds[0]["mensajeCliente"].(string), "cambio de aceite") {
		t.Fatalf("el mensaje original no pasó al agente: %v", ds)
	}
	// La respuesta queda guardada como botón, y el número del cliente queda en formato normal.
	_, msgs := w.conversation("3001234600")
	if msgs == nil {
		t.Fatal("no hay conversación con 3001234600")
	}
	var reply map[string]any
	for _, m := range msgs {
		if m["cuerpo"] == "1" {
			reply = m
		}
	}
	if reply == nil || reply["tipo"] != "respuesta_boton" {
		t.Fatalf("respuesta 1: %v", reply)
	}

	// Entregas: message.ack con el id que devolvió WAHA marca el mensaje como leído.
	ack := fmt.Sprintf(`{"event":"message.ack","session":"default","payload":{"id":"true_%s_OUT2","from":%q,"fromMe":true,"ack":3}}`, chat, chat)
	if s := postWAHA(t, e, []byte(ack), "secreto-hook"); s != http.StatusOK {
		t.Fatalf("ack: %d", s)
	}
	w.pump()
	if last := w.last("3001234600"); last["estado"] != "leido" {
		t.Fatalf("estado tras el ack: %v", last["estado"])
	}

	// Un texto que no es una opción sigue al agente como texto normal.
	write("¿y abren el sábado?")
	if _, msgs := w.conversation("3001234600"); msgs[len(msgs)-1]["tipo"] == "respuesta_boton" {
		t.Fatal("un texto libre no es una opción")
	}

	// HU-28 por WAHA: la cotización sale con el menú y el cliente la aprueba escribiendo "aprobar".
	id, _ := w.orderFor(c["id"].(string), "WAH600")
	w.move(id, "recibido", nil)
	w.move(id, "en_diagnostico", nil)
	q := "/api/v1/orders/" + id + "/quote"
	owner.must(http.StatusOK, owner.do("POST", q+"/lines", map[string]any{"tipo": "servicio", "descripcion": "Cambio de aceite", "cantidad": 1, "precioUnitario": 45000}))
	owner.must(http.StatusOK, owner.do("POST", q+"/send", nil))
	w.pump()
	got = f.texts(chat)
	if !strings.Contains(got[len(got)-1], "¿La apruebas?") || !strings.Contains(got[len(got)-1], "Responde con el número:") {
		t.Fatalf("cotización por WAHA: %q", got[len(got)-1])
	}
	write("Aprobar")
	if o := owner.must(http.StatusOK, owner.do("GET", "/api/v1/orders/"+id, nil)).body; o["estado"] != "en_reparacion" {
		t.Fatalf("tras escribir aprobar: %v", o["estado"])
	}

	if s := postWAHA(t, e, []byte(`{"event":"message","session":"default","payload":{}}`), "otra-clave"); s != http.StatusUnauthorized {
		t.Fatalf("firma mala: %d", s)
	}
}

func postWAHA(t *testing.T, e *env, body []byte, secret string) int {
	t.Helper()
	mac := hmac.New(sha512.New, []byte(secret))
	mac.Write(body)
	req, _ := http.NewRequest(http.MethodPost, e.srv.URL+"/webhooks/waha", bytes.NewReader(body))
	req.Header.Set("X-Webhook-Hmac", hex.EncodeToString(mac.Sum(nil)))
	req.Header.Set("X-Webhook-Hmac-Algorithm", "sha512")
	res, err := e.srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	return res.StatusCode
}
