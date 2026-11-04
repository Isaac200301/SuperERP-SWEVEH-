package server_test

import (
	"context"
	"net/http"
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
