package domain

import (
	"errors"
	"testing"
	"time"

	"sweveh/internal/shared"
)

var now = time.Date(2026, 11, 1, 15, 0, 0, 0, time.UTC)

func ptr[T any](v T) *T { return &v }

// HU-10: cada formato de placa se reconoce y se normaliza.
func TestParsePlate_Formats(t *testing.T) {
	cases := []struct {
		in     string
		value  string
		format PlateFormat
		kind   Kind
		known  bool
	}{
		{"KLM482", "KLM482", PlateCar, KindCar, true},       // criterio 1
		{"XVN43F", "XVN43F", PlateMoto, KindMoto, true},     // criterio 2
		{"XVN43", "XVN43", PlateMotoLegacy, KindMoto, true}, // criterio 3: formato antiguo
		{"klm-482", "KLM482", PlateCar, KindCar, true},      // criterio 6: normalizada
		{" klm 482 ", "KLM482", PlateCar, KindCar, true},
		{"R12345", "R12345", PlateSpecial, "", false}, // criterio 4: especial
	}
	for _, c := range cases {
		p, err := ParsePlate(c.in)
		if err != nil {
			t.Fatalf("ParsePlate(%q): %v", c.in, err)
		}
		k, known := p.Kind()
		if p.String() != c.value || p.Format() != c.format || k != c.kind || known != c.known {
			t.Errorf("ParsePlate(%q) = %s/%s/%s/%v", c.in, p, p.Format(), k, known)
		}
	}
}

func TestParsePlate_Invalid(t *testing.T) {
	for _, in := range []string{"", "AB", "ABCDEFGH1", "KL#482", "ÑAB123"} {
		if _, err := ParsePlate(in); !errors.Is(err, ErrPlateInvalid) {
			t.Errorf("ParsePlate(%q) debía ser inválida, dio %v", in, err)
		}
	}
}

// HU-10, criterio 4: la placa especial se guarda solo después de confirmar, con tipo.
func TestNewVehicle_SpecialPlateNeedsConfirmAndKind(t *testing.T) {
	p, _ := ParsePlate("R12345")
	in := NewVehicleInput{Plate: p, CustomerID: shared.NewID()}
	if _, err := NewVehicle(shared.NewID(), shared.NewID(), in, now); !errors.Is(err, ErrSpecialPlateNeedsConfirm) {
		t.Fatalf("sin confirmar: %v", err)
	}
	in.ConfirmSpecial = true
	if _, err := NewVehicle(shared.NewID(), shared.NewID(), in, now); !errors.Is(err, ErrSpecialPlateNeedsKind) {
		t.Fatalf("sin tipo: %v", err)
	}
	in.Kind = "moto"
	v, err := NewVehicle(shared.NewID(), shared.NewID(), in, now)
	if err != nil || v.Kind != KindMoto || v.Plate.Format() != PlateSpecial {
		t.Fatalf("confirmada: %+v %v", v, err)
	}
}

func TestNewVehicle_KindMustMatchPlate(t *testing.T) {
	p, _ := ParsePlate("KLM482")
	_, err := NewVehicle(shared.NewID(), shared.NewID(), NewVehicleInput{Plate: p, Kind: "moto"}, now)
	if !errors.Is(err, ErrKindMismatch) {
		t.Fatalf("carro marcado como moto: %v", err)
	}
}

// HU-11, criterio 3: en moto el turbo viene en No por defecto; criterio 4: motor pendiente.
func TestNewVehicle_MotoTurboDefaultsToNo(t *testing.T) {
	p, _ := ParsePlate("XVN43F")
	v, err := NewVehicle(shared.NewID(), shared.NewID(), NewVehicleInput{Plate: p}, now)
	if err != nil {
		t.Fatal(err)
	}
	if v.Engine.Turbo == nil || *v.Engine.Turbo {
		t.Fatalf("turbo de moto debía ser No, es %v", v.Engine.Turbo)
	}
	if !v.Engine.Pending() {
		t.Fatal("sin cilindraje el motor queda pendiente")
	}
}

