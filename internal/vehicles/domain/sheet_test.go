package domain

import (
	"errors"
	"testing"

	"sweveh/internal/shared"
)

// Ficha técnica (decisión del 3-nov-2026): qué se exige para recibir y qué se valida por tipo.
func TestEngine_PendingRuleByEnergyAndKind(t *testing.T) {
	cases := []struct {
		name    string
		kind    Kind
		e       Engine
		pending bool
	}{
		{"carro diésel turbo automático", KindCar, Engine{Energy: ptr("diesel"), Aspiration: ptr("turbo"), Transmission: ptr("automatica"), DisplacementCC: ptr(2800)}, false},
		{"carro sin caja", KindCar, Engine{Energy: ptr("gasolina"), Aspiration: ptr("atmosferico"), DisplacementCC: ptr(1600)}, true},
		{"carro sin aspiración", KindCar, Engine{Energy: ptr("gasolina"), Transmission: ptr("manual"), DisplacementCC: ptr(1600)}, true},
		{"eléctrico sin cilindraje", KindCar, Engine{Energy: ptr("electrico"), Transmission: ptr("automatica")}, false},
		{"moto 4T", KindMoto, Engine{Energy: ptr("gasolina"), Aspiration: ptr("atmosferico"), Transmission: ptr("manual"), DisplacementCC: ptr(150), Cycle: ptr("4t")}, false},
		{"moto sin ciclo", KindMoto, Engine{Energy: ptr("gasolina"), Aspiration: ptr("atmosferico"), Transmission: ptr("manual"), DisplacementCC: ptr(150)}, true},
	}
	for _, c := range cases {
		if got := c.e.Pending(c.kind); got != c.pending {
			t.Errorf("%s: pendiente=%v, se esperaba %v", c.name, got, c.pending)
		}
	}
}

func TestEngine_ValidatesValuesByKind(t *testing.T) {
	bad := []struct {
		kind Kind
		e    Engine
		want error
	}{
		{KindMoto, Engine{Energy: ptr("diesel")}, ErrEnergy},
		{KindMoto, Engine{Drive: ptr("4x4")}, ErrDrive},
		{KindCar, Engine{Cycle: ptr("2t")}, ErrCycle},
		{KindCar, Engine{Cooling: ptr("aire")}, ErrCooling},
		{KindCar, Engine{Transmission: ptr("semiautomatica")}, ErrTransmission},
		{KindMoto, Engine{Aspiration: ptr("supercargado")}, ErrAspiration},
	}
	for _, b := range bad {
		if err := b.e.Validate(b.kind); !errors.Is(err, b.want) {
			t.Errorf("%s %+v: %v, se esperaba %v", b.kind, b.e, err, b.want)
		}
	}
}

func TestNewVehicle_SheetDefaultsAndDetails(t *testing.T) {
	p, _ := ParsePlate("KLM482")
	in := NewVehicleInput{Plate: p, Engine: Engine{Energy: ptr("electrico"), DisplacementCC: ptr(1600), Aspiration: ptr("turbo"), Transmission: ptr("automatica")},
		Details: Details{Color: ptr("  gris  plata "), Body: ptr("suv"), Tire: ptr("215/60 r17")}}
	v, err := NewVehicle(shared.NewID(), shared.NewID(), in, now)
	if err != nil {
		t.Fatal(err)
	}
	if v.Engine.DisplacementCC != nil || v.Engine.Aspiration != nil || v.Engine.Turbo != nil {
		t.Errorf("un eléctrico no guarda datos de combustión: %+v", v.Engine)
	}
	if *v.Details.Color != "gris plata" || *v.Details.Tire != "215/60 R17" {
		t.Errorf("detalles normalizados: %q %q", *v.Details.Color, *v.Details.Tire)
	}

	moto, _ := ParsePlate("XVN43F")
	for _, d := range []Details{{Body: ptr("suv")}, {Tire: ptr("muy grande")}} {
		if _, err := NewVehicle(shared.NewID(), shared.NewID(), NewVehicleInput{Plate: moto, Details: d}, now); err == nil {
			t.Errorf("%+v debía rechazarse en moto", d)
		}
	}
}

func TestUpdateEngine_CompletesSheetAndKeepsTheRest(t *testing.T) {
	p, _ := ParsePlate("XVN43F")
	v, _ := NewVehicle(shared.NewID(), shared.NewID(), NewVehicleInput{Plate: p, Engine: Engine{DisplacementCC: ptr(150)}}, now)
	if !v.Engine.Pending(v.Kind) {
		t.Fatal("sin energía, caja ni ciclo la moto queda pendiente")
	}
	_, err := v.UpdateEngine(Engine{Energy: ptr("gasolina"), Transmission: ptr("manual"), Cycle: ptr("2t"), Cooling: ptr("aire")},
		Details{Color: ptr("Rojo"), Tire: ptr("90/90-17")}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if v.Engine.Pending(v.Kind) || *v.Engine.DisplacementCC != 150 || *v.Engine.Aspiration != "atmosferico" || *v.Details.Color != "Rojo" {
		t.Errorf("ficha de la moto: %+v %+v", v.Engine, v.Details)
	}
}
