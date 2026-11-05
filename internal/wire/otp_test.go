package wire

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"sweveh/internal/platform/config"
)

// ADR-0034: el código sale por WAHA a la sesión configurada, al chat del celular, con el código y su vencimiento. Si
// WAHA falla, el error no trae el código (docs/08 §6).
func TestWAHAOTP(t *testing.T) {
	var got map[string]any
	fail := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "clave" || r.URL.Path != "/api/sendText" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if fail {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"id":"true_x_OUT1"}`))
	}))
	t.Cleanup(srv.Close)
	o := NewWAHAOTP(config.WAHA{URL: srv.URL, APIKey: "clave", WebhookSecret: "s", Session: "default"})

	if err := o.Send(context.Background(), "+573177510761", "778003"); err != nil {
		t.Fatal(err)
	}
	text, _ := got["text"].(string)
	if got["session"] != "default" || got["chatId"] != "573177510761@c.us" || !strings.Contains(text, "778003") || !strings.Contains(text, "10 minutos") {
		t.Fatalf("envío a WAHA: %v", got)
	}
	fail = true
	err := o.Send(context.Background(), "+573177510761", "778003")
	if err == nil || strings.Contains(err.Error(), "778003") || strings.Contains(err.Error(), "3177510761") {
		t.Fatalf("el error no debe traer el código ni el celular: %v", err)
	}
}
