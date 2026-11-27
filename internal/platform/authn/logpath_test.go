package authn

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"sweveh/internal/platform/reqctx"
)

// R11 (R-03 punto 6, docs/06 §10): el 403 de una ruta con el celular no deja el número completo en el log.
func TestRequireLogsMaskedPath(t *testing.T) {
	var buf bytes.Buffer
	g := NewGuard(nil, func([]string, string) bool { return false }, slog.New(slog.NewTextHandler(&buf, nil)))
	h := g.Require("customer:create")(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	r := httptest.NewRequest(http.MethodGet, "/api/v1/customers/by-phone/3001234567", nil)
	r = r.WithContext(reqctx.WithPrincipal(r.Context(), reqctx.Principal{Roles: []string{"mecanico"}}))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status %d", w.Code)
	}
	if strings.Contains(buf.String(), "3001234567") || !strings.Contains(buf.String(), "********67") {
		t.Fatalf("log: %s", buf.String())
	}
}
