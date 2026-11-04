package server_test

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"

	assistantapp "sweveh/internal/assistant/app"
)

// fakeAdvisor es un modelo de prueba: step decide qué responder en cada ronda según lo que ya pasó (las tools que
// respondieron llegan como mensajes "tool"). Guarda cada petición para revisar el prompt, el historial y las tools.
type fakeAdvisor struct {
	mu   sync.Mutex
	reqs []assistantapp.ChatRequest
	step func(req assistantapp.ChatRequest) (assistantapp.ChatMessage, error)
}

func (f *fakeAdvisor) Chat(_ context.Context, r assistantapp.ChatRequest) (assistantapp.ChatResult, error) {
	f.mu.Lock()
	f.reqs = append(f.reqs, r)
	f.mu.Unlock()
	step := f.step
	if step == nil {
		step = func(assistantapp.ChatRequest) (assistantapp.ChatMessage, error) {
			return say("¿En qué te ayudo?"), nil
		}
	}
	m, err := step(r)
	return assistantapp.ChatResult{Message: m, Provider: "falsa", Model: "asesor", TokensIn: 300, TokensOut: 60}, err
}

func call(name, args string) assistantapp.ChatMessage {
	return assistantapp.ChatMessage{Role: "assistant", ToolCalls: []assistantapp.ToolCall{{ID: "c-" + name, Name: name, Args: args}}}
}

func say(text string) assistantapp.ChatMessage {
	return assistantapp.ChatMessage{Role: "assistant", Content: text}
}

// toolResult son las respuestas de las tools desde el último mensaje del cliente.
func toolResult(r assistantapp.ChatRequest) string {
	var out []string
	for i := len(r.Messages) - 1; i >= 0 && r.Messages[i].Role != "user"; i-- {
		if r.Messages[i].Role == "tool" {
			out = append(out, r.Messages[i].Content)
		}
	}
	return strings.Join(out, " | ")
}

func lastUser(r assistantapp.ChatRequest) string {
	for i := len(r.Messages) - 1; i >= 0; i-- {
		if r.Messages[i].Role == "user" {
			return r.Messages[i].Content
		}
	}
	return ""
}

func (e *env) advisorWA(t *testing.T, owner *client, adv assistantapp.Advisor) *wa {
	t.Helper()
	w := e.whatsapp(t, owner, nil)
	w.s.Assistant.SetAdvisor(adv)
	w.autoReply(true)
	return w
}

// ADR-0029: el asesor saluda como una persona del taller, ofrece servicios y la vitrina con precios que trajo de las
// tools (nunca el costo), recuerda la conversación, agenda sin duplicar y manda la foto del producto.
func TestAdvisor_GreetsOffersAndSchedules(t *testing.T) {
	e := newEnv(t)
	owner := e.register("3105550700", "Rosa Pérez")
	adv := &fakeAdvisor{}
	w := e.advisorWA(t, owner, adv)
	w.service("Cambio de aceite", "ambos", 45000)
	p := owner.must(http.StatusCreated, owner.do("POST", "/api/v1/parts", map[string]any{"referencia": "AC-20W50", "nombre": "Aceite 20W50 moto",
		"tipoVehiculo": "moto", "costo": 18000, "precioVenta": 32000, "existencia": 6, "minimo": 1}))
	owner.must(http.StatusOK, owner.do("PATCH", "/api/v1/parts/"+p.str("id"), map[string]any{"ofrecerWhatsapp": true, "version": p.body["version"]}))
	if s := owner.putPhoto(p.str("id"), jpg); s != http.StatusOK {
		t.Fatalf("foto: %d", s)
	}
	const laura = "3001234700"
	w.customer(laura, "Laura Gómez")

	adv.step = func(r assistantapp.ChatRequest) (assistantapp.ChatMessage, error) {
		switch res := toolResult(r); {
		case strings.Contains(lastUser(r), "hola"):
			if !strings.Contains(res, "servicios") {
				return call("servicios", `{"tipo_vehiculo":""}`), nil
			}
			if !strings.Contains(res, "productos") {
				return call("productos", `{"busqueda":""}`), nil
			}
			return say("¡Hola Laura! Soy el asesor del taller. Te puedo hacer el cambio de aceite por $45.000 y tenemos aceite 20W50 para moto a $32.000. ¿En qué te ayudo?"), nil
		case strings.Contains(lastUser(r), "foto"):
			if res == "" || !strings.Contains(res, "enviada") {
				return call("enviar_foto", `{"producto":"AC-20W50"}`), nil
			}
			return say("Te mando la foto del aceite 20W50. Cuesta $32.000."), nil
		case strings.Contains(lastUser(r), "GNO667"):
			if !strings.Contains(res, "resultado") {
				return call("agendar", `{"placa":"GNO667","sintoma":"cambio de aceite"}`), nil
			}
			if strings.Contains(res, "ya_tenia_orden") {
				return say("Ya tienes tu moto agendada, no te preocupes."), nil
			}
			return say("Listo, te esperamos con la moto GNO667. Tu orden es la " + regexp.MustCompile(`OS-\d+`).FindString(res) + "."), nil
		}
		return say("¿En qué te ayudo?"), nil
	}

	w.write(laura, "Laura Gómez", "hola")
	got := w.last(laura)["cuerpo"].(string)
	if !strings.Contains(got, "Soy el asesor") || !strings.Contains(got, "$32.000") {
		t.Fatalf("saludo: %q", got)
	}
	sys := adv.reqs[0].System
	if !strings.Contains(sys, "asesor de servicio") || !strings.Contains(sys, "Laura") || len(adv.reqs[0].Tools) == 0 {
		t.Fatalf("prompt sin rol, cliente o tools: %.200s", sys)
	}
	for _, r := range adv.reqs {
		if res := toolResult(r); strings.Contains(res, "18000") || strings.Contains(res, "costo") {
			t.Fatalf("una tool mostró el costo: %s", res)
		}
	}

	// Recuerda la conversación: el saludo y lo que respondió van en el historial del siguiente mensaje.
	n := len(adv.reqs)
	w.write(laura, "Laura Gómez", "¿me mandas la foto del aceite?")
	if hist := adv.reqs[n].Messages; len(hist) < 3 || !strings.Contains(hist[len(hist)-2].Content, "Soy el asesor") {
		t.Fatalf("sin historial: %+v", hist)
	}
	var img map[string]any
	for _, m := range w.sent(laura) {
		if m["tipo"] == "imagen" {
			img = m
		}
	}
	if img == nil || img["cuerpo"] != "Aceite 20W50 moto · $32.000" || img["estado"] != "enviado" {
		t.Fatalf("no mandó la foto: %v", img)
	}

	w.write(laura, "Laura Gómez", "te la llevo mañana, placa GNO667")
	open := func() int {
		return len(owner.must(http.StatusOK, owner.do("GET", "/api/v1/orders?abiertas=1", nil)).body["ordenes"].([]any))
	}
	o := open()
	if o != 1 || !strings.Contains(w.last(laura)["cuerpo"].(string), "GNO667") || !strings.Contains(w.last(laura)["cuerpo"].(string), "OS-") {
		t.Fatalf("agendar: abiertas %d, %q", o, w.last(laura)["cuerpo"])
	}
	w.write(laura, "Laura Gómez", "GNO667 otra vez")
	if open() != o || !strings.Contains(w.last(laura)["cuerpo"].(string), "Ya tienes") {
		t.Fatalf("duplicó la orden: abiertas %d, %q", open(), w.last(laura)["cuerpo"])
	}
	if len(w.drafts()) != 0 {
		t.Fatalf("quedó algo por revisar: %v", w.drafts())
	}
}

