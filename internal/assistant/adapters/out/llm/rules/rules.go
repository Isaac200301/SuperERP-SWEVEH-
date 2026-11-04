// Package rules es el intérprete local por reglas (AI_PROVIDER=reglas): sin costo, determinista y sin red.
// Es el valor por defecto y el respaldo cuando se agota el tope mensual del taller.
package rules

import (
	"context"
	"encoding/json"

	"sweveh/internal/assistant/app"
	"sweveh/internal/assistant/domain"
)

// Model identifica la versión de las reglas en LlamadaIA (HU-99).
const Model = "reglas-v2"

type Interpreter struct{}

func (Interpreter) Interpret(_ context.Context, r app.InterpretRequest) (app.InterpretResult, error) {
	raw, err := json.Marshal(domain.Interpret(r.Text, r.Catalog))
	return app.InterpretResult{Raw: raw, Provider: "reglas", Model: Model}, err
}
