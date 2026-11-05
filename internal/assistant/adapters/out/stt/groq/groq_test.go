package groq

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// La petición lleva el audio como archivo .ogg, el modelo, el idioma y la clave; la respuesta es el texto. Un error de
// Groq no trae su cuerpo (puede traer el audio o la clave).
func TestTranscribe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openai/v1/audio/transcriptions" || r.Header.Get("Authorization") != "Bearer clave" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		f, h, err := r.FormFile("file")
		if err != nil {
			t.Errorf("sin archivo: %v", err)
			return
		}
		data, _ := io.ReadAll(f)
		if h.Filename != "nota.ogg" || string(data) != "OggS-audio" || r.FormValue("model") != "whisper-large-v3-turbo" || r.FormValue("language") != "es" {
			t.Errorf("petición: %s %q %s %s", h.Filename, data, r.FormValue("model"), r.FormValue("language"))
		}
		_, _ = io.WriteString(w, `{"text":"  ¿A cómo el cambio de aceite?  "}`)
	}))
	t.Cleanup(srv.Close)
	c := New(srv.URL+"/openai/v1/", "clave", "whisper-large-v3-turbo", 5*time.Second)
	text, err := c.Transcribe(context.Background(), []byte("OggS-audio"), "audio/ogg; codecs=opus")
	if err != nil || text != "¿A cómo el cambio de aceite?" {
		t.Fatalf("transcripción: %q %v", text, err)
	}
	bad := New(srv.URL+"/openai/v1", "otra", "m", 5*time.Second)
	if _, err := bad.Transcribe(context.Background(), []byte("x"), "audio/ogg"); err == nil || strings.Contains(err.Error(), "otra") {
		t.Fatalf("error sin la clave: %v", err)
	}
	if _, err := c.Transcribe(context.Background(), nil, "audio/ogg"); err == nil {
		t.Fatal("audio vacío")
	}
}
