package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	ok := func() Part { return Part{Ref: " pf 20 ", Name: "  Pastillas   de freno ", Price: 1} }
	p := ok()
	if err := p.Normalize(); err != nil || p.Ref != "PF-20" || p.Name != "Pastillas de freno" || p.Kind != "ambos" {
		t.Fatalf("normalizado: %+v %v", p, err)
	}
	cases := map[string]struct {
		mut  func(*Part)
		want error
	}{
		"referencia vacía":      {func(p *Part) { p.Ref = " " }, ErrRef},
		"referencia con signos": {func(p *Part) { p.Ref = "A/B" }, ErrRef},
		"referencia con guion":  {func(p *Part) { p.Ref = "-AB" }, ErrRef},
		"referencia muy larga":  {func(p *Part) { p.Ref = strings.Repeat("A", 41) }, ErrRef},
		"sin nombre":            {func(p *Part) { p.Name = "" }, ErrName},
		"tipo inválido":         {func(p *Part) { p.Kind = "bus" }, ErrKind},
		"existencia negativa":   {func(p *Part) { p.Stock = -1 }, ErrMoney},
		"mínimo negativo":       {func(p *Part) { p.Min = -1 }, ErrMoney},
	}
	for name, c := range cases {
		p := ok()
		c.mut(&p)
		if err := p.Normalize(); !errors.Is(err, c.want) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if (Part{Active: true, Stock: 1, Min: 1}).LowStock() != true || (Part{Active: false, Stock: 0}).LowStock() {
		t.Error("LowStock")
	}
}
