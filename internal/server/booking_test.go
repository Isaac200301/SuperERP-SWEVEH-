package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	assistantapp "sweveh/internal/assistant/app"
	"sweveh/internal/assistant/domain"
	"sweveh/internal/platform/reqctx"
	"sweveh/internal/shared"
)

var dayKeys = []string{"dom", "lun", "mar", "mie", "jue", "vie", "sab"}

// book es lo que el asesor de prueba manda a agendar: placa, fecha y hora (y nombre si lo hay).
func book(plate string, when time.Time, name string) assistantapp.ChatMessage {
	args, _ := json.Marshal(map[string]string{"placa": plate, "sintoma": "cambio de aceite", "fecha": when.Format("2006-01-02"),
		"hora": when.Format("15:04"), "nombre": name})
	return call("agendar", string(args))
}

// ADR-0029 / HU-55: el asesor agenda con día y hora. La cita queda en la fecha prometida de la orden "Por recibir" y
// AU-06 se la recuerda al cliente ese día; un día que el taller no abre no se agenda; el nombre de relleno se
// reemplaza por el que da el cliente; y cambiar la cita reagenda la misma orden en vez de crear otra.
func TestAdvisor_BooksWithDateTimeAndName(t *testing.T) {
	e := newEnv(t)
	owner := e.register("3105550703", "Rosa Pérez")
	adv := &fakeAdvisor{}
	w := e.advisorWA(t, owner, adv)

	// Cita en dos días a las 10:00 (hora de Colombia); el día siguiente a la cita el taller no abre.
	now := e.clk.Now().In(domain.Bogota)
	when := time.Date(now.Year(), now.Month(), now.Day()+2, 10, 0, 0, 0, domain.Bogota)
	closed := when.AddDate(0, 0, 1)
	sched := map[string]any{}
	for _, k := range dayKeys {
		if k != dayKeys[closed.Weekday()] {
			sched[k] = map[string]any{"abre": "08:00", "cierra": "18:00"}
		}
	}
	owner.must(http.StatusOK, owner.do("PUT", "/api/v1/workshop/schedule", map[string]any{"horario": sched}))
	owner.must(http.StatusOK, owner.do("PATCH", "/api/v1/automations/AU-06", map[string]any{"activa": true}))

	const phone = "3001234703"
	w.customer(phone, "Cliente de prueba")
	var results []string
	step := func(msg assistantapp.ChatMessage, reply string) {
		adv.step = func(r assistantapp.ChatRequest) (assistantapp.ChatMessage, error) {
			if res := toolResult(r); res != "" {
				results = append(results, res)
				return say(reply), nil
			}
			return msg, nil
		}
	}
	open := func() []any {
		return owner.must(http.StatusOK, owner.do("GET", "/api/v1/orders?abiertas=1", nil)).body["ordenes"].([]any)
	}

	// Día que no abre: no se agenda nada y la tool lo dice.
	step(book("MTO45E", closed, ""), "Ese día no abrimos, ¿te sirve otro?")
	w.write(phone, "Cliente de prueba", "agéndame la MTO45E")
	if len(open()) != 0 || !strings.Contains(results[len(results)-1], "fuera_de_horario") {
		t.Fatalf("día cerrado: abiertas %d, %s", len(open()), results[len(results)-1])
	}

	// Cita válida con el nombre: orden "Por recibir" con la fecha prometida y el nombre guardado.
	step(book("MTO45E", when, "Carlos Ruiz"), "Listo Carlos, te esperamos.")
	w.write(phone, "Cliente de prueba", "sí, confirmo, soy Carlos Ruiz")
	res := results[len(results)-1]
	if !strings.Contains(res, `"agendado"`) || !strings.Contains(res, domain.FormatWhen(when)) {
		t.Fatalf("agendar: %s", res)
	}
	orders := open()
	if len(orders) != 1 {
		t.Fatalf("órdenes abiertas: %d", len(orders))
	}
	o := orders[0].(map[string]any)
	if o["estado"] != "por_recibir" || o["fechaPrometida"] == nil {
		t.Fatalf("orden: %v", o)
	}
	if got, _ := time.Parse(time.RFC3339, o["fechaPrometida"].(string)); !got.Equal(when) {
		t.Fatalf("fecha prometida %v; quiero %v", got, when)
	}
	if c := owner.must(http.StatusOK, owner.do("GET", "/api/v1/customers/by-phone/"+phone, nil)).body; c["nombre"] != "Carlos Ruiz" {
		t.Fatalf("nombre: %v", c["nombre"])
	}

	// Cambiar la cita: la misma orden queda con la nueva hora.
	later := when.Add(4 * time.Hour)
	step(book("MTO45E", later, "Otro Nombre"), "Te cambié la cita.")
	w.write(phone, "Carlos Ruiz", "mejor a las 2 de la tarde, la MTO45E")
	if res := results[len(results)-1]; !strings.Contains(res, `"reagendado"`) || len(open()) != 1 {
		t.Fatalf("reagendar: %s, abiertas %d", res, len(open()))
	}
	if c := owner.must(http.StatusOK, owner.do("GET", "/api/v1/customers/by-phone/"+phone, nil)).body; c["nombre"] != "Carlos Ruiz" {
		t.Fatalf("un nombre real no se cambia: %v", c["nombre"])
	}

	// El día de la cita, AU-06 le recuerda al cliente. Sin plantilla aprobada por Meta (Q-07) solo sale con la ventana
	// de 24 h abierta (HU-47): aquí el cliente escribió esa mañana.
	tid := tallerOf(t, owner)
	e.clk.Advance(later.Sub(e.clk.Now()) - 7*time.Hour)
	w.owner = e.login("3105550703") // pasaron dos días: la sesión del dueño venció
	adv.step = func(assistantapp.ChatRequest) (assistantapp.ChatMessage, error) { return say("¡Te esperamos hoy!"), nil }
	w.write(phone, "Carlos Ruiz", "hoy voy para allá")
	if err := w.s.Automations.Daily(context.Background(), []shared.ID{tid}, reqctx.WithTenant); err != nil {
		t.Fatal(err)
	}
	w.pump()
	if got := w.sentBy(phone, "AU-06"); len(got) != 1 {
		t.Fatalf("recordatorio AU-06: %v; ejecuciones %v", got, w.runsOf("AU-06"))
	}
}
