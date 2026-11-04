package server_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	assistantapp "sweveh/internal/assistant/app"
	"sweveh/internal/platform/testdb"
)

// Pruebas del agente (T3) con el canal local y el intérprete por reglas, salvo donde se inyecta una IA falsa.
// El cliente escribe → el worker interpreta → el borrador espera al dueño en "Por revisar" (RF-18).

// draftFor es el borrador pendiente que respondió a un texto del cliente.
func (w *wa) draftFor(text string) map[string]any {
	w.t.Helper()
	for _, d := range w.drafts() {
		if strings.Contains(d["mensajeCliente"].(string), text) {
			return d
		}
	}
	w.t.Fatalf("no hay borrador para %q: %v", text, w.drafts())
	return nil
}

func (w *wa) resolve(draftID string, body map[string]any) map[string]any {
	w.t.Helper()
	r := w.owner.must(http.StatusOK, w.owner.do("POST", "/api/v1/inbox/"+draftID, body))
	w.pump()
	return r.body
}

func (w *wa) service(name, kind string, price any) {
	w.t.Helper()
	body := map[string]any{"nombre": name, "tipoVehiculo": kind}
	if price != nil {
		body["precio"] = price
	}
	w.owner.must(http.StatusCreated, w.owner.do("POST", "/api/v1/services", body))
}

func sources(d map[string]any) string {
	var out []string
	for _, f := range d["fuentes"].([]any) {
		m := f.(map[string]any)
		out = append(out, m["campo"].(string)+"="+m["valor"].(string)+" ("+m["origen"].(string)+")")
	}
	return strings.Join(out, "; ")
}

