package waha

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sweveh/internal/messaging/app"
	"sweveh/internal/messaging/domain"
)

type call struct {
	method, path, key string
	body              map[string]any
}

// fake imita la API de WAHA: guarda cada petición y responde lo que diga reply.
func fake(t *testing.T, reply func(w http.ResponseWriter, r *http.Request)) (*Client, *[]call) {
	t.Helper()
	var calls []call
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := call{method: r.Method, path: r.URL.Path, key: r.Header.Get("X-Api-Key")}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &c.body)
		calls = append(calls, c)
		reply(w, r)
	}))
	t.Cleanup(srv.Close)
	return New(srv.URL+"/", "clave-waha", time.Second), &calls
}

func ok(id string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, `{"id":`+id+`}`) }
}

func TestSend(t *testing.T) {
	c, calls := fake(t, ok(`"true_573001234567@c.us_AAA"`))
	ctx := context.Background()

	id, err := c.SendText(ctx, "waha:default", "+573001234567", "Hola")
	if err != nil || id != "true_573001234567@c.us_AAA" {
		t.Fatalf("%q %v", id, err)
	}
	b := []domain.Button{{ID: domain.BtnConsentYes, Title: "Acepto"}, {ID: domain.BtnConsentNo, Title: "No acepto"}}
	if _, err := c.SendButtons(ctx, "waha:taller2", "+573001234567", "¿Nos autorizas?", b); err != nil {
		t.Fatal(err)
	}
	if _, err := c.SendImage(ctx, "waha:default", "123@lid", app.Image{Data: []byte("jpg"), MIME: "image/jpeg", Caption: "Recibido"}); err != nil {
		t.Fatal(err)
	}

	got := *calls
	if len(got) != 3 {
		t.Fatalf("%d llamadas", len(got))
	}
	if got[0].method != http.MethodPost || got[0].path != "/api/sendText" || got[0].key != "clave-waha" ||
		got[0].body["session"] != "default" || got[0].body["chatId"] != "573001234567@c.us" || got[0].body["text"] != "Hola" {
		t.Fatalf("sendText: %+v", got[0])
	}
	// Sin botones en WAHA: las opciones van como menú numerado en el texto.
	if got[1].body["session"] != "taller2" || got[1].body["text"] != domain.Menu("¿Nos autorizas?", b) {
		t.Fatalf("botones: %+v", got[1])
	}
	file, _ := got[2].body["file"].(map[string]any)
	if got[2].path != "/api/sendImage" || got[2].body["chatId"] != "123@lid" || got[2].body["caption"] != "Recibido" ||
		file["mimetype"] != "image/jpeg" || file["data"] != base64.StdEncoding.EncodeToString([]byte("jpg")) {
		t.Fatalf("sendImage: %+v", got[2])
	}

	if _, err := c.SendTemplate(ctx, "waha:default", "+57", app.Template{}); !errors.Is(err, app.ErrUnsupported) {
		t.Fatalf("plantilla: %v", err)
	}
	if _, err := c.SendButtons(ctx, "waha:default", "+57", "x", []domain.Button{{ID: "a"}}); !errors.Is(err, domain.ErrButtons) {
		t.Fatalf("botón inválido: %v", err)
	}
}

func TestSend_IDFormats(t *testing.T) {
	c, _ := fake(t, ok(`{"fromMe":true,"_serialized":"true_57@c.us_BBB"}`))
	if id, err := c.SendText(context.Background(), "waha:default", "+57", "x"); err != nil || id != "true_57@c.us_BBB" {
		t.Fatalf("%q %v", id, err)
	}
	c, _ = fake(t, ok(`null`))
	if _, err := c.SendText(context.Background(), "waha:default", "+57", "x"); err == nil {
		t.Fatal("sin id debe fallar")
	}
}

