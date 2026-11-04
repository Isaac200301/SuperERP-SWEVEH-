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

func (b *bookTaller) CreateOrderFromPlate(_ context.Context, _, _ shared.ID, _, _ string, when *time.Time) (PlateInfo, error) {
	b.booked = when
	return PlateInfo{Owner: "nueva", Code: "OS-0007"}, nil
}

// La tool agendar valida en código la placa escrita por el cliente, la fecha y el horario antes de crear nada.
func TestAgendarValidates(t *testing.T) {
	now := time.Date(2026, 11, 4, 14, 20, 0, 0, domain.Bogota) // miércoles
	for _, tc := range []struct{ name, said, args, want string }{
		{"placa que no escribió", "quiero agendar", `{"placa":"ABC123","sintoma":"aceite","fecha":"2026-11-07","hora":"09:00"}`, "placa_no_confirmada"},
		{"en el pasado", "ABC123", `{"placa":"ABC123","sintoma":"aceite","fecha":"2026-11-03","hora":"09:00"}`, "fecha_invalida"},
		{"domingo", "ABC123", `{"placa":"ABC123","sintoma":"aceite","fecha":"2026-11-08","hora":"09:00"}`, "no abrimos"},
		{"de noche", "ABC123", `{"placa":"ABC123","sintoma":"aceite","fecha":"2026-11-07","hora":"20:00"}`, "atendemos de 8:00"},
	} {
		b := &bookTaller{}
		out := NewTools(b, shared.NewID(), Customer{}, Inbound{}, tc.said, now).Run(context.Background(), "agendar", tc.args)
		if !strings.Contains(out, tc.want) || b.booked != nil {
			t.Errorf("%s: %s (agendó %v)", tc.name, out, b.booked)
		}
	}
	b := &bookTaller{}
	out := NewTools(b, shared.NewID(), Customer{}, Inbound{}, "es la abc-123", now).Run(context.Background(), "agendar",
		`{"placa":"ABC123","sintoma":"aceite","fecha":"2026-11-07","hora":"09:00","nombre":"Carlos Ruiz"}`)
	if !strings.Contains(out, `"agendado"`) || !strings.Contains(out, "sábado 7 de noviembre, 9:00 a. m.") || b.booked == nil ||
		b.named != "Carlos Ruiz" {
		t.Fatalf("agendar: %s, nombre %q", out, b.named)
	}
}
