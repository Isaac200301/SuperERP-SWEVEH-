package server_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"sweveh/internal/assistant/adapters/out/stt/groq"
	"sweveh/internal/platform/config"
	"sweveh/internal/wire"
)

// HU-60 (ADR-0033): una nota de voz por WhatsApp se descarga de WAHA, se transcribe con Groq y sigue el flujo del
// texto; la conversación muestra la transcripción y el audio se escucha desde el panel. Si Groq falla, queda la
// tarjeta para que el dueño la escuche.
func TestWhatsApp_VoiceNoteTranscribed(t *testing.T) {
	f := &fakeWAHA{status: "WORKING"}
	wahaSrv := httptest.NewServer(f.handler(t))
	t.Cleanup(wahaSrv.Close)
	var groqFails atomic.Bool
	groqSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		file, _, err := r.FormFile("file")
		if err != nil {
			t.Errorf("sin audio: %v", err)
			return
		}
		data, _ := io.ReadAll(file)
		if groqFails.Load() || string(data) != "OggS-nota-de-voz" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = io.WriteString(w, `{"text":"Hola, ¿a cómo el cambio de aceite para la moto?"}`)
	}))
	t.Cleanup(groqSrv.Close)

	cfg := config.API{Env: config.EnvLocal, WhatsApp: config.WhatsApp{Provider: "local",
		WAHA: config.WAHA{URL: wahaSrv.URL, APIKey: "clave-waha", WebhookSecret: "secreto-hook", Session: "default"}}}
	e := newEnvWith(t, cfg)
	owner := e.register("3105550870", "Pablo Rodríguez")
	owner.must(http.StatusOK, owner.do("PUT", "/api/v1/whatsapp/channel", map[string]any{"proveedor": "waha"}))
	w := e.whatsapp(t, owner, wire.Channel(cfg.WhatsApp))
	w.s.Assistant.SetTranscriber(groq.New(groqSrv.URL, "k", "whisper-large-v3-turbo", 5*time.Second))

	const chat, phone = "573001234870@c.us", "3001234870"
	n := 0
	post := func(payload string) {
		t.Helper()
		e.clk.Advance(time.Minute)
		n++
		body := fmt.Sprintf(`{"event":"message","session":"default","engine":"GOWS","payload":{"id":"false_%s_IN%d","timestamp":%d,"from":%q,`+
			`"fromMe":false,%s,"_data":{"notifyName":"Laura Gómez"}}}`, chat, n, e.clk.Now().Unix(), chat, payload)
		if s := postWAHA(t, e, []byte(body), "secreto-hook"); s != http.StatusOK {
			t.Fatalf("webhook: %d", s)
		}
		w.pump()
	}
	text := func(s string) { post(fmt.Sprintf(`"body":%q,"hasMedia":false`, s)) }
	voice := func() {
		post(`"body":"","hasMedia":true,"media":{"mimetype":"audio/ogg; codecs=opus","url":"https://waha-publico.example/api/files/default/nota.oga"}`)
	}
	text("Hola")
	text("1") // acepta la autorización

	voice()
	conv, msgs := w.conversation(phone)
	var audio map[string]any
	for _, m := range msgs {
		if m["tipo"] == "audio" {
			audio = m
		}
	}
	if audio == nil || !strings.Contains(audio["cuerpo"].(string), "cambio de aceite") {
		t.Fatalf("la nota de voz quedó con su transcripción: %v", msgs)
	}
	var found bool
	for _, d := range w.drafts() {
		if strings.Contains(d["mensajeCliente"].(string), "cambio de aceite") {
			found = d["notaDeVoz"] == true && d["mensajeId"] == audio["id"]
		}
	}
	if !found {
		t.Fatalf("la transcripción siguió el flujo del texto: %v", w.drafts())
	}

	// El audio se escucha desde el panel (se descarga de WAHA la primera vez y queda guardado).
	for range 2 {
		req, _ := http.NewRequest("GET", e.srv.URL+"/api/v1/conversations/"+conv["id"].(string)+"/messages/"+audio["id"].(string)+"/audio", nil)
		res, err := owner.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		if res.StatusCode != http.StatusOK || string(body) != "OggS-nota-de-voz" || !strings.HasPrefix(res.Header.Get("Content-Type"), "audio/ogg") {
			t.Fatalf("audio: %d %q %s", res.StatusCode, body, res.Header.Get("Content-Type"))
		}
	}

	// Si Groq falla, queda la tarjeta de nota de voz para el dueño.
	groqFails.Store(true)
	voice()
	var card bool
	for _, d := range w.drafts() {
		if d["intencion"] == "nota_de_voz" {
			card = true
		}
	}
	if !card {
		t.Fatalf("sin transcripción, tarjeta para el dueño: %v", w.drafts())
	}
}
