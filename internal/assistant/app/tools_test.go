package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"sweveh/internal/assistant/domain"
	"sweveh/internal/shared"
)

// bookTaller es un taller de prueba para la tool agendar: abre lunes a sábado de 8 a 18 y registra lo que agenda.
type bookTaller struct {
	Taller
	booked *time.Time
	named  string
	owner  string // "" = placa nueva
	handed bool   // pasó la conversación a una persona
}

func (b *bookTaller) OpenAt(_ context.Context, _ shared.ID, at time.Time) (bool, string, error) {
	at = at.In(domain.Bogota)
	if at.Weekday() == time.Sunday {
		return false, "", nil
	}
	hm := at.Format("15:04")
	return hm >= "08:00" && hm < "18:00", "8:00 a. m. a 6:00 p. m.", nil
}

func (b *bookTaller) SetCustomerName(_ context.Context, _, _ shared.ID, name string) (bool, error) {
	b.named = name
	return true, nil
}

func (b *bookTaller) PlateOf(context.Context, shared.ID, shared.ID, string) (PlateInfo, error) {
	if b.owner != "" {
		return PlateInfo{Owner: b.owner}, nil
	}
	return PlateInfo{Owner: "nueva"}, nil
}

func (b *bookTaller) SetMode(_ context.Context, _, _ shared.ID, mode string, _ bool) error {
	b.handed = mode == "humano"
	return nil
}

func (b *bookTaller) CreateOrderFromPlate(_ context.Context, _, _ shared.ID, _, _ string, when *time.Time) (PlateInfo, error) {
	b.booked = when
	return PlateInfo{Owner: "nueva", Code: "OS-0007"}, nil
}

// La tool agendar valida en código, en orden, la placa escrita por el cliente, el nombre, la fecha, el horario y la
// confirmación antes de crear nada.
func TestAgendarValidates(t *testing.T) {
	now := time.Date(2026, 11, 4, 14, 20, 0, 0, domain.Bogota) // miércoles
	for _, tc := range []struct{ name, said, args, want string }{
		{"placa que no escribió", "quiero agendar", `{"placa":"ABC123","sintoma":"aceite","fecha":"2026-11-07","hora":"09:00"}`, "placa_no_confirmada"},
		{"en el pasado", "ABC123", `{"placa":"ABC123","sintoma":"aceite","fecha":"2026-11-03","hora":"09:00"}`, "fecha_invalida"},
		{"domingo", "ABC123", `{"placa":"ABC123","sintoma":"aceite","fecha":"2026-11-08","hora":"09:00"}`, "no abrimos"},
		{"de noche", "ABC123", `{"placa":"ABC123","sintoma":"aceite","fecha":"2026-11-07","hora":"20:00"}`, "atendemos de 8:00"},
		{"sin día", "ABC123", `{"placa":"ABC123","sintoma":"aceite"}`, "falta_fecha"},
		{"sin nombre", "ABC123", `{"placa":"ABC123","sintoma":"aceite"}`, "falta_nombre"},
	} {
		b := &bookTaller{}
		c := Customer{Name: "Laura Gómez"}
		if tc.name == "sin nombre" {
			c.Name = "Cliente de prueba"
		}
		out := NewTools(b, shared.NewID(), c, Inbound{Text: "sí"}, tc.said, now).Run(context.Background(), "agendar", tc.args)
		if !strings.Contains(out, tc.want) || b.booked != nil {
			t.Errorf("%s: %s (agendó %v)", tc.name, out, b.booked)
		}
	}
	// Todo listo pero el cliente aún no dijo que sí: devuelve el resumen y no agenda.
	args := `{"placa":"ABC123","sintoma":"aceite","fecha":"2026-11-07","hora":"09:00","nombre":"Carlos Ruiz"}`
	b := &bookTaller{}
	out := NewTools(b, shared.NewID(), Customer{}, Inbound{Text: "el sábado a las 9"}, "es la abc-123", now).Run(context.Background(), "agendar", args)
	if !strings.Contains(out, `"confirmar"`) || b.booked != nil || b.named != "" {
		t.Fatalf("sin confirmar: %s", out)
	}
	out = NewTools(b, shared.NewID(), Customer{}, Inbound{Text: "sí, confirmo"}, "es la abc-123", now).Run(context.Background(), "agendar", args)
	if !strings.Contains(out, `"agendado"`) || !strings.Contains(out, "sábado 7 de noviembre, 9:00 a. m.") || b.booked == nil ||
		b.named != "Carlos Ruiz" {
		t.Fatalf("agendar: %s, nombre %q", out, b.named)
	}
}

// Una placa de otro cliente no corta la conversación: primero se piden el nombre y la cita, y solo con la solicitud
// completa pasa a una persona, sin decir nada del vehículo ni crear la orden (HU-12).
func TestAgendarForeignPlateAfterBooking(t *testing.T) {
	now := time.Date(2026, 11, 4, 14, 20, 0, 0, domain.Bogota)
	run := func(b *bookTaller, args string) string {
		return NewTools(b, shared.NewID(), Customer{Name: "Cliente de prueba"}, Inbound{Text: "sí"}, "es la GNO667", now).
			Run(context.Background(), "agendar", args)
	}
	b := &bookTaller{owner: "ajena"}
	if out := run(b, `{"placa":"GNO667","sintoma":"pintura"}`); !strings.Contains(out, "falta_nombre") || b.handed {
		t.Fatalf("sin nombre: %s (persona %v)", out, b.handed)
	}
	if out := run(b, `{"placa":"GNO667","sintoma":"pintura","nombre":"Pablo"}`); !strings.Contains(out, "falta_fecha") || b.handed {
		t.Fatalf("sin cita: %s (persona %v)", out, b.handed)
	}
	out := run(b, `{"placa":"GNO667","sintoma":"pintura","fecha":"2026-11-07","hora":"09:00","nombre":"Pablo"}`)
	if !strings.Contains(out, `"revision"`) || !strings.Contains(out, "sábado 7 de noviembre, 9:00 a. m.") || !b.handed ||
		b.booked != nil || b.named != "Pablo" || strings.Contains(out, "GNO667") {
		t.Fatalf("placa ajena: %s (persona %v, agendó %v, nombre %q)", out, b.handed, b.booked, b.named)
	}
}
