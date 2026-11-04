package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// La SPA: los archivos con hash se guardan un año; los de nombre fijo (sw.js, manifest) y las rutas del
// cliente se revalidan, para que la PWA del piso tome la versión nueva (HU-23). /api nunca cae en la SPA.
func TestSPA_CacheHeadersAndFallback(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"index.html": "<html>app</html>", "sw.js": "// sw", "manifest.webmanifest": "{}", "assets/app-abc123.js": "// js",
	} {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	h := spa(dir)
	for _, tc := range []struct {
		path, cache string
		status      int
	}{
		{"/assets/app-abc123.js", "public, max-age=31536000, immutable", http.StatusOK},
		{"/sw.js", "no-cache", http.StatusOK},
		{"/manifest.webmanifest", "no-cache", http.StatusOK},
		{"/panel/piso", "no-cache", http.StatusOK},
		{"/api/v1/nada", "", http.StatusNotFound},
	} {
		rec := httptest.NewRecorder()
		h(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if rec.Code != tc.status || rec.Header().Get("Cache-Control") != tc.cache {
			t.Errorf("%s: %d %q", tc.path, rec.Code, rec.Header().Get("Cache-Control"))
		}
	}
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/manifest.webmanifest", nil))
	if ct := rec.Header().Get("Content-Type"); ct != "application/manifest+json" {
		t.Errorf("tipo del manifest: %q", ct)
	}
}
