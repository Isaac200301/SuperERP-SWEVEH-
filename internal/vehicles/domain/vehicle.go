package domain

import (
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
)

// Details son los datos descriptivos del vehículo (HU-11). Todos opcionales al crear.
type Details struct {
	Brand   *string
	Line    *string
	Year    *int
	Mileage *int
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
	details, err := cleanDetails(in.Details, now)
	if err != nil {
		return Vehicle{}, err
	}
	if in.Engine.DisplacementCC != nil {
		if err := ValidateDisplacement(*in.Engine.DisplacementCC); err != nil {
			return Vehicle{}, err
		}
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

// UpdateEngine cambia cilindraje y/o turbo; devuelve el motor anterior para la bitácora (HU-11).
func (v *Vehicle) UpdateEngine(e Engine, expectedVersion int) (Engine, error) {
	if expectedVersion != v.Version {
		return Engine{}, shared.ErrVersionConflict
	}
	if e.DisplacementCC != nil {
		if err := ValidateDisplacement(*e.DisplacementCC); err != nil {
			return Engine{}, err
		}
	}
	prev := v.Engine
	next := prev
	if e.DisplacementCC != nil {
		next.DisplacementCC = e.DisplacementCC
	}
	if e.Turbo != nil {
		next.Turbo = e.Turbo
	}
	v.Engine = next
	v.Version++
	return prev, nil
}

func cleanDetails(d Details, now time.Time) (Details, error) {
	var err error
	if d.Brand, err = cleanText(d.Brand, "marca"); err != nil {
		return Details{}, err
	}
	if d.Line, err = cleanText(d.Line, "linea"); err != nil {
		return Details{}, err
	}
	if d.Year != nil && (*d.Year < 1950 || *d.Year > now.Year()+1) {
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
