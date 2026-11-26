package app

import (
	"errors"
	"strings"
	"testing"

	"sweveh/internal/assistant/domain"
)

// R-05: al confirmar se revisa en el servidor el permiso de cada parte de la propuesta, no solo assistant:use. Hoy los
// 3 roles tienen todo lo que pide el asistente (V-05), así que la regla se prueba aquí con un rol que no lo tiene.
func TestFloorAllowedChecksEachPart(t *testing.T) {
	price := int64(1000)
	parts := map[string]domain.FloorProposal{
		"order:create":       {NewOrder: true},
		"vehicle:update":     {OrderID: "o", Sheet: map[string]domain.Field{"marca": {Value: "Mazda"}}},
		"evidence:upload":    {OrderID: "o", Km: &domain.Field{Value: "1"}},
		"order:diagnose":     {OrderID: "o", Findings: []domain.FloorFinding{{Description: "x"}}},
		"quote:edit":         {OrderID: "o", Quote: []domain.FloorLine{{Name: "x", Qty: 1, Price: &price}}},
		"expense:manage":     {OrderID: "o", Expenses: []domain.FloorExpense{{Description: "x", Value: 1}}},
		"order:update_state": {OrderID: "o", Move: &domain.Field{Value: "recibido"}},
		"assistant:use":      {OrderID: "o"},
	}
	for missing, p := range parts {
		f := &Floor{can: func(_ []string, perm string) bool { return perm != missing }}
		if err := f.allowed(p, []string{"x"}); !errors.Is(err, ErrFloorPerm) {
			t.Errorf("sin %s: %v", missing, err)
		}
		all := &Floor{can: func([]string, string) bool { return true }}
		if err := all.allowed(p, nil); err != nil {
			t.Errorf("con todo, %s: %v", missing, err)
		}
	}
}

func TestJoinY(t *testing.T) {
	for in, want := range map[string]string{"": "nada nuevo", "a": "a", "a,b": "a y b", "a,b,c": "a, b y c"} {
		var items []string
		if in != "" {
			items = strings.Split(in, ",")
		}
		if got := joinY(items); got != want {
			t.Errorf("%q: %q", in, got)
		}
	}
}
