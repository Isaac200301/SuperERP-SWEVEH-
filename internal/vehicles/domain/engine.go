package domain

import (
	"math"
	"strconv"
	"strings"

	"sweveh/internal/shared"
)

// Límites del cilindraje aceptado (Propuesta, a calibrar con talleres reales).
const (
	MinDisplacementCC = 50
	MaxDisplacementCC = 10000
)

var ErrDisplacementInvalid = shared.Validation("cilindraje_invalido",
	"Escribe el cilindraje en cc (1200) o en litros (1.2 L).").WithField("cilindrajeCc")

// Engine es el motor del vehículo (decisión del equipo, 1-nov-2026): cilindraje en cc y turbo.
// Un campo en nil significa que todavía no se sabe: "motor pendiente" (HU-11, Q-23).
type Engine struct {
	DisplacementCC *int
	Turbo          *bool
}

// Pending indica si falta algún dato del motor.
func (e Engine) Pending() bool { return e.DisplacementCC == nil || e.Turbo == nil }

// DefaultEngine aplica los valores por defecto del tipo: en moto el turbo viene en No (HU-11).
func DefaultEngine(kind Kind, e Engine) Engine {
	if kind == KindMoto && e.Turbo == nil {
		no := false
		e.Turbo = &no
	}
	return e
}

// ParseDisplacement acepta "1200", "1200 cc", "1.2", "1,2 L" y devuelve siempre cc (HU-11).
// Un número con decimales o menor que 20 se interpreta como litros.
func ParseDisplacement(raw string) (int, error) {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return 0, ErrDisplacementInvalid
	}
	liters := false
	switch {
	case strings.HasSuffix(s, "cc"):
		s = strings.TrimSpace(strings.TrimSuffix(s, "cc"))
	case strings.HasSuffix(s, "l"):
		s = strings.TrimSpace(strings.TrimSuffix(s, "l"))
		liters = true
	}
	s = strings.ReplaceAll(s, ",", ".")
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f <= 0 || math.IsInf(f, 0) || math.IsNaN(f) {
		return 0, ErrDisplacementInvalid
	}
	if liters || strings.Contains(s, ".") || f < 20 {
		f *= 1000
	}
	cc := int(math.Round(f))
	if err := ValidateDisplacement(cc); err != nil {
		return 0, err
	}
	return cc, nil
}

// ValidateDisplacement revisa el rango en cc.
func ValidateDisplacement(cc int) error {
	if cc < MinDisplacementCC || cc > MaxDisplacementCC {
		return ErrDisplacementInvalid
	}
	return nil
}