// aiCalls cuenta las llamadas registradas por resultado (HU-99) y cuántas quedaron ligadas a un mensaje.
func aiCalls(t *testing.T, e *env, c *client) (byResult map[string]int, linked int) {
	t.Helper()
	byResult = map[string]int{}
	err := e.db.Tx.Do(testdb.As(tallerOf(t, c)), func(ctx context.Context) error {
		var ok, inv, er, tope int
		if err := queryRow(ctx, `SELECT count(*) FILTER (WHERE resultado = 'ok'), count(*) FILTER (WHERE resultado = 'invalido'),
			count(*) FILTER (WHERE resultado = 'error'), count(*) FILTER (WHERE resultado = 'tope'), count(mensaje_id) FROM assistant.llamada_ia`,
			&ok, &inv, &er, &tope, &linked); err != nil {
			return err
		}
		byResult["ok"], byResult["invalido"], byResult["error"], byResult["tope"] = ok, inv, er, tope
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return byResult, linked
}

// HU-51, HU-53, HU-62, HU-63, HU-64, HU-65, HU-99 y HU-55: Laura pregunta en sus palabras; el borrador usa el precio
// de la lista con su fuente y no sale hasta que el dueño lo envía (editado); al llegar la placa y enviar, se crea
// la orden "Por recibir" con vehículo de motor pendiente y la conversación ligada.
func TestAgent_QuoteDraftThenPlateCreatesTheOrder(t *testing.T) {
	e := newEnv(t)
	owner := e.register("3105550500", "Rosa Pérez")
	w := e.whatsapp(t, owner, nil)
	w.service("Revisión de frenos", "carro", 60000)
	w.service("Cambio de amortiguadores", "carro", nil)
	w.service("Cambio de aceite", "moto", 25000)

	const laura = "3001234500"
	w.customer(laura, "Laura Gómez")
	before := len(w.sent(laura))
	w.write(laura, "Laura Gómez", "Buenas, mi Spark GT 2015 está chillando cuando freno, ¿cuánto la revisión? puedo llevarlo mañana")

	d := w.draftFor("chillando")
	text := d["propuesto"].(string)
	if !strings.Contains(text, "¡Hola Laura!") || !strings.Contains(text, "$60.000") || !strings.Contains(text, "placa") ||
		strings.Contains(text, "$25.000") || d["origen"] != "ia" || d["intencion"] != "cotizar" {
		t.Fatalf("borrador: %v", d)
	}
	if src := sources(d); !strings.Contains(src, "precio=$60.000 (Servicio: Revisión de frenos)") {
		t.Fatalf("fuentes (HU-64): %s", src)
	}
	if len(w.sent(laura)) != before {
		t.Fatal("el texto del agente salió sin que el dueño lo enviara (RF-18)")
	}

	// El dueño lo edita y lo envía (HU-63); cuenta como editado (HU-65).
	edited := "¡Hola Laura! La revisión de frenos cuesta $60.000. ¿Me regalas la placa?"
	if r := w.resolve(d["id"].(string), map[string]any{"decision": "enviar", "texto": edited}); r["estado"] != "editado_enviado" {
		t.Fatalf("resolver: %v", r)
	}
	if got := w.last(laura); got["cuerpo"] != edited || got["origen"] != "agente" {
		t.Fatalf("enviado: %v", got)
	}
	stats := owner.must(http.StatusOK, owner.do("GET", "/api/v1/inbox/stats", nil)).body
	if stats["editados"] != 1.0 || stats["enviados"] != 1.0 {
		t.Fatalf("estadísticas: %v", stats)
	}
	// Otro dueño no puede volver a resolverlo (HU-62).
	is4xx(t, "resuelto dos veces", owner.do("POST", "/api/v1/inbox/"+d["id"].(string), map[string]any{"decision": "descartar"}))

	// Llega la placa: el borrador trae la acción de crear la orden; al enviarlo se crea (HU-55).
	w.write(laura, "Laura Gómez", "es la KLM 482")
	p := w.draftFor("KLM 482")
	if p["intencion"] != "placa" || p["accion"] == nil {
		t.Fatalf("borrador de la placa: %v", p)
	}
	w.resolve(p["id"].(string), map[string]any{"decision": "enviar"})
	if body := w.last(laura)["cuerpo"].(string); !strings.Contains(body, "KLM482") || !strings.Contains(body, "Tu orden es la OS-") {
		t.Fatalf("confirmación: %q", body)
	}
	var estado, tipo, sintoma string
	err := e.db.Tx.Do(testdb.As(tallerOf(t, owner)), func(ctx context.Context) error {
		return queryRow(ctx, `SELECT estado, tipo_vehiculo, sintoma FROM orders.orden WHERE placa = 'KLM482'`, &estado, &tipo, &sintoma)
	})
	if err != nil || estado != "por_recibir" || tipo != "carro" || !strings.Contains(sintoma, "chillando") {
		t.Fatalf("orden: %q %q %q %v", estado, tipo, sintoma, err)
	}

	// Cada mensaje interpretado dejó su llamada registrada y ligada al mensaje (HU-99).
	calls, linked := aiCalls(t, e, owner)
	if calls["ok"] < 3 || linked != calls["ok"] {
		t.Fatalf("llamadas: %v, ligadas %d", calls, linked)
	}
}

// HU-55, criterio 2: la placa de moto crea el vehículo como moto.
func TestAgent_MotoPlateCreatesAMoto(t *testing.T) {
	e := newEnv(t)
	owner := e.register("3105550510", "Rosa Pérez")
	w := e.whatsapp(t, owner, nil)
	const andres = "3001234510"
	w.customer(andres, "Andrés Ruiz")
	w.write(andres, "Andrés Ruiz", "mi NMAX hace ruido en la cadena, la placa es ABC12D")
	w.resolve(w.draftFor("NMAX")["id"].(string), map[string]any{"decision": "enviar"})
	var tipo string
	err := e.db.Tx.Do(testdb.As(tallerOf(t, owner)), func(ctx context.Context) error {
		return queryRow(ctx, `SELECT tipo_vehiculo FROM orders.orden WHERE placa = 'ABC12D'`, &tipo)
	})
	if err != nil || tipo != "moto" {
		t.Fatalf("tipo %q, %v", tipo, err)
	}
}

// HU-56 y HU-57: "¿ya está?" con una orden abierta responde el estado real al instante (plantilla AU-05, sin
// borrador); con dos, pregunta cuál con botones de placa y responde la que escoja.
func TestAgent_StatusAndWhichVehicle(t *testing.T) {
	e := newEnv(t)
	owner := e.register("3105550520", "Rosa Pérez")
	w := e.whatsapp(t, owner, nil)
	const andres = "3001234520"
	cid := w.customer(andres, "Andrés Ruiz")
	first, _ := w.orderFor(cid, "KLM482")

	w.write(andres, "Andrés Ruiz", "ya está mi carro?")
	if m := w.last(andres); m["automatizacion"] != "AU-05" || !strings.Contains(m["cuerpo"].(string), "KLM482") {
		t.Fatalf("estado: %v", m)
	}
	for _, d := range w.drafts() {
		if strings.Contains(d["mensajeCliente"].(string), "ya está mi carro") {
			t.Fatal("el estado no debe pasar por borrador con AU-05 encendida")
		}
	}

	w.orderFor(cid, "XYZ789")
	w.write(andres, "Andrés Ruiz", "cómo va?")
	q := w.last(andres)
	ids := buttons(q)
	if !strings.Contains(q["cuerpo"].(string), "cuál vehículo") || len(ids) != 2 || !hasPrefix(ids, "estado:"+first) {
		t.Fatalf("¿cuál?: %v", q)
	}
	w.press(andres, "Andrés Ruiz", "estado:"+first, "KLM482")
	if m := w.last(andres); m["automatizacion"] != "AU-05" || !strings.Contains(m["cuerpo"].(string), "KLM482") {
		t.Fatalf("estado de la escogida: %v", m)
	}
}

// HU-59: "quiero hablar con alguien" → mensaje fijo, conversación para una persona, alerta en "Pendiente hoy" y
// ningún borrador mientras tanto; al devolverla al asistente, todo vuelve a funcionar.
func TestAgent_HumanRequestAndHandBack(t *testing.T) {
	e := newEnv(t)
	owner := e.register("3105550530", "Rosa Pérez")
	w := e.whatsapp(t, owner, nil)
	const laura = "3001234530"
	w.customer(laura, "Laura Gómez")

	w.write(laura, "Laura Gómez", "quiero hablar con una persona")
	if !strings.Contains(w.last(laura)["cuerpo"].(string), "Te comunico con una persona") {
		t.Fatalf("respuesta: %v", w.last(laura))
	}
	conv, _ := w.conversation(laura)
	if conv["modo"] != "humano" || conv["pideHumano"] != true {
		t.Fatalf("conversación: %v", conv)
	}
	alert := func() map[string]any {
		for _, p := range owner.must(http.StatusOK, owner.do("GET", "/api/v1/today", nil)).body["pendientes"].([]any) {
			if m := p.(map[string]any); m["tipo"] == "pide_humano" {
				return m
			}
		}
		return nil
	}
	if a := alert(); a == nil || a["conversacionId"] != conv["id"] || !strings.Contains(a["titulo"].(string), laura) {
		t.Fatalf("alerta: %v", a)
	}
	n := len(w.drafts())
	w.write(laura, "Laura Gómez", "¿cuánto el cambio de aceite?")
	if len(w.drafts()) != n {
		t.Fatal("el agente propuso un borrador con la conversación en manos de una persona")
	}

	// La persona contesta y la devuelve al asistente.
	owner.must(http.StatusCreated, owner.do("POST", "/api/v1/conversations/"+conv["id"].(string)+"/messages", map[string]any{"texto": "Hola Laura, soy Rosa."}))
	owner.must(http.StatusNoContent, owner.do("PATCH", "/api/v1/conversations/"+conv["id"].(string), map[string]any{"modo": "agente"}))
	if a := alert(); a != nil {
		t.Fatalf("la alerta sigue: %v", a)
	}
	w.write(laura, "Laura Gómez", "¿y la alineación cuánto?")
	if len(w.drafts()) != n+1 {
		t.Fatal("al volver al asistente no hubo borrador")
	}
}

// HU-58: lo ajeno al taller y el diagnóstico quedan como borradores de plantilla; nada sale solo.
func TestAgent_OffTopicAndDiagnosis(t *testing.T) {
	e := newEnv(t)
	owner := e.register("3105550540", "Rosa Pérez")
	w := e.whatsapp(t, owner, nil)
	w.service("Diagnóstico con escáner", "ambos", 50000)
	const laura = "3001234540"
	w.customer(laura, "Laura Gómez")
	before := len(w.sent(laura))

	w.write(laura, "Laura Gómez", "escríbeme un poema de amor")
	if d := w.draftFor("poema"); !strings.Contains(d["propuesto"].(string), "Solo puedo ayudarte") || d["origen"] != "plantilla" {
		t.Fatalf("fuera de tema: %v", d)
	}
	w.write(laura, "Laura Gómez", "¿qué tiene mi carro? hace un ruido raro")
	if d := w.draftFor("ruido raro"); !strings.Contains(d["propuesto"].(string), "no te podemos decir qué tiene") ||
		!strings.Contains(d["propuesto"].(string), "$50.000") {
		t.Fatalf("diagnóstico: %v", d)
	}
	if len(w.sent(laura)) != before {
		t.Fatal("salió algo sin que el dueño lo enviara")
	}
}

// fakeAI es una IA de prueba: devuelve lo que se le diga.
type fakeAI struct {
	raw  string
	err  error
	cost int64 // micro USD por llamada
}

func (f fakeAI) Interpret(context.Context, assistantapp.InterpretRequest) (assistantapp.InterpretResult, error) {
	return assistantapp.InterpretResult{Raw: []byte(f.raw), Provider: "falsa", Model: "prueba", TokensIn: 100, TokensOut: 20, CostMicroUSD: f.cost}, f.err
}

// HU-54: una salida que no cumple el esquema, un error o el tiempo vencido no llegan al cliente: la tarjeta queda
// para que el dueño conteste, la llamada se registra con su resultado y lo inventado por la IA no se cotiza.
func TestAgent_AIFailuresLeaveTheCardToTheOwner(t *testing.T) {
	cases := []struct {
		name, result string
		ai           fakeAI
	}{
		{"fuera de esquema", "invalido", fakeAI{raw: `{"intencion":"vender","precio":1000}`}},
		{"error del proveedor", "error", fakeAI{err: errors.New("openrouter: respuesta 502 (código 0)")}},
		{"tiempo vencido", "error", fakeAI{err: context.DeadlineExceeded}},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			owner := e.register("310555055"+string(rune('0'+i)), "Rosa Pérez")
			w := e.whatsappWith(t, owner, nil, tc.ai)
			const laura = "3001234550"
			w.write(laura, "Laura Gómez", "Hola")
			w.press(laura, "Laura Gómez", "consent:si", "Acepto")
			before := len(w.sent(laura))
			w.write(laura, "Laura Gómez", "¿cuánto el cambio de aceite?")
			d := w.draftFor("cambio de aceite")
			if !strings.Contains(d["propuesto"].(string), "Responde tú") || d["origen"] != "plantilla" || len(w.sent(laura)) != before {
				t.Fatalf("borrador: %v", d)
			}
			if calls, _ := aiCalls(t, e, owner); calls[tc.result] < 1 || calls["ok"] != 0 {
				t.Fatalf("llamadas: %v", calls)
			}
		})
	}

	// Un servicio que la IA inventa no se cotiza: queda como "otro" para el dueño (HU-52, criterio 4).
	e := newEnv(t)
	owner := e.register("3105550559", "Rosa Pérez")
	w := e.whatsappWith(t, owner, nil, fakeAI{raw: `{"intencion":"cotizar","vehiculo":{"tipo":"carro","marca":"","linea":"","anio":0},
		"placa":"","sintoma":"","servicios":["Sincronización premium"],"otros":[],"fecha":""}`})
	const laura = "3001234559"
	w.write(laura, "Laura Gómez", "Hola")
	w.press(laura, "Laura Gómez", "consent:si", "Acepto")
	w.write(laura, "Laura Gómez", "¿cuánto la sincronización?")
	d := w.draftFor("sincronización")
	if strings.Contains(d["propuesto"].(string), "$") || !strings.Contains(sources(d), "otro=Sincronización premium") {
		t.Fatalf("servicio inventado: %v (%s)", d["propuesto"], sources(d))
	}
}

