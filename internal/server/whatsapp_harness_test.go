package server_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"sweveh/internal/assistant/adapters/out/llm/rules"
	assistantapp "sweveh/internal/assistant/app"
	messagingapp "sweveh/internal/messaging/app"
	"sweveh/internal/outbox"
	"sweveh/internal/platform/config"
	"sweveh/internal/platform/db"
	"sweveh/internal/server"
	"sweveh/internal/wire"
)

// wa es el circuito de WhatsApp completo con el canal de prueba: el cliente escribe por el simulador
// (POST /simulator/inbound, mismo formato del webhook de Meta), el "worker" interpreta los webhooks y despacha
// el outbox con los mismos consumidores de cmd/worker (wire.Register), y se mira qué le respondimos.
type wa struct {
	t     *testing.T
	e     *env
	owner *client
	s     *wire.Services
	d     *outbox.Dispatcher
}

// whatsapp arma el lado del worker sobre el rol sweveh_worker. ch nil = canal local (simulador).
func (e *env) whatsapp(t *testing.T, owner *client, ch messagingapp.Channel) *wa {
	t.Helper()
	return e.whatsappWith(t, owner, ch, nil)
}

// whatsappWith es whatsapp con un intérprete de IA de prueba (nil = reglas).
func (e *env) whatsappWith(t *testing.T, owner *client, ch messagingapp.Channel, it assistantapp.Interpreter) *wa {
	t.Helper()
	if it == nil {
		it = rules.Interpreter{}
	}
	m := server.Build(db.NewTxManager(e.db.Worker), e.st, e.clk, e.log)
	if ch == nil {
		ch = wire.Channel(config.WhatsApp{Provider: "local"})
	}
	s := wire.BuildWith(m, ch, it, config.AI{Provider: "reglas"})
	d := outbox.NewDispatcher(e.db.Worker, e.clk, e.log, 100)
	wire.Register(d, s)
	w := &wa{t: t, e: e, owner: owner, s: s, d: d}
	// Las pruebas miran el borrador en "Por revisar": apagan el envío automático (ADR-0028). TestAgent_AutoReply
	// lo prueba encendido.
	w.autoReply(false)
	return w
}

func (w *wa) autoReply(on bool) {
	w.t.Helper()
	w.owner.must(http.StatusOK, w.owner.do("PUT", "/api/v1/assistant/settings", map[string]any{"autoEnvio": on}))
}

// pump procesa webhooks y eventos hasta que no quede nada pendiente (o falla si no se estabiliza).
func (w *wa) pump() {
	w.t.Helper()
	ctx := context.Background()
	for range 30 {
		n1, err := w.s.Messaging.ProcessWebhooks(ctx)
		if err != nil {
			w.t.Fatalf("webhooks: %v", err)
		}
		n2, err := w.d.RunOnce(ctx)
		if err != nil {
			w.t.Fatalf("outbox: %v", err)
		}
		if n1+n2 == 0 {
			return
		}
	}
	w.t.Fatal("el circuito no se estabilizó en 30 vueltas")
}

// write: el cliente escribe un texto. press: pulsa un botón (id) cuyo título es title. Cada uno avanza el reloj
// un minuto para que los mensajes queden en orden, como en la vida real.
func (w *wa) write(phone, name, text string) {
	w.t.Helper()
	w.e.clk.Advance(time.Minute)
	w.owner.must(http.StatusAccepted, w.owner.do("POST", "/api/v1/simulator/inbound", map[string]any{"celular": phone, "nombre": name, "texto": text}))
	w.pump()
}

func (w *wa) press(phone, name, button, title string) {
	w.t.Helper()
	w.e.clk.Advance(time.Minute)
	w.owner.must(http.StatusAccepted, w.owner.do("POST", "/api/v1/simulator/inbound", map[string]any{"celular": phone, "nombre": name, "texto": title, "boton": button}))
	w.pump()
}

// conversation devuelve la conversación del celular (o nil) y sus mensajes.
func (w *wa) conversation(phone string) (map[string]any, []map[string]any) {
	w.t.Helper()
	for _, c := range w.owner.must(http.StatusOK, w.owner.do("GET", "/api/v1/conversations", nil)).body["conversaciones"].([]any) {
		conv := c.(map[string]any)
		if conv["celular"] != phone {
			continue
		}
		r := w.owner.must(http.StatusOK, w.owner.do("GET", "/api/v1/conversations/"+conv["id"].(string), nil))
		var out []map[string]any
		for _, m := range r.body["mensajes"].([]any) {
			out = append(out, m.(map[string]any))
		}
		return r.body["conversacion"].(map[string]any), out
	}
	return nil, nil
}

// sent son los mensajes que le mandamos al celular, en orden.
func (w *wa) sent(phone string) []map[string]any {
	w.t.Helper()
	_, msgs := w.conversation(phone)
	var out []map[string]any
	for _, m := range msgs {
		if m["direccion"] == "saliente" {
			out = append(out, m)
		}
	}
	return out
}

