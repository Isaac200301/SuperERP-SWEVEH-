package rules

import (
	"context"
	"testing"

	"sweveh/internal/assistant/app"
	"sweveh/internal/assistant/domain"
)

func TestInterpreter_OutputPassesTheSchema(t *testing.T) {
	cat := []domain.CatalogItem{{Name: "Revisión de frenos", Vehicle: "ambos"}}
	r, err := Interpreter{}.Interpret(context.Background(), app.InterpretRequest{Text: "mi carro chilla al frenar", Catalog: cat})
	if err != nil || r.Provider != "reglas" || r.CostMicroUSD != 0 {
		t.Fatalf("%+v, %v", r, err)
	}
	u, err := domain.ParseUnderstanding(r.Raw, "mi carro chilla al frenar", domain.Names(cat))
	if err != nil || len(u.Services) != 1 {
		t.Fatalf("%+v, %v", u, err)
	}
}
