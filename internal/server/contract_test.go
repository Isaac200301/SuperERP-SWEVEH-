package server_test

// El contrato (api/openapi.yaml) es la fuente de verdad: cada ruta del router está en el
// contrato y cada operación del contrato existe en el router. Sin dependencias extra:
// se leen las rutas del YAML por su indentación, que es estable en este archivo.

import (
	"bufio"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func contractRoutes(t *testing.T) map[string]bool {
	t.Helper()
	f, err := os.Open("../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	out := map[string]bool{}
	inPaths, current := false, ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "paths:":
			inPaths = true
		case inPaths && len(line) > 0 && line[0] != ' ':
			inPaths = false
		case inPaths && strings.HasPrefix(line, "  /") && strings.HasSuffix(line, ":"):
			current = strings.TrimSuffix(strings.TrimSpace(line), ":")
		case inPaths && current != "" && strings.HasPrefix(line, "    ") && !strings.HasPrefix(line, "     "):
			m := strings.TrimSuffix(strings.TrimSpace(line), ":")
			switch m {
			case "get", "post", "put", "patch", "delete":
				out[strings.ToUpper(m)+" /api/v1"+current] = true
			}
		}
	}
	return out
}

func TestRouterMatchesOpenAPIContract(t *testing.T) {
	e := newEnv(t)
	want := contractRoutes(t)
	if len(want) < 10 {
		t.Fatalf("solo se leyeron %d operaciones del contrato", len(want))
	}
	got := map[string]bool{}
	err := chi.Walk(e.mux, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if strings.HasPrefix(route, "/api/v1/") {
			got[method+" "+strings.TrimSuffix(route, "/")] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var missing, extra []string
	for k := range want {
		if !got[k] {
			missing = append(missing, k)
		}
	}
	for k := range got {
		if !want[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) > 0 || len(extra) > 0 {
		t.Fatalf("contrato y router no coinciden.\nEn el contrato pero no en el router: %v\nEn el router pero no en el contrato: %v", missing, extra)
	}
}
