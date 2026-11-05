package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"sweveh/internal/platform/config"
	"sweveh/internal/platform/testdb"
)

// HU-74 (ADR-0030): el administrador del ERP ve, por taller, los mensajes enviados del mes contra los incluidos y el
// costo de IA en pesos; coincide con lo registrado en la base. Un usuario de taller no lo ve: ni en /me ni por la API.
func TestPlatform_UsageMeterForAdminsOnly(t *testing.T) {
	cfg := config.API{Env: config.EnvLocal, Platform: config.Platform{Admins: []string{"+573105550640"}, IncludedMessages: 1000, COPPerUSD: 4000}}
	e := newEnvWith(t, cfg)
	admin := e.register("3105550640", "Admin ERP")
	owner := e.register("3105550641", "Rosa Pérez")

	// Taller de Rosa: un cliente escribe; la IA de prueba cuesta 300.000 micro USD por llamada (US$0,30 = $1.200).
	w := e.whatsappWith(t, owner, nil, fakeAI{raw: `{"intencion":"saludo","vehiculo":{"tipo":"","marca":"","linea":"","anio":0},
		"placa":"","sintoma":"","servicios":[],"otros":[],"fecha":""}`, cost: 300_000})
	w.customer("3001234641", "Laura Gómez")

	if me := admin.must(http.StatusOK, admin.do("GET", "/api/v1/me", nil)).body; me["adminPlataforma"] != true {
		t.Fatalf("/me del administrador: %v", me)
	}
	if me := owner.must(http.StatusOK, owner.do("GET", "/api/v1/me", nil)).body; me["adminPlataforma"] != false {
		t.Fatalf("/me del dueño: %v", me)
	}
	if r := owner.do("GET", "/api/v1/platform/usage", nil); r.status != http.StatusForbidden || r.str("code") != "sin_permiso" {
		t.Fatalf("el dueño ve el medidor: %d %v", r.status, r.body)
	}

	body := admin.must(http.StatusOK, admin.do("GET", "/api/v1/platform/usage", nil)).body
	rows := body["talleres"].([]any)
	if len(rows) != 2 {
		t.Fatalf("talleres: %v", rows)
	}
	first := rows[0].(map[string]any) // el que más cuesta va primero
	var sent int
	var cost int64
	tid := tallerOf(t, owner)
	if err := e.db.Tx.Do(testdb.As(tid), func(ctx context.Context) error {
		if err := queryRow(ctx, `SELECT count(*) FROM messaging.mensaje WHERE direccion = 'saliente' AND estado IN ('enviado','entregado','leido')`, &sent); err != nil {
			return err
		}
		return queryRow(ctx, `SELECT COALESCE(sum(costo_micro_usd), 0) FROM assistant.llamada_ia`, &cost)
	}); err != nil {
		t.Fatal(err)
	}
	if first["id"] != tid.String() || first["mensajes"] != float64(sent) || first["iaMicroUsd"] != float64(cost) || sent == 0 || cost == 0 {
		t.Fatalf("no coincide con la base (%d mensajes, %d micro USD): %v", sent, cost, first)
	}
	if first["mensajesTexto"] != itoaThousands(sent)+" de 1.000" || first["iaTexto"] != "IA $"+itoaThousands(int(cost*4000/1_000_000)) ||
		first["sePasa"] != false {
		t.Fatalf("textos: %v", first)
	}
	if second := rows[1].(map[string]any); second["mensajes"] != float64(0) || second["iaTexto"] != "IA $0" {
		t.Fatalf("taller del administrador: %v", second)
	}
}

// itoaThousands escribe el número con punto de miles, como el medidor: 1200 → "1.200".
func itoaThousands(n int) string {
	d := strconv.Itoa(n)
	out := ""
	for i, c := range d {
		if i > 0 && (len(d)-i)%3 == 0 {
			out += "."
		}
		out += string(c)
	}
	return out
}

