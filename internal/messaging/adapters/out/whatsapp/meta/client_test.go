package meta_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sweveh/internal/messaging/adapters/out/whatsapp/meta"
	"sweveh/internal/messaging/app"
	"sweveh/internal/messaging/domain"
)

// graph imita la Graph API: guarda cada petición y responde con lo que diga reply.
type graph struct {
	reqs  []captured
	reply func(path string) (int, string)
}

type captured struct {
	path, auth, contentType string
	body                    []byte
}

func newGraph(t *testing.T, reply func(path string) (int, string)) (*graph, *meta.Client) {
	t.Helper()
	g := &graph{reply: reply}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		g.reqs = append(g.reqs, captured{r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("Content-Type"), b})
		status, body := g.reply(r.URL.Path)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return g, meta.New(srv.URL+"/v21.0/", "token-de-prueba", 2*time.Second)
}

func ok(string) (int, string) {
	return 200, `{"messaging_product":"whatsapp","messages":[{"id":"wamid.ABC"}]}`
}

func bodyOf(t *testing.T, c captured) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(c.body, &m); err != nil {
		t.Fatalf("cuerpo no es JSON: %s", c.body)
	}
	return m
}

func TestClient_TextButtonsAndTemplate(t *testing.T) {
	g, c := newGraph(t, ok)
	ctx := context.Background()

	id, err := c.SendText(ctx, "123", "+573001112233", "Hola Laura")
	if err != nil || id != "wamid.ABC" {
		t.Fatalf("texto: %q %v", id, err)
	}
	r := g.reqs[0]
	if r.path != "/v21.0/123/messages" || r.auth != "Bearer token-de-prueba" || r.contentType != "application/json" {
		t.Fatalf("petición: %+v", r)
	}
	b := bodyOf(t, r)
	if b["messaging_product"] != "whatsapp" || b["to"] != "573001112233" || b["type"] != "text" ||
		b["text"].(map[string]any)["body"] != "Hola Laura" {
		t.Fatalf("texto: %v", b)
	}

	_, err = c.SendButtons(ctx, "123", "+573001112233", "¿Apruebas?", []domain.Button{{ID: "quote:aprobar:1", Title: "Aprobar"}})
	if err != nil {
		t.Fatal(err)
	}
	b = bodyOf(t, g.reqs[1])
	inter := b["interactive"].(map[string]any)
	btn := inter["action"].(map[string]any)["buttons"].([]any)[0].(map[string]any)["reply"].(map[string]any)
	if b["type"] != "interactive" || inter["type"] != "button" || btn["id"] != "quote:aprobar:1" || btn["title"] != "Aprobar" {
		t.Fatalf("botones: %v", b)
	}
	// Más de 3 botones no llega a Meta (DR-08).
	four := []domain.Button{{ID: "a", Title: "a"}, {ID: "b", Title: "b"}, {ID: "c", Title: "c"}, {ID: "d", Title: "d"}}
	if _, err := c.SendButtons(ctx, "123", "+573001112233", "x", four); !errors.Is(err, domain.ErrButtons) {
		t.Fatalf("4 botones: %v", err)
	}

	_, err = c.SendTemplate(ctx, "123", "+573001112233", app.Template{Name: "vehiculo_listo", Language: "es_CO", Params: []string{"Laura", "ABC123"}})
	if err != nil {
		t.Fatal(err)
	}
	b = bodyOf(t, g.reqs[2])
	tpl := b["template"].(map[string]any)
	params := tpl["components"].([]any)[0].(map[string]any)["parameters"].([]any)
	if b["type"] != "template" || tpl["name"] != "vehiculo_listo" || tpl["language"].(map[string]any)["code"] != "es_CO" ||
		len(params) != 2 || params[1].(map[string]any)["text"] != "ABC123" {
		t.Fatalf("plantilla: %v", b)
	}
	if len(g.reqs) != 3 {
		t.Fatalf("peticiones: %d", len(g.reqs))
	}
}

func TestClient_ImageIsUploadedThenSentByID(t *testing.T) {
	g, c := newGraph(t, func(path string) (int, string) {
		if strings.HasSuffix(path, "/media") {
			return 200, `{"id":"media-77"}`
		}
		return ok(path)
	})
	jpg := []byte{0xFF, 0xD8, 0xFF, 0xE0}
	if _, err := c.SendImage(context.Background(), "123", "+573001112233", app.Image{Data: jpg, MIME: "image/jpeg", Caption: "Frente"}); err != nil {
		t.Fatal(err)
	}
	up := g.reqs[0]
	if up.path != "/v21.0/123/media" || !strings.HasPrefix(up.contentType, "multipart/form-data") ||
		!strings.Contains(string(up.body), `name="messaging_product"`) || !strings.Contains(string(up.body), string(jpg)) {
		t.Fatalf("subida: %s %s", up.path, up.contentType)
	}
	img := bodyOf(t, g.reqs[1])["image"].(map[string]any)
	if img["id"] != "media-77" || img["caption"] != "Frente" {
		t.Fatalf("imagen: %v", img)
	}
}

func TestClient_ErrorsAreTypedWithoutPersonalData(t *testing.T) {
	cases := []struct {
		status    int
		body      string
		retryable bool
		window    bool
	}{
		{400, `{"error":{"message":"Recipient +573001112233 not in allowed list","code":131030}}`, false, false},
		{400, `{"error":{"message":"Re-engagement message","code":131047}}`, false, true},
		{429, `{"error":{"message":"Rate limit hit","code":130429}}`, true, false},
		{500, `{"error":{"message":"Something went wrong","code":131000}}`, true, false},
		{502, `no es json`, true, false},
	}
	for _, tc := range cases {
		_, c := newGraph(t, func(string) (int, string) { return tc.status, tc.body })
		_, err := c.SendText(context.Background(), "123", "+573001112233", "hola")
		var apiErr *meta.APIError
		if !errors.As(err, &apiErr) || apiErr.Status != tc.status {
			t.Fatalf("%d: %v", tc.status, err)
		}
		if app.IsRetryable(err) != tc.retryable || errors.Is(err, app.ErrOutsideWindow) != tc.window {
			t.Errorf("%d %s: reintentable=%v ventana=%v", tc.status, tc.body, app.IsRetryable(err), errors.Is(err, app.ErrOutsideWindow))
		}
		if strings.Contains(err.Error(), "573001112233") || strings.Contains(err.Error(), "Recipient") {
			t.Errorf("el error repite datos de Meta: %v", err)
		}
	}
}

func TestClient_NetworkFailureIsRetryable(t *testing.T) {
	c := meta.New("http://127.0.0.1:1", "t", 500*time.Millisecond) // nadie escucha en el puerto 1
	_, err := c.SendText(context.Background(), "123", "+573001112233", "hola")
	if err == nil || !app.IsRetryable(err) {
		t.Fatalf("red caída: %v", err)
	}
}