// last es el último mensaje que le mandamos; falla si no hay ninguno.
func (w *wa) last(phone string) map[string]any {
	w.t.Helper()
	s := w.sent(phone)
	if len(s) == 0 {
		w.t.Fatalf("no se le ha enviado nada a %s", phone)
	}
	return s[len(s)-1]
}

// buttons son los ids de los botones de un mensaje saliente.
func buttons(m map[string]any) []string {
	p, _ := m["payload"].(map[string]any)
	var ids []string
	if bs, ok := p["botones"].([]any); ok {
		for _, b := range bs {
			ids = append(ids, b.(map[string]any)["id"].(string))
		}
	}
	return ids
}

func hasPrefix(ids []string, prefix string) bool {
	for _, id := range ids {
		if strings.HasPrefix(id, prefix) {
			return true
		}
	}
	return false
}

// runs son las ejecuciones de automatizaciones del taller (código → resultados).
func (w *wa) runs() []map[string]any {
	w.t.Helper()
	var out []map[string]any
	for _, r := range w.owner.must(http.StatusOK, w.owner.do("GET", "/api/v1/automations/runs", nil)).body["ejecuciones"].([]any) {
		out = append(out, r.(map[string]any))
	}
	return out
}

// drafts son los borradores pendientes en "Por revisar" (HU-62).
func (w *wa) drafts() []map[string]any {
	w.t.Helper()
	var out []map[string]any
	for _, d := range w.owner.must(http.StatusOK, w.owner.do("GET", "/api/v1/inbox", nil)).body["borradores"].([]any) {
		out = append(out, d.(map[string]any))
	}
	return out
}

// customer: el cliente escribe por primera vez y acepta la autorización; devuelve su id.
func (w *wa) customer(phone, name string) string {
	w.t.Helper()
	w.write(phone, name, "Hola")
	w.press(phone, name, "consent:si", "Acepto")
	return w.owner.must(http.StatusOK, w.owner.do("GET", "/api/v1/customers/by-phone/"+phone, nil)).str("id")
}

// orderFor abre una orden "Por recibir" para el cliente, con la ficha completa y un repuesto con existencia,
// en un taller que no exige fotos ni kilometraje para recibir (parámetros, para no subir fotos en cada prueba).
func (w *wa) orderFor(customerID, plate string) (orderID, partID string) {
	w.t.Helper()
	c := w.owner
	s := c.must(http.StatusOK, c.do("GET", "/api/v1/workshop/settings", nil)).body
	s["fotosMinimas"], s["kmObligatorio"] = 0, false
	c.must(http.StatusOK, c.do("PUT", "/api/v1/workshop/settings", s))
	partID = c.must(http.StatusCreated, c.do("POST", "/api/v1/parts", map[string]any{
		"referencia": "FA-" + plate, "nombre": "Filtro de aceite", "tipoVehiculo": "carro", "costo": 10000, "precioVenta": 25000, "existencia": 5, "minimo": 0,
	})).str("id")
	c.must(http.StatusCreated, c.do("POST", "/api/v1/vehicles", map[string]any{"placa": plate, "clienteId": customerID, "marca": "Renault", "linea": "Logan",
		"cilindraje": "1600", "energia": "gasolina", "aspiracion": "atmosferico", "transmision": "manual"}))
	orderID = c.must(http.StatusCreated, c.do("POST", "/api/v1/orders", map[string]any{"placa": plate, "sintoma": "Cambio de aceite"})).str("id")
	c.must(http.StatusOK, c.do("PUT", "/api/v1/orders/"+orderID+"/checklist", map[string]any{"combustible": "1/2"}))
	return orderID, partID
}

// move pasa la orden al estado indicado y deja que el worker procese el evento.
func (w *wa) move(orderID, to string, extra map[string]any) {
	w.t.Helper()
	w.e.clk.Advance(time.Minute)
	body := map[string]any{"estado": to, "version": r2v(w.owner, orderID)}
	for k, v := range extra {
		body[k] = v
	}
	w.owner.must(http.StatusOK, w.owner.do("POST", "/api/v1/orders/"+orderID+"/transition", body))
	w.pump()
}

// sentBy son los mensajes que salieron por una automatización.
func (w *wa) sentBy(phone, code string) []map[string]any {
	var out []map[string]any
	for _, m := range w.sent(phone) {
		if m["automatizacion"] == code {
			out = append(out, m)
		}
	}
	return out
}

// runsOf son las ejecuciones de una automatización.
func (w *wa) runsOf(code string) []map[string]any {
	var out []map[string]any
	for _, r := range w.runs() {
		if r["codigo"] == code {
			out = append(out, r)
		}
	}
	return out
}
