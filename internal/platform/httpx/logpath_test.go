package httpx

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLogPath(t *testing.T) {
	for in, want := range map[string]string{
		"/api/v1/customers/by-phone/3001234567":               "/api/v1/customers/by-phone/********67",
		"/api/v1/orders/01a21aab-67ea-789e-b474-a0b3cfc6949a": "/api/v1/orders/01a21aab-67ea-789e-b474-a0b3cfc6949a",
		"/api/v1/search/ABC123":                               "/api/v1/search/ABC123",
		"/api/v1/customers/by-phone/%2B573001234567":          "/api/v1/customers/by-phone/%2B**********67",
	} {
		if got := LogPath(in); got != want {
			t.Errorf("LogPath(%q) = %q, quería %q", in, got, want)
		}
	}
}

// Un error interno en una ruta con el celular tampoco lo deja completo en el log.
func TestWriteErrorLogsMaskedPath(t *testing.T) {
	var buf bytes.Buffer
	r := httptest.NewRequest(http.MethodGet, "/api/v1/customers/by-phone/3001234567", nil)
	WriteError(httptest.NewRecorder(), r, slog.New(slog.NewTextHandler(&buf, nil)), errors.New("se cayó la base"))
	if strings.Contains(buf.String(), "3001234567") || !strings.Contains(buf.String(), "********67") {
		t.Fatalf("log: %s", buf.String())
	}
}