func TestSend_Errors(t *testing.T) {
	for _, c := range []struct {
		status    int
		retryable bool
	}{{http.StatusTooManyRequests, true}, {http.StatusBadGateway, true}, {http.StatusUnprocessableEntity, false}} {
		cl, _ := fake(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(c.status)
			_, _ = io.WriteString(w, `{"message":"no se pudo enviar a +573001234567 clave-waha"}`)
		})
		_, err := cl.SendText(context.Background(), "waha:default", "+573001234567", "x")
		var e *APIError
		if !errors.As(err, &e) || e.Status != c.status || app.IsRetryable(err) != c.retryable {
			t.Fatalf("%d: %v", c.status, err)
		}
		// El error no repite el cuerpo: puede traer el número del cliente.
		if strings.Contains(err.Error(), "3001234567") || strings.Contains(err.Error(), "clave") {
			t.Fatalf("el error filtra datos: %v", err)
		}
	}
	cl, _ := fake(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "<html>") })
	if _, err := cl.SendText(context.Background(), "waha:default", "+57", "x"); err == nil {
		t.Fatal("respuesta que no es JSON")
	}
}

func TestSend_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
		}
	}))
	defer srv.Close()
	_, err := New(srv.URL, "k", 50*time.Millisecond).SendText(context.Background(), "waha:default", "+57", "x")
	if err == nil || !app.IsRetryable(err) {
		t.Fatalf("timeout: %v", err)
	}
}

func TestSession(t *testing.T) {
	status := "NOT_FOUND"
	c, calls := fake(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/sessions/default":
			if status == "NOT_FOUND" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = io.WriteString(w, `{"name":"default","status":"`+status+`","me":{"id":"573009999999@c.us","pushName":"Taller"}}`)
		case r.URL.Path == "/api/default/auth/qr":
			_, _ = io.WriteString(w, `{"mimetype":"image/png","data":"`+base64.StdEncoding.EncodeToString([]byte("png"))+`"}`)
		default:
			_, _ = io.WriteString(w, `{}`)
		}
	})
	ctx := context.Background()

	if err := c.StartSession(ctx, "default"); err != nil {
		t.Fatal(err)
	}
	last := (*calls)[len(*calls)-1]
	if last.method != http.MethodPost || last.path != "/api/sessions" || last.body["name"] != "default" || last.body["start"] != true {
		t.Fatalf("crear: %+v", last)
	}
	status = "STOPPED"
	if err := c.StartSession(ctx, "default"); err != nil || (*calls)[len(*calls)-1].path != "/api/sessions/default/start" {
		t.Fatalf("arrancar: %v %+v", err, (*calls)[len(*calls)-1])
	}
	status = "WORKING"
	n := len(*calls)
	if err := c.StartSession(ctx, "default"); err != nil || len(*calls) != n+1 {
		t.Fatal("si ya funciona no se vuelve a arrancar")
	}
	st, err := c.SessionStatus(ctx, "default")
	if err != nil || st.Status != "WORKING" || st.Me == nil || st.Me.PushName != "Taller" {
		t.Fatalf("%+v %v", st, err)
	}
	png, err := c.QR(ctx, "default")
	if err != nil || string(png) != "png" {
		t.Fatalf("qr: %q %v", png, err)
	}
}

// Ejemplo de firma de la documentación oficial (https://waha.devlike.pro/docs/how-to/events/, "HMAC
// authentication", consultada el 2026-11-04).
func TestValidSignature(t *testing.T) {
	body := []byte(`{"event":"message","session":"default","engine":"WEBJS"}`)
	const sig = "208f8a55dde9e05519e898b10b89bf0d0b3b0fdf11fdbf09b6b90476301b98d8097c462b2b17a6ce93b6b47a136cf2e78a33a63f6752c2c1631777076153fa89"
	if !ValidSignature("my-secret-key", body, sig) {
		t.Fatal("la firma del ejemplo oficial debe ser válida")
	}
	for name, c := range map[string]struct{ secret, header string }{
		"otra clave":  {"otra", sig},
		"sin clave":   {"", sig},
		"sin firma":   {"my-secret-key", ""},
		"no es hex":   {"my-secret-key", "zz"},
		"otro cuerpo": {"my-secret-key", strings.Replace(sig, "208f", "308f", 1)},
	} {
		if ValidSignature(c.secret, body, c.header) {
			t.Errorf("%s: no debe validar", name)
		}
	}
}