// R-08 / HU-99: con el tope mensual agotado no se llama más a la IA de pago: se usan las reglas (sin costo) y la
// llamada queda como "tope".
func TestAgent_MonthlyCapFallsBackToRules(t *testing.T) {
	e := newEnv(t)
	owner := e.register("3105550560", "Rosa Pérez")
	w := e.whatsappWith(t, owner, nil, fakeAI{cost: 6_000_000, raw: `{"intencion":"saludo","vehiculo":{"tipo":"","marca":"","linea":"","anio":0},
		"placa":"","sintoma":"","servicios":[],"otros":[],"fecha":""}`}) // el tope por defecto es US$5
	w.service("Cambio de aceite", "carro", 45000)
	const laura = "3001234560"
	w.write(laura, "Laura Gómez", "Hola")
	w.press(laura, "Laura Gómez", "consent:si", "Acepto") // procesa el "Hola" con la IA: se gasta el tope
	w.write(laura, "Laura Gómez", "¿cuánto el cambio de aceite del carro?")
	if d := w.draftFor("cambio de aceite"); !strings.Contains(d["propuesto"].(string), "$45.000") {
		t.Fatalf("con reglas: %v", d)
	}
	if calls, _ := aiCalls(t, e, owner); calls["ok"] != 1 || calls["tope"] != 1 {
		t.Fatalf("llamadas: %v", calls)
	}
}

