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

	"sweveh/internal/platform/config"
)

// contractRoutes devuelve cada operación del contrato con su bandera (x-feature del path: "whatsapp", "platform" o
// "" si siempre está). on dice qué banderas están encendidas; las operaciones de una bandera apagada no se cuentan.
func contractRoutes(t *testing.T, on map[string]bool) map[string]bool {
	t.Helper()
	all := contractOperations(t)
	out := map[string]bool{}
	for op, feature := range all {
		if feature == "" || on[feature] {
			out[op] = true
		}
	}
	return out
}

func contractOperations(t *testing.T) map[string]string {
	t.Helper()
	f, err := os.Open("../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	out := map[string]string{}
	features := map[string]string{} // path → x-feature
	var ops []string
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
		case inPaths && current != "" && strings.HasPrefix(line, "    x-feature: "):
			features[current] = strings.TrimSpace(strings.TrimPrefix(line, "    x-feature: "))
		case inPaths && current != "" && strings.HasPrefix(line, "    ") && !strings.HasPrefix(line, "     "):
			m := strings.TrimSuffix(strings.TrimSpace(line), ":")
			switch m {
			case "get", "post", "put", "patch", "delete":
				ops = append(ops, strings.ToUpper(m)+" "+current)
			}
		}
	}
	for _, op := range ops {
		method, path, _ := strings.Cut(op, " ")
		out[method+" /api/v1"+path] = features[path]
	}
	return out
}

func TestRouterMatchesOpenAPIContract(t *testing.T) {
	routerMatches(t, newEnv(t), map[string]bool{"whatsapp": true, "platform": true})
}

// Entrega del 27-nov (V-01, V-02): con FEATURE_WHATSAPP y FEATURE_PLATFORM apagadas, el router tiene exactamente las
// operaciones del contrato sin x-feature; lo de WhatsApp y Plataforma no existe.
func TestRouterHidesFeaturesThatAreOff(t *testing.T) {
	routerMatches(t, newEnvRaw(t, config.API{Env: config.EnvLocal}), map[string]bool{})
}

func routerMatches(t *testing.T, e *env, on map[string]bool) {
	t.Helper()
	want := contractRoutes(t, on)
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
