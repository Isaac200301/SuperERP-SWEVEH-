package server_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"sweveh/internal/platform/config"
)

// fakeVision imita a OpenRouter leyendo la foto de una lista de precios con 7 servicios (más una fila vacía, que se
// descarta). fail=true responde 429 (límite del plan gratuito).
func fakeVision(t *testing.T, fail *atomic.Bool) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		if fail.Load() {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		content := req["messages"].([]any)[0].(map[string]any)["content"].([]any)
		img := content[1].(map[string]any)["image_url"].(map[string]any)["url"].(string)
		if req["model"] != "vision-1" || !strings.HasPrefix(img, "data:image/jpeg;base64,") {
			t.Errorf("petición de visión: modelo %v, imagen %.40s", req["model"], img)
		}
		rows := `{"servicios":[
			{"nombre":"Cambio de aceite","precio":45000,"tipo_vehiculo":"ambos"},
			{"nombre":"  Revisión   de frenos ","precio":40000,"tipo_vehiculo":"ambos"},
			{"nombre":"Sincronización","precio":180000,"tipo_vehiculo":"carro"},
			{"nombre":"Mantenimiento moto","precio":120000,"tipo_vehiculo":"moto"},
			{"nombre":"Alineación y balanceo","precio":80000,"tipo_vehiculo":"carro"},
			{"nombre":"Diagnóstico con escáner","precio":70000,"tipo_vehiculo":"bus"},
			{"nombre":"Cambio de amortiguadores","precio":null,"tipo_vehiculo":"carro"},
			{"nombre":"","precio":5000,"tipo_vehiculo":"carro"}]}`
		raw, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": rows}}}})
		_, _ = w.Write(raw)
	}))
}

// HU-93 (ADR-0032): la foto de una lista de 7 servicios deja 7 filas por revisar con la foto al lado; el dueño corrige
// y aprueba cada una (solo entonces existe el servicio) o la descarta. Sin IA, o si la IA falla, lo dice sin crear nada.
func TestServices_PriceListFromPhoto(t *testing.T) {
	var fail atomic.Bool
	ai := fakeVision(t, &fail)
	t.Cleanup(ai.Close)
	cfg := config.API{Env: config.EnvLocal, AI: config.AI{Provider: "openrouter", APIKey: "k", Model: "texto-1", APIBase: ai.URL,
		VisionModel: "vision-1"}}
	e := newEnvWith(t, cfg)
	c := e.register("3105550660", "Rosa Pérez")

	r := c.must(http.StatusCreated, c.upload("/api/v1/services/photo", nil, jpg))
	rows := r.body["propuestas"].([]any)
	if len(rows) != 7 {
		t.Fatalf("filas: %d %v", len(rows), rows)
	}
	byName := map[string]map[string]any{}
	for _, x := range rows {
		p := x.(map[string]any)
		byName[p["nombre"].(string)] = p
	}
	if byName["Revisión de frenos"] == nil || byName["Diagnóstico con escáner"]["tipoVehiculo"] != "ambos" ||
		byName["Cambio de amortiguadores"]["precio"] != nil || byName["Mantenimiento moto"]["tipoVehiculo"] != "moto" {
		t.Fatalf("filas limpias: %v", byName)
	}
	if n := len(c.must(http.StatusOK, c.do("GET", "/api/v1/services/proposals", nil)).body["propuestas"].([]any)); n != 7 {
		t.Fatalf("por revisar: %d", n)
	}
	if n := len(c.must(http.StatusOK, c.do("GET", "/api/v1/services", nil)).body["servicios"].([]any)); n != 0 {
		t.Fatalf("la IA creó servicios sin aprobación: %d", n)
	}

	// La foto se ve al lado de cada fila.
	sync := byName["Sincronización"]["id"].(string)
	req, _ := http.NewRequest("GET", e.srv.URL+"/api/v1/services/proposals/"+sync+"/photo", nil)
	res, err := c.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "image/jpeg" || len(body) != len(jpg) {
		t.Fatalf("foto: %d %s", res.StatusCode, res.Header.Get("Content-Type"))
	}

	// Corrige el precio y aprueba: queda el servicio con el precio del dueño.
	ok := c.must(http.StatusOK, c.do("POST", "/api/v1/services/proposals/"+sync, map[string]any{"decision": "aprobar", "precio": 190000})).body
	if ok["estado"] != "aprobada" || ok["servicioId"] == nil || ok["precio"] != float64(190000) {
		t.Fatalf("aprobar: %v", ok)
	}
	svcs := c.must(http.StatusOK, c.do("GET", "/api/v1/services", nil)).body["servicios"].([]any)
	if len(svcs) != 1 || svcs[0].(map[string]any)["nombre"] != "Sincronización" || svcs[0].(map[string]any)["precio"] != float64(190000) {
		t.Fatalf("servicio creado: %v", svcs)
	}
	if r := c.do("POST", "/api/v1/services/proposals/"+sync, map[string]any{"decision": "descartar"}); r.status != http.StatusConflict {
		t.Fatalf("resolver dos veces: %d", r.status)
	}
	c.must(http.StatusOK, c.do("POST", "/api/v1/services/proposals/"+byName["Cambio de aceite"]["id"].(string), map[string]any{"decision": "descartar"}))
	if n := len(c.must(http.StatusOK, c.do("GET", "/api/v1/services/proposals", nil)).body["propuestas"].([]any)); n != 5 {
		t.Fatalf("quedan por revisar: %d", n)
	}

	// Si la IA falla, no se crea nada y se dice qué pasó.
	fail.Store(true)
	if r := c.upload("/api/v1/services/photo", nil, jpg); r.status != http.StatusUnprocessableEntity || r.str("code") != "lista_no_leida" {
		t.Fatalf("IA caída: %d %v", r.status, r.body)
	}
	if r := c.upload("/api/v1/services/photo", nil, []byte("no es una foto")); r.status != http.StatusBadRequest {
		t.Fatalf("archivo que no es imagen: %d", r.status)
	}

	// Sin IA configurada (reglas), lo dice.
	plain := newEnv(t)
	o := plain.register("3105550661", "Iván Soto")
	if r := o.upload("/api/v1/services/photo", nil, jpg); r.status != http.StatusUnprocessableEntity || r.str("code") != "lista_sin_ia" {
		t.Fatalf("sin IA: %d %v", r.status, r.body)
	}
}