func TestNewVehicle_CarWithoutEngineIsPending(t *testing.T) {
	p, _ := ParsePlate("KLM482")
	v, err := NewVehicle(shared.NewID(), shared.NewID(), NewVehicleInput{Plate: p}, now)
	if err != nil || !v.Engine.Pending() || v.Engine.Turbo != nil {
		t.Fatalf("carro sin motor: %+v %v", v.Engine, err)
	}
}

func TestNewVehicle_ValidatesDetails(t *testing.T) {
	p, _ := ParsePlate("KLM482")
	cases := []struct {
		d   Details
		err *shared.Error
	}{
		{Details{Year: ptr(1900)}, ErrYearInvalid},
		{Details{Year: ptr(now.Year() + 2)}, ErrYearInvalid},
		{Details{Mileage: ptr(-1)}, ErrMileageInvalid},
		{Details{Brand: ptr("una marca con un nombre larguísimo que pasa de sesenta letras sin parar")}, ErrTextTooLong},
	}
	for _, c := range cases {
		if _, err := NewVehicle(shared.NewID(), shared.NewID(), NewVehicleInput{Plate: p, Details: c.d}, now); !errors.Is(err, c.err) {
			t.Errorf("detalles %+v: quería %v, dio %v", c.d, c.err, err)
		}
	}
	v, err := NewVehicle(shared.NewID(), shared.NewID(), NewVehicleInput{Plate: p, Details: Details{Brand: ptr("  Chevrolet  "), Line: ptr("")}}, now)
	if err != nil || *v.Details.Brand != "Chevrolet" || v.Details.Line != nil {
		t.Fatalf("limpieza de textos: %+v %v", v.Details, err)
	}
}

// HU-11, criterio 2: 1.2 L o 1200 cc se guardan siempre en cc.
func TestParseDisplacement(t *testing.T) {
	ok := map[string]int{
		"1200": 1200, "1200 cc": 1200, "1200cc": 1200, "1.2": 1200, "1,2 L": 1200, "1.2l": 1200,
		"1.6 L": 1600, "2": 2000, "125": 125, "150 CC": 150, "1.200 cc": 1200,
	}
	for in, want := range ok {
		got, err := ParseDisplacement(in)
		if err != nil || got != want {
			t.Errorf("ParseDisplacement(%q) = %d, %v; quiero %d", in, got, err, want)
		}
	}
	// "15" se lee como 15 L = 15000 cc: fuera de rango.
	for _, in := range []string{"", "abc", "0", "-1200", "20000", "15", "NaN", "Inf", "0.01"} {
		if _, err := ParseDisplacement(in); !errors.Is(err, ErrDisplacementInvalid) {
			t.Errorf("ParseDisplacement(%q) debía fallar, dio %v", in, err)
		}
	}
}

// HU-11, criterio 5: el cambio de motor devuelve el valor anterior para la bitácora.
func TestUpdateEngine_ReturnsPreviousAndBumpsVersion(t *testing.T) {
	p, _ := ParsePlate("KLM482")
	v, _ := NewVehicle(shared.NewID(), shared.NewID(), NewVehicleInput{Plate: p, Engine: Engine{DisplacementCC: ptr(1200), Turbo: ptr(false)}}, now)
	prev, err := v.UpdateEngine(Engine{Turbo: ptr(true)}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if *prev.Turbo || !*v.Engine.Turbo || *v.Engine.DisplacementCC != 1200 || v.Version != 2 {
		t.Fatalf("prev=%+v ahora=%+v versión=%d", prev, v.Engine, v.Version)
	}
	if _, err := v.UpdateEngine(Engine{Turbo: ptr(false)}, 1); !errors.Is(err, shared.ErrVersionConflict) {
		t.Fatalf("versión vieja debía dar conflicto: %v", err)
	}
	if _, err := v.UpdateEngine(Engine{DisplacementCC: ptr(5)}, 2); !errors.Is(err, ErrDisplacementInvalid) {
		t.Fatalf("cilindraje inválido: %v", err)
	}
}
