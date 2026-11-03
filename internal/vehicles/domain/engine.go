package domain

import (
	"math"
	"slices"
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

// Engine es la ficha mecánica del vehículo. Reemplaza la decisión del 1-nov-2026 ("cilindraje + turbo")
// por decisión de Pablo del 3-nov-2026: energía, aspiración, caja y, según el tipo, tracción o ciclo y
// refrigeración. Un campo en nil significa que todavía no se sabe (HU-11, Q-23).
type Engine struct {
	DisplacementCC *int
	Turbo          *bool   // derivado de Aspiration; se conserva para avisos, IA y la API anterior
	Energy         *string // gasolina | diesel | gas | hibrido | hibrido_enchufable | electrico
	Aspiration     *string // atmosferico | turbo | supercargado
	Transmission   *string
	Drive          *string // solo carro: delantera | trasera | awd | 4x4
	Cycle          *string // solo moto: 2t | 4t
	Cooling        *string // solo moto: aire | liquido
}

// Valores permitidos por tipo de vehículo (Propuesta; las motos diésel o con tracción no aplican).
var (
	energies = map[Kind][]string{KindCar: {"gasolina", "diesel", "gas", "hibrido", "hibrido_enchufable", "electrico"},
		KindMoto: {"gasolina", "electrico"}}
	aspirations   = map[Kind][]string{KindCar: {"atmosferico", "turbo", "supercargado"}, KindMoto: {"atmosferico", "turbo"}}
	transmissions = map[Kind][]string{KindCar: {"manual", "automatica", "cvt", "doble_embrague", "amt"},
		KindMoto: {"manual", "automatica", "semiautomatica"}}
	drives   = map[Kind][]string{KindCar: {"delantera", "trasera", "awd", "4x4"}}
	cycles   = map[Kind][]string{KindMoto: {"2t", "4t"}}
	coolings = map[Kind][]string{KindMoto: {"aire", "liquido"}}

	ErrEnergy       = shared.Validation("energia_invalida", "Escoge el combustible o la energía del vehículo.").WithField("energia")
	ErrAspiration   = shared.Validation("aspiracion_invalida", "Escoge si el motor es atmosférico, turbo o supercargado.").WithField("aspiracion")
	ErrTransmission = shared.Validation("transmision_invalida", "Escoge el tipo de caja del vehículo.").WithField("transmision")
	ErrDrive        = shared.Validation("traccion_invalida", "La tracción es delantera, trasera, AWD o 4x4, y solo aplica a carros.").WithField("traccion")
	ErrCycle        = shared.Validation("ciclo_invalido", "El ciclo es 2 o 4 tiempos, y solo aplica a motos.").WithField("ciclo")
	ErrCooling      = shared.Validation("refrigeracion_invalida", "La refrigeración es por aire o por líquido, y solo aplica a motos.").WithField("refrigeracion")
)

func validIn(v *string, allowed map[Kind][]string, kind Kind, err error) error {
	if v != nil && !slices.Contains(allowed[kind], *v) {
		return err
	}
	return nil
}

// Electric dice si el vehículo no tiene motor de combustión: sin cilindraje ni aspiración.
func (e Engine) Electric() bool { return e.Energy != nil && *e.Energy == "electrico" }

// Validate revisa cada valor contra lo que permite el tipo de vehículo.
func (e Engine) Validate(kind Kind) error {
	if e.DisplacementCC != nil {
		if err := ValidateDisplacement(*e.DisplacementCC); err != nil {
			return err
		}
	}
	for _, c := range []struct {
		v       *string
		allowed map[Kind][]string
		err     error
	}{{e.Energy, energies, ErrEnergy}, {e.Aspiration, aspirations, ErrAspiration}, {e.Transmission, transmissions, ErrTransmission},
		{e.Drive, drives, ErrDrive}, {e.Cycle, cycles, ErrCycle}, {e.Cooling, coolings, ErrCooling}} {
		if err := validIn(c.v, c.allowed, kind, c.err); err != nil {
			return err
		}
	}
	return nil
}

// Pending indica si falta un dato que el taller necesita para recibir el vehículo (HU-11 criterio 4):
// energía y caja siempre; en combustión, cilindraje y aspiración; en moto de combustión, el ciclo.
// La tracción, la refrigeración y lo demás de la ficha no bloquean.
func (e Engine) Pending(kind Kind) bool {
	if e.Energy == nil || e.Transmission == nil {
		return true
	}
	if e.Electric() {
		return false
	}
	return e.DisplacementCC == nil || e.Aspiration == nil || (kind == KindMoto && e.Cycle == nil)
}

// DefaultEngine completa lo deducible: el turbo de la API anterior da la aspiración, la moto viene atmosférica
// por defecto (HU-11 criterio 3), el eléctrico no lleva datos de combustión y Turbo se deriva de la aspiración.
func DefaultEngine(kind Kind, e Engine) Engine {
	if e.Aspiration == nil && e.Turbo != nil {
		a := map[bool]string{true: "turbo", false: "atmosferico"}[*e.Turbo]
		e.Aspiration = &a
	}
	if kind == KindMoto && e.Aspiration == nil {
		a := "atmosferico"
		e.Aspiration = &a
	}
	if e.Electric() {
		e.DisplacementCC, e.Aspiration, e.Cycle = nil, nil, nil
	}
	e.Turbo = nil
	if e.Aspiration != nil {
		t := *e.Aspiration != "atmosferico"
		e.Turbo = &t
	}
	return e
}

// merge aplica sobre e los campos que vienen en n (nil = no cambia).
func (e Engine) merge(n Engine) Engine {
	pick := func(cur, next *string) *string {
		if next != nil {
			return next
		}
		return cur
	}
	if n.DisplacementCC != nil {
		e.DisplacementCC = n.DisplacementCC
	}
	if n.Turbo != nil && n.Aspiration == nil {
		e.Aspiration, e.Turbo = nil, n.Turbo
	}
	e.Energy, e.Aspiration, e.Transmission = pick(e.Energy, n.Energy), pick(e.Aspiration, n.Aspiration), pick(e.Transmission, n.Transmission)
	e.Drive, e.Cycle, e.Cooling = pick(e.Drive, n.Drive), pick(e.Cycle, n.Cycle), pick(e.Cooling, n.Cooling)
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