// Cuerpos con la forma de los ejemplos de https://waha.devlike.pro/docs/how-to/events/ (consultada el
// 2026-11-04), con números de Colombia.
func TestParse(t *testing.T) {
	msg := `{"id":"evt_01","timestamp":1741249702485,"event":"message","session":"default","engine":"GOWS",
		"me":{"id":"573009999999@c.us","pushName":"Taller"},
		"payload":{"id":"false_573001234567@c.us_AAAA","timestamp":1667561485,"from":"573001234567@c.us","fromMe":false,
		"source":"app","to":"573009999999@c.us","body":"¿cuánto el cambio de aceite?","hasMedia":false,"ack":1,"vCards":[],
		"_data":{"notifyName":"Laura Gómez"}}}`
	ins, sts, err := Parser{}.Parse([]byte(msg))
	if err != nil || len(sts) != 0 || len(ins) != 1 {
		t.Fatalf("%v %v %v", ins, sts, err)
	}
	want := app.Inbound{PhoneNumberID: "waha:default", From: "+573001234567", WAMID: "false_573001234567@c.us_AAAA", Kind: "texto",
		Text: "¿cuánto el cambio de aceite?", At: time.Unix(1667561485, 0).UTC(), Name: "Laura Gómez"}
	if ins[0] != want {
		t.Fatalf("%+v", ins[0])
	}

	ack := `{"event":"message.ack","session":"default","engine":"WEBJS","payload":{"id":"true_573001234567@c.us_4CC5",
		"from":"573001234567@c.us","participant":null,"fromMe":true,"ack":3,"ackName":"READ"}}`
	_, sts, _ = Parser{}.Parse([]byte(ack))
	if len(sts) != 1 || sts[0] != (app.Status{PhoneNumberID: "waha:default", WAMID: "true_573001234567@c.us_4CC5", State: "leido"}) {
		t.Fatalf("%+v", sts)
	}

	for name, body := range map[string]string{
		"mensaje propio": strings.Replace(msg, `"fromMe":false`, `"fromMe":true`, 1),
		"grupo":          strings.ReplaceAll(msg, "573001234567@c.us", "120363@g.us"),
		"estado":         strings.ReplaceAll(msg, "573001234567@c.us", "status@broadcast"),
		"otro evento":    strings.Replace(msg, `"event":"message"`, `"event":"message.reaction"`, 1),
		"ack pendiente":  strings.Replace(ack, `"ack":3`, `"ack":0`, 1),
	} {
		if ins, sts, err := (Parser{}).Parse([]byte(body)); err != nil || len(ins)+len(sts) != 0 {
			t.Errorf("%s: %v %v %v", name, ins, sts, err)
		}
	}

	media := strings.Replace(msg, `"hasMedia":false`, `"hasMedia":true,"media":{"mimetype":"audio/ogg; codecs=opus"}`, 1)
	if ins, _, _ := (Parser{}).Parse([]byte(media)); len(ins) != 1 || ins[0].Kind != "audio" {
		t.Fatalf("audio: %+v", ins)
	}
	media = strings.Replace(msg, `"hasMedia":false`, `"hasMedia":true`, 1)
	if ins, _, _ := (Parser{}).Parse([]byte(media)); len(ins) != 1 || ins[0].Kind != "imagen" {
		t.Fatalf("imagen: %+v", ins)
	}
	lid := strings.ReplaceAll(msg, "573001234567@c.us", "9999@lid")
	if ins, _, _ := (Parser{}).Parse([]byte(lid)); len(ins) != 1 || ins[0].From != "9999@lid" || ChatID(ins[0].From) != "9999@lid" {
		t.Fatalf("lid: %+v", ins)
	}
	if _, _, err := (Parser{}).Parse([]byte("{")); err == nil {
		t.Fatal("JSON roto")
	}

	if !Is([]byte(msg)) || Is([]byte(`{"object":"whatsapp_business_account","entry":[]}`)) || Is([]byte("x")) {
		t.Fatal("Is")
	}
}
