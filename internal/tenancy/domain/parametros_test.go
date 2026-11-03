package domain

import (
	"errors"
	"testing"
)

func TestParametros_DefaultsAndValidation(t *testing.T) {
	p := DefaultParametros()
	if err := p.Validate(); err != nil || p.VATPct != 19 || p.MinPhotos != 4 || len(p.ItemsFor("moto")) == 0 {
		t.Fatalf("por defecto: %+v %v", p, err)
	}
	neg := int64(-1)
	for name, c := range map[string]struct {
		mod  func(*Parametros)
		want error
	}{
		"fotos":     {func(p *Parametros) { p.MinPhotos = 9 }, ErrParamPhotos},
		"iva":       {func(p *Parametros) { p.VATPct = 31 }, ErrParamVAT},
		"tarifa":    {func(p *Parametros) { p.HourlyRate = &neg }, ErrParamRate},
		"descuento": {func(p *Parametros) { p.MaxDiscountPct = 101 }, ErrParamDiscount},
		"vigencia":  {func(p *Parametros) { p.ValidityDays = 0 }, ErrParamValidity},
		"repetido":  {func(p *Parametros) { p.CarItems = []ChecklistItem{{Name: "Luces"}, {Name: "  luces "}} }, ErrParamItems},
		"vacío":     {func(p *Parametros) { p.MotoItems = []ChecklistItem{{Name: "  "}} }, ErrParamItems},
	} {
		q := DefaultParametros()
		c.mod(&q)
		if err := q.Validate(); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, se esperaba %v", name, err, c.want)
		}
	}
	q := DefaultParametros()
	q.CarItems = []ChecklistItem{{Name: "  Llanta   de repuesto "}, {Key: "radio", Name: "Radio original"}}
	if err := q.Validate(); err != nil || q.CarItems[0].Key != "llanta_de_repuesto" || q.CarItems[0].Name != "Llanta de repuesto" || q.CarItems[1].Key != "radio" {
		t.Fatalf("ítems: %+v %v", q.CarItems, err)
	}
}