// HU-100: el administrador busca un número y ve qué entró y salió en cada taller con su estado, el error y la hora,
// y si el webhook llegó y se procesó. Sin el texto de los mensajes. Un dueño de taller no lo ve.
func TestPlatform_MessageLogByPhone(t *testing.T) {
	cfg := config.API{Env: config.EnvLocal, Platform: config.Platform{Admins: []string{"+573105550650"}}}
	e := newEnvWith(t, cfg)
	admin := e.register("3105550650", "Admin ERP")
	owner := e.register("3105550651", "Rosa Pérez")
	ch := &flaky{}
	w := e.whatsapp(t, owner, ch)
	cid := w.customer("3001234651", "Laura Gómez")
	ch.fail, ch.retryable = 1, false
	id, _ := w.orderFor(cid, "LOG651")
	w.move(id, "recibido", nil) // AU-01 falla del todo

	if r := owner.do("GET", "/api/v1/platform/messages?celular=3001234651", nil); r.status != http.StatusForbidden {
		t.Fatalf("el dueño ve el registro técnico: %d", r.status)
	}
	if r := admin.do("GET", "/api/v1/platform/messages?celular=123", nil); r.status != http.StatusBadRequest {
		t.Fatalf("celular inválido: %d", r.status)
	}
	body := admin.must(http.StatusOK, admin.do("GET", "/api/v1/platform/messages?celular=300%20123%204651", nil)).body
	var failed, in map[string]any
	for _, m := range body["mensajes"].([]any) {
		m := m.(map[string]any)
		if m["taller"] != "Taller de Rosa" && m["taller"] == nil {
			t.Fatalf("mensaje sin taller: %v", m)
		}
		if _, ok := m["texto"]; ok {
			t.Fatalf("el registro técnico muestra el texto: %v", m)
		}
		switch {
		case m["estado"] == "fallido":
			failed = m
		case m["direccion"] == "entrante":
			in = m
		}
	}
	if failed == nil || failed["error"] == nil || failed["fecha"] == nil || in == nil {
		t.Fatalf("registro: %v", body["mensajes"])
	}
	if body["celular"] != "3001234651" || body["diasWebhooks"] != float64(7) {
		t.Fatalf("encabezado: %v", body)
	}
}

// HU-100: el webhook crudo que menciona el número sale en el registro técnico con la hora en que llegó y si se procesó.
func TestPlatform_MessageLogShowsWebhooks(t *testing.T) {
	f := &fakeWAHA{status: "WORKING"}
	srv := httptest.NewServer(f.handler(t))
	t.Cleanup(srv.Close)
	cfg := config.API{Env: config.EnvLocal, Platform: config.Platform{Admins: []string{"+573105550660"}},
		WhatsApp: config.WhatsApp{Provider: "local", WAHA: config.WAHA{URL: srv.URL, APIKey: "k", WebhookSecret: "s", Session: "default"}}}
	e := newEnvWith(t, cfg)
	admin := e.register("3105550660", "Admin ERP")
	body := `{"event":"message","session":"otra","engine":"GOWS","payload":{"id":"x1","timestamp":1,"from":"573001234661@c.us",` +
		`"fromMe":false,"body":"hola","hasMedia":false}}`
	if s := postWAHA(t, e, []byte(body), "s"); s != http.StatusOK {
		t.Fatalf("webhook: %d", s)
	}
	hooks := admin.must(http.StatusOK, admin.do("GET", "/api/v1/platform/messages?celular=3001234661", nil)).body["webhooks"].([]any)
	if len(hooks) != 1 || hooks[0].(map[string]any)["recibido"] == nil {
		t.Fatalf("webhooks: %v", hooks)
	}
	if other := admin.must(http.StatusOK, admin.do("GET", "/api/v1/platform/messages?celular=3001234662", nil)).body["webhooks"].([]any); len(other) != 0 {
		t.Fatalf("webhooks de otro número: %v", other)
	}
}