// Un precio que no salió de una tool no llega al cliente (tiene una ronda para corregir); si la IA falla, responden las
// reglas sin volver a llamarla;
// y si el asesor pasa la conversación a una persona, se detiene.
func TestAdvisor_GuardsAndFallbacks(t *testing.T) {
	e := newEnv(t)
	owner := e.register("3105550701", "Rosa Pérez")
	adv := &fakeAdvisor{}
	w := e.advisorWA(t, owner, adv)
	w.service("Cambio de aceite", "ambos", 45000)
	const laura = "3001234701"
	w.customer(laura, "Laura Gómez")

	adv.step = func(assistantapp.ChatRequest) (assistantapp.ChatMessage, error) {
		return say("El cambio de aceite te sale en $39.000."), nil // inventado: no consultó servicios
	}
	// Insiste en el precio inventado aun después de la ronda de corrección: no sale; responde el flujo de reglas
	// con el precio de la lista.
	n0 := len(adv.reqs)
	w.write(laura, "Laura Gómez", "¿cuánto el cambio de aceite?")
	if got := w.last(laura)["cuerpo"].(string); strings.Contains(got, "39.000") || !strings.Contains(got, "$45.000") || len(adv.reqs) != n0+2 {
		t.Fatalf("precio inventado: %q, llamadas %d", got, len(adv.reqs)-n0)
	}

	adv.step = func(assistantapp.ChatRequest) (assistantapp.ChatMessage, error) {
		return assistantapp.ChatMessage{}, errors.New("openrouter: respuesta 429 (código 429)")
	}
	n := len(adv.reqs)
	w.write(laura, "Laura Gómez", "¿cuánto vale el cambio de aceite?")
	if got := w.last(laura)["cuerpo"].(string); !strings.Contains(got, "$45.000") || len(adv.reqs) != n+1 {
		t.Fatalf("respaldo de reglas: %q, llamadas al asesor %d", got, len(adv.reqs)-n)
	}

	adv.step = func(r assistantapp.ChatRequest) (assistantapp.ChatMessage, error) {
		if toolResult(r) == "" {
			return call("pasar_a_persona", `{"motivo":"está molesto"}`), nil
		}
		return say("Entiendo, ya le aviso a una persona del taller para que te escriba."), nil
	}
	w.write(laura, "Laura Gómez", "esto es una burla, nadie me responde bien")
	conv, _ := w.conversation(laura)
	if conv["modo"] != "humano" || conv["pideHumano"] != true {
		t.Fatalf("no pasó a una persona: %v", conv)
	}
	n = len(adv.reqs)
	w.write(laura, "Laura Gómez", "¿hola?")
	if len(adv.reqs) != n {
		t.Fatal("el asesor siguió con la conversación en manos de una persona")
	}
}

// Si el asesor le dice al cliente que una persona le escribe pero olvida la tool, la conversación pasa igual a una
// persona con alerta: lo que se le promete al cliente se cumple.
func TestAdvisor_PromiseOfAPersonIsKept(t *testing.T) {
	e := newEnv(t)
	owner := e.register("3105550702", "Rosa Pérez")
	adv := &fakeAdvisor{}
	w := e.advisorWA(t, owner, adv)
	const laura = "3001234702"
	w.customer(laura, "Laura Gómez")
	adv.step = func(assistantapp.ChatRequest) (assistantapp.ChatMessage, error) {
		return say("Te pido disculpas. Una persona del taller te va a escribir ya para darte el detalle."), nil
	}
	w.write(laura, "Laura Gómez", "llevo 3 días esperando, qué falta de seriedad")
	if conv, _ := w.conversation(laura); conv["modo"] != "humano" || conv["pideHumano"] != true {
		t.Fatalf("prometió una persona y no pasó la conversación: %v", conv)
	}
}
