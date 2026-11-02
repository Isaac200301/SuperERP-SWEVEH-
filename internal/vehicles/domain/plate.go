// Package domain contiene las reglas del vehículo: placa, tipo (carro o moto) y motor. Sin I/O.
package domain

import (
	"regexp"
	"strings"

	"sweveh/internal/shared"
)

// PlateFormat es el formato reconocido de una placa (HU-10, docs/08 §4).
type PlateFormat string

const (
	PlateCar        PlateFormat = "carro"        // ABC123 (Res. 3800 de 1995, Oficial)
	PlateMoto       PlateFormat = "moto"         // ABC12D (Tercero, Q-01 provisional)
	PlateMotoLegacy PlateFormat = "moto_antigua" // ABC12  (Tercero, Q-01 provisional)
	PlateSpecial    PlateFormat = "especial"     // 5 a 7 alfanuméricos, se guarda tras confirmar (Q-01)
)

// Kind es el tipo de vehículo.
type Kind string

const (
	KindCar  Kind = "carro"
	KindMoto Kind = "moto"
)

// ValidKind indica si el texto es un tipo conocido.
func ValidKind(k string) bool { return k == string(KindCar) || k == string(KindMoto) }

var (
	reCar        = regexp.MustCompile(`^[A-Z]{3}[0-9]{3}$`)
	reMoto       = regexp.MustCompile(`^[A-Z]{3}[0-9]{2}[A-Z]$`)
	reMotoLegacy = regexp.MustCompile(`^[A-Z]{3}[0-9]{2}$`)
	reSpecial    = regexp.MustCompile(`^[A-Z0-9]{5,7}$`)
)

var (
	ErrPlateInvalid = shared.Validation("placa_invalida",
		"La placa debe tener letras y números, por ejemplo ABC123 (carro) o ABC12D (moto).").WithField("placa")
	ErrSpecialPlateNeedsConfirm = shared.Rule("placa_especial_sin_confirmar",
		"Esta placa no tiene el formato de carro ni de moto. Confirma que quieres guardarla como placa especial.").WithField("placa")
	ErrSpecialPlateNeedsKind = shared.Validation("placa_especial_sin_tipo",
		"Para una placa especial indica si es carro o moto.").WithField("tipo")
)

// Plate es una placa normalizada (mayúsculas, sin espacios ni guiones) con su formato.
type Plate struct {
	value  string
	format PlateFormat
}

func (p Plate) String() string      { return p.value }
func (p Plate) Format() PlateFormat { return p.format }
func (p Plate) IsSpecial() bool     { return p.format == PlateSpecial }

// Kind deduce el tipo de vehículo; en una placa especial no se puede deducir.
func (p Plate) Kind() (Kind, bool) {
	switch p.format {
	case PlateCar:
		return KindCar, true
	case PlateMoto, PlateMotoLegacy:
		return KindMoto, true
	default:
		return "", false
	}
}

// NormalizePlate pasa a mayúsculas y quita espacios, guiones y puntos ("klm-482" → "KLM482").
func NormalizePlate(raw string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(raw) {
		switch r {
		case ' ', '-', '.', '\t':
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// ParsePlate reconoce el formato. Una placa especial se devuelve sin error: quien la use
// decide si pide confirmación (HU-10: "me deja guardarla después de confirmar").
func ParsePlate(raw string) (Plate, error) {
	v := NormalizePlate(raw)
	switch {
	case reCar.MatchString(v):
		return Plate{v, PlateCar}, nil
	case reMoto.MatchString(v):
		return Plate{v, PlateMoto}, nil
	case reMotoLegacy.MatchString(v):
		return Plate{v, PlateMotoLegacy}, nil
	case reSpecial.MatchString(v):
		return Plate{v, PlateSpecial}, nil
	default:
		return Plate{}, ErrPlateInvalid
	}
}

// RestorePlate reconstruye una placa ya guardada sin volver a validar.
func RestorePlate(value string, format PlateFormat) Plate { return Plate{value, format} }
