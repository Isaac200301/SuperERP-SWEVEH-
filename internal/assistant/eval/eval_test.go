package eval

import (
	"context"
	"strings"
	"testing"

	"sweveh/internal/assistant/adapters/out/llm/rules"
)

// Con las reglas (sin red, en CI): el juego está bien armado y ningún borrador sale con precios fuera de lista.
// Las reglas cumplen HU-102 (al menos 25 de 30) y no pueden bajar de su línea base; cada modelo candidato se mide
// con el mismo juego usando cmd/agent-eval (D-08).
func TestDefaultSet_WithRules(t *testing.T) {
	s, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	moto := 0
	for _, c := range s.Cases {
		if c.Vehicle == "moto" {
			moto++
		}
	}
	if len(s.Cases) != 30 || moto != 10 {
		t.Fatalf("el juego debe tener 30 mensajes con 10 de moto: %d y %d", len(s.Cases), moto)
	}
	rep := Run(context.Background(), rules.Interpreter{}, s)
	var out strings.Builder
	rep.Print(&out)
	t.Log("\n" + out.String())
	if rep.OutOfList != 0 || rep.Invalid != 0 {
		t.Fatalf("precios fuera de lista %d, salidas inválidas %d", rep.OutOfList, rep.Invalid)
	}
	if !rep.Passes() || rep.Services < rulesBaseline {
		t.Fatalf("las reglas sacaron %d servicios bien; la línea base es %d", rep.Services, rulesBaseline)
	}
}

// rulesBaseline es lo que logran hoy las reglas (28 de 30, medido el 2026-11-04).
const rulesBaseline = 28