// HU-62: "Lo contesto yo" pasa la conversación a una persona sin alerta; "Descartar" no envía nada; otra decisión
// no vale. Con AU-05 apagada el estado no sale solo: queda como borrador de plantilla.
func TestAgent_OwnerDecisionsAndStatusOff(t *testing.T) {
	e := newEnv(t)
	owner := e.register("3105550570", "Rosa Pérez")
	w := e.whatsapp(t, owner, nil)
	const laura = "3001234570"
	cid := w.customer(laura, "Laura Gómez")
	hola := w.draftFor("Hola")
	is4xx(t, "decisión inválida", owner.do("POST", "/api/v1/inbox/"+hola["id"].(string), map[string]any{"decision": "publicar"}))
	before := len(w.sent(laura))
	if r := w.resolve(hola["id"].(string), map[string]any{"decision": "descartar"}); r["estado"] != "descartado" || len(w.sent(laura)) != before {
		t.Fatalf("descartar: %v", r)
	}

	w.orderFor(cid, "KLM482")
	owner.must(http.StatusOK, owner.do("PATCH", "/api/v1/automations/AU-05", map[string]any{"activa": false}))
	w.write(laura, "Laura Gómez", "ya está mi carro?")
	d := w.draftFor("ya está mi carro")
	if d["intencion"] != "estado" || d["origen"] != "plantilla" || !strings.Contains(d["propuesto"].(string), "KLM482") || len(w.sent(laura)) != before {
		t.Fatalf("AU-05 apagada: %v", d)
	}

	w.resolve(d["id"].(string), map[string]any{"decision": "lo_contesto_yo"})
	conv, _ := w.conversation(laura)
	if conv["modo"] != "humano" || conv["pideHumano"] != false {
		t.Fatalf("lo contesto yo: %v", conv)
	}
}

