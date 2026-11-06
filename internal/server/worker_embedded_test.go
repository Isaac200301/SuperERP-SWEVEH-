package server_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"sweveh/internal/platform/config"
	"sweveh/internal/wire"
)

// ADR-0036: el worker que arranca StartWorker (lo usa el api con RUN_WORKER=true) procesa solo, sin nadie que lo
// bombee, un mensaje que entra por WhatsApp: el cliente recibe la pregunta de autorización. Al cancelar el contexto,
// se detiene.
func TestStartWorker_ProcessesInboundAlone(t *testing.T) {
	e := newEnv(t)
	owner := e.register("3105550920", "Rosa Pérez")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		wire.StartWorker(ctx, e.db.Worker, e.st, e.clk, e.log, wire.WorkerConfig{WhatsApp: config.WhatsApp{Provider: "local"},
			AI: config.AI{Provider: "reglas"}, Poll: 50 * time.Millisecond, Batch: 50})
	}()

	owner.must(http.StatusAccepted, owner.do("POST", "/api/v1/simulator/inbound", map[string]any{"celular": "3001234920", "nombre": "Laura", "texto": "Hola"}))
	deadline := time.Now().Add(15 * time.Second)
	for {
		var replied bool
		for _, c := range owner.must(http.StatusOK, owner.do("GET", "/api/v1/conversations", nil)).body["conversaciones"].([]any) {
			id := c.(map[string]any)["id"].(string)
			for _, m := range owner.must(http.StatusOK, owner.do("GET", "/api/v1/conversations/"+id, nil)).body["mensajes"].([]any) {
				if m.(map[string]any)["direccion"] == "saliente" {
					replied = true
				}
			}
		}
		if replied {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("el worker no procesó el mensaje entrante")
		}
		time.Sleep(200 * time.Millisecond)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("el worker no se detuvo al cancelar")
	}
}
