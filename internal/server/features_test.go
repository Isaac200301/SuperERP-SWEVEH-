package server_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"sweveh/internal/assistant/adapters/out/llm/rules"
	"sweveh/internal/outbox"
	"sweveh/internal/platform/config"
	"sweveh/internal/platform/db"
	"sweveh/internal/server"
	"sweveh/internal/wire"
)

// Entrega del 27-nov (R1, V-01, V-02, V-09): con FEATURE_WHATSAPP y FEATURE_PLATFORM apagadas no existe nada de
// WhatsApp ni de Plataforma, /me lo dice y el tablero queda con los KPI que no dependen de WhatsApp.
func TestFeatures_WhatsAppAndPlatformOff(t *testing.T) {
	e := newEnvRaw(t, config.API{Env: config.EnvLocal})
	c := e.register("3105550960", "Rosa Pérez")

	me := c.must(http.StatusOK, c.do("GET", "/api/v1/me", nil))
	if f := nested(me.body, "funciones").(map[string]any); f["whatsapp"] != false || f["plataforma"] != false {
		t.Fatalf("funciones: %v", f)
	}
	for _, path := range []string{"/api/v1/conversations", "/api/v1/inbox", "/api/v1/automations", "/api/v1/whatsapp/channel",
		"/api/v1/assistant/settings", "/api/v1/platform/usage", "/api/v1/platform/messages"} {
		if r := c.do("GET", path, nil); r.status != http.StatusNotFound {
			t.Fatalf("GET %s con la bandera apagada: %d %v", path, r.status, r.body)
		}
	}
	// HU-93 (lista por foto) responde igual que cualquier /services/<algo> que no existe: chi tiene PATCH y DELETE
	// en /services/{id} y contesta 405 a un GET.
	none := c.do("GET", "/api/v1/services/no-existe", nil).status
	if r := c.do("GET", "/api/v1/services/proposals", nil); r.status != none {
		t.Fatalf("propuestas de la foto: %d, una ruta que no existe da %d", r.status, none)
	}
	if r := c.do("POST", "/api/v1/simulator/inbound", map[string]any{"celular": "3001234960", "texto": "Hola"}); r.status != http.StatusNotFound {
		t.Fatalf("simulador: %d", r.status)
	}
	// Los webhooks tampoco existen (sin cuerpo firmado: si existieran responderían 401, no 404).
	for _, path := range []string{"/webhooks/whatsapp", "/webhooks/waha"} {
		res, err := e.srv.Client().Post(e.srv.URL+path, "application/json", strings.NewReader("{}"))
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		if res.StatusCode != http.StatusNotFound {
			t.Fatalf("POST %s: %d", path, res.StatusCode)
		}
	}

	// V-09: salen "clientes que vuelven" (HU-72) y "respuesta en WhatsApp" (HU-73). Quedan 4; el quinto llega en R6.
	d := c.must(http.StatusOK, c.do("GET", "/api/v1/dashboard", nil))
	var codes []string
	for _, k := range d.body["kpis"].([]any) {
		codes = append(codes, k.(map[string]any)["codigo"].(string))
	}
	if strings.Join(codes, ",") != "abiertas,aprobaciones,tiempo,ticket" {
		t.Fatalf("KPI con WhatsApp apagado: %v", codes)
	}
}

// Con las banderas encendidas, /me lo dice y el tablero vuelve a tener los 6 KPI de antes.
func TestFeatures_OnKeepsEverything(t *testing.T) {
	e := newEnv(t)
	c := e.register("3105550961", "Rosa Pérez")
	me := c.must(http.StatusOK, c.do("GET", "/api/v1/me", nil))
	if f := nested(me.body, "funciones").(map[string]any); f["whatsapp"] != true || f["plataforma"] != true {
		t.Fatalf("funciones: %v", f)
	}
	d := c.must(http.StatusOK, c.do("GET", "/api/v1/dashboard", nil))
	if n := len(d.body["kpis"].([]any)); n != 6 {
		t.Fatalf("KPI con WhatsApp encendido: %d", n)
	}
	c.must(http.StatusOK, c.do("GET", "/api/v1/automations", nil))
}

// R1: con WhatsApp apagado, el aviso de un cambio de estado (AU-01…AU-07 cuelgan de orders.estado_cambiado) queda
// "omitido: whatsapp_apagado", procesado y sin reintentos: no se atasca en el outbox (nota del 2026-11-10, docs/09).
func TestFeatures_WorkerSkipsWhatsAppEvents(t *testing.T) {
	e := newEnvRaw(t, config.API{Env: config.EnvLocal})
	c := e.register("3105550962", "Rosa Pérez")
	r := c.must(http.StatusCreated, c.do("POST", "/api/v1/customers", map[string]any{"celular": "3005550962", "nombre": "Cliente OMI962"}))
	c.must(http.StatusCreated, c.do("POST", "/api/v1/vehicles", map[string]any{"placa": "OMI962", "clienteId": nested(r.body, "cliente", "id"),
		"cilindraje": "1600", "turbo": false, "energia": "gasolina", "transmision": "manual"}))
	r = c.must(http.StatusCreated, c.do("POST", "/api/v1/orders", map[string]any{"placa": "OMI962", "sintoma": "Vibra", "estadoInicial": "recibido"}))
	id := r.str("id")
	c.must(http.StatusOK, c.do("POST", "/api/v1/orders/"+id+"/transition", map[string]any{"estado": "en_diagnostico", "version": r.body["version"]}))

	m := server.Build(db.NewTxManager(e.db.Worker), e.st, e.clk, e.log)
	s := wire.BuildWith(m, wire.Channel(config.WhatsApp{Provider: "local"}), rules.Interpreter{}, config.AI{Provider: "reglas"})
	d := outbox.NewDispatcher(e.db.Worker, e.clk, e.log, 100)
	wire.Register(d, s, false)
	ctx := context.Background()
	if _, err := d.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}

	var pending, skipped int
	if err := e.db.Worker.QueryRow(ctx, `SELECT count(*) FILTER (WHERE procesado_en IS NULL),
		count(*) FILTER (WHERE procesado_en IS NOT NULL AND ultimo_error = 'omitido: whatsapp_apagado')
		FROM outbox.evento WHERE tipo = 'orders.estado_cambiado'`).Scan(&pending, &skipped); err != nil {
		t.Fatal(err)
	}
	if pending != 0 || skipped == 0 {
		t.Fatalf("estado_cambiado: %d pendientes, %d omitidos", pending, skipped)
	}
	// No se reintenta: la segunda vuelta no encuentra nada.
	if n, err := d.RunOnce(ctx); err != nil || n != 0 {
		t.Fatalf("segunda vuelta: %d %v", n, err)
	}
}