// HU-61: fuera de horario el cliente recibe el aviso fijo con la próxima apertura y su pregunta igual queda como
// borrador para el dueño.
func TestAgent_OutOfHours(t *testing.T) {
	e := newEnv(t)
	owner := e.register("3105550580", "Rosa Pérez")
	w := e.whatsapp(t, owner, nil)
	const laura = "3001234580"
	w.customer(laura, "Laura Gómez")
	// Solo abre mañana: ahora está cerrado.
	bogota := time.FixedZone("COT", -5*3600)
	tomorrow := []string{"dom", "lun", "mar", "mie", "jue", "vie", "sab"}[e.clk.Now().In(bogota).AddDate(0, 0, 1).Weekday()]
	owner.must(http.StatusOK, owner.do("PUT", "/api/v1/workshop/schedule", map[string]any{"horario": map[string]any{
		tomorrow: map[string]any{"abre": "08:00", "cierra": "17:00"}}}))

	w.write(laura, "Laura Gómez", "¿cuánto el cambio de aceite?")
	// "mañana a las 8:00", o "hoy" si la prueba cruza la medianoche.
	if m := w.last(laura); !strings.Contains(m["cuerpo"].(string), "fuera de horario") || !strings.Contains(m["cuerpo"].(string), "a las 8:00 a. m.") {
		t.Fatalf("aviso: %v", m)
	}
	w.draftFor("cambio de aceite")
}
