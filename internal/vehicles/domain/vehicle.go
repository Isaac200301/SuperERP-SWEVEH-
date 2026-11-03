package domain

import (
	"regexp"
	"slices"
	"strings"
	"time"

	"sweveh/internal/shared"
)

var (
	ErrVehicleNotFound = shared.NotFound("vehiculo_no_encontrado", "No encontramos ese vehículo en tu taller.")
	ErrYearInvalid     = shared.Validation("modelo_invalido", "El modelo es el año del vehículo, por ejemplo 2015.").WithField("modeloAnio")
	ErrMileageInvalid  = shared.Validation("kilometraje_invalido", "El kilometraje no puede ser negativo.").WithField("kilometraje")
	ErrTextTooLong     = shared.Validation("texto_largo", "Ese dato puede tener hasta 60 letras.")
	ErrKindMismatch    = shared.Validation("tipo_no_coincide", "La placa no corresponde a ese tipo de vehículo.").WithField("tipo")
	ErrBody            = shared.Validation("carroceria_invalida", "La carrocería es sedán, hatchback, SUV, pick-up, van u otra, y solo aplica a carros.").WithField("carroceria")
	ErrTire            = shared.Validation("llanta_invalida", "Escribe la medida de la llanta como 205/55 R16 o 90/90-17.").WithField("llanta")
)

// Carrocerías del catálogo propio (los nombres comerciales cambian por país).
var bodies = []string{"sedan", "hatchback", "suv", "pickup", "van", "otra"}

// reTire acepta medidas de carro (205/55 R16, 205/55R16) y de moto (90/90-17, 2.75-18).
var reTire = regexp.MustCompile(`^(\d{2,3}/\d{2}\s?Z?R\s?\d{2}|\d{2,3}/\d{2,3}-\d{2}|\d\.\d{2}-\d{2})$`)

// Details son los datos descriptivos del vehículo (HU-11). Todos opcionales al crear.
type Details struct {
	Brand   *string
	Line    *string
	Year    *int
	Mileage *int
	Color   *string
	Body    *string // solo carro
	Tire    *string // medida de llanta
}

// Vehicle es el agregado vehículo. Sin VIN (decisión del equipo).
type Vehicle struct {
	ID         shared.ID
	TallerID   shared.ID
	CustomerID shared.ID
	Kind       Kind
	Plate      Plate
	Details    Details
	Engine     Engine
	Version    int
	CreatedAt  time.Time
}

// NewVehicleInput es lo que llega para registrar un vehículo.
type NewVehicleInput struct {
	Plate          Plate
	Kind           string // obligatorio solo si la placa es especial
	ConfirmSpecial bool
	CustomerID     shared.ID
	Details        Details
	Engine         Engine
}

// NewVehicle aplica las reglas de HU-10 y HU-11.
func NewVehicle(id, tallerID shared.ID, in NewVehicleInput, now time.Time) (Vehicle, error) {
	kind, known := in.Plate.Kind()
	if !known {
		if !in.ConfirmSpecial {
			return Vehicle{}, ErrSpecialPlateNeedsConfirm
		}
		if !ValidKind(in.Kind) {
			return Vehicle{}, ErrSpecialPlateNeedsKind
		}
		kind = Kind(in.Kind)
	} else if in.Kind != "" && Kind(in.Kind) != kind {
		return Vehicle{}, ErrKindMismatch
	}
	details, err := cleanDetails(in.Details, kind, now)
	if err != nil {
		return Vehicle{}, err
	}
	if err := in.Engine.Validate(kind); err != nil {
		return Vehicle{}, err
	}
	return Vehicle{
		ID:         id,
		TallerID:   tallerID,
		CustomerID: in.CustomerID,
		Kind:       kind,
		Plate:      in.Plate,
		Details:    details,
		Engine:     DefaultEngine(kind, in.Engine),
		Version:    1,
		CreatedAt:  now,
	}, nil
}

// UpdateEngine completa o cambia la ficha (motor, caja, tracción y color, carrocería o llanta); devuelve
// la ficha anterior para la bitácora (HU-11). Los campos en nil no cambian.
func (v *Vehicle) UpdateEngine(e Engine, d Details, expectedVersion int) (Engine, error) {
	if expectedVersion != v.Version {
		return Engine{}, shared.ErrVersionConflict
	}
	next := DefaultEngine(v.Kind, v.Engine.merge(e))
	if err := next.Validate(v.Kind); err != nil {
		return Engine{}, err
	}
	sheet := Details{Color: d.Color, Body: d.Body, Tire: d.Tire}
	clean, err := cleanDetails(sheet, v.Kind, time.Time{})
	if err != nil {
		return Engine{}, err
	}
	for _, f := range []struct{ cur, next **string }{{&v.Details.Color, &clean.Color}, {&v.Details.Body, &clean.Body}, {&v.Details.Tire, &clean.Tire}} {
		if *f.next != nil {
			*f.cur = *f.next
		}
	}
	prev := v.Engine
	v.Engine = next
	v.Version++
	return prev, nil
}

func cleanDetails(d Details, kind Kind, now time.Time) (Details, error) {
	var err error
	for _, f := range []struct {
		p     **string
		field string
	}{{&d.Brand, "marca"}, {&d.Line, "linea"}, {&d.Color, "color"}} {
		if *f.p, err = cleanText(*f.p, f.field); err != nil {
			return Details{}, err
		}
	}
	if d.Body != nil && (kind != KindCar || !slices.Contains(bodies, *d.Body)) {
		return Details{}, ErrBody
	}
	if d.Tire, err = cleanText(d.Tire, "llanta"); err != nil {
		return Details{}, err
	}
	if d.Tire != nil {
		t := strings.ToUpper(*d.Tire)
		if !reTire.MatchString(t) {
			return Details{}, ErrTire
		}
		d.Tire = &t
	}
	if d.Year != nil && !now.IsZero() && (*d.Year < 1950 || *d.Year > now.Year()+1) {
		return Details{}, ErrYearInvalid
	}
	if d.Mileage != nil && *d.Mileage < 0 {
		return Details{}, ErrMileageInvalid
	}
	return d, nil
}

func cleanText(s *string, field string) (*string, error) {
	if s == nil {
		return nil, nil
	}
	t := strings.Join(strings.Fields(*s), " ")
	if t == "" {
		return nil, nil
	}
	if len([]rune(t)) > 60 {
		return nil, ErrTextTooLong.WithField(field)
	}
	return &t, nil
}
