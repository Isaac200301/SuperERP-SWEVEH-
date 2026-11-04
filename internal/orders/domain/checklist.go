package domain

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"sweveh/internal/shared"
)

// Lados de foto por tipo (HU-17 carro: los 4 lados; HU-18 moto: 5 posibles, mínimo 4).
var (
	CarSides  = []string{"frente", "trasera", "izquierdo", "derecho"}
	MotoSides = []string{"frente", "trasera", "izquierdo", "derecho", "tablero"}
	Fuels     = []string{"vacio", "1/4", "1/2", "3/4", "lleno", "no_aplica"}
	// Accesorios entregados en moto (HU-18); en carro se anotan como objetos.
	MotoAccessories = []string{"casco", "baul", "maletas", "documentos"}
)

// MinPhotos es el mínimo por defecto; cada taller lo ajusta en sus parámetros (ReceptionRules).
const MinPhotos = 4

// ReceptionRules son los parámetros del taller que cambian qué se exige al recibir (decisión de Pablo, 3-nov-2026).
type ReceptionRules struct {
	MinPhotos       int
	MileageRequired bool
}

// DefaultReceptionRules: 4 fotos y kilometraje obligatorio, como antes de los parámetros.
var DefaultReceptionRules = ReceptionRules{MinPhotos: MinPhotos, MileageRequired: true}

// Estados de un ítem revisado al recibir. "" = todavía no se revisó.
var ItemStates = []string{"bien", "mal", "no_tiene"}

// ItemCheck es un ítem del checklist del taller (luces, gato, documentos…) con lo que se encontró.
// El checklist guarda la copia del ítem: si el taller lo renombra después, la recepción hecha no cambia.
type ItemCheck struct {
	Key   string `json:"clave"`
	Name  string `json:"nombre"`
	State string `json:"estado"`
	Note  string `json:"nota,omitempty"`
}

var (
	ErrFuel      = shared.Validation("combustible_invalido", "Elige el nivel de combustible.").WithField("combustible")
	ErrFuelNA    = shared.Validation("combustible_no_aplica", "\"No aplica\" solo es para motos sin indicador.").WithField("combustible")
	ErrKm        = shared.Validation("kilometraje_invalido", "El kilometraje no puede ser negativo.").WithField("kilometraje")
	ErrException = shared.Validation("salvedad_invalida", "La salvedad necesita una descripción de hasta 300 letras.")
	ErrItemState = shared.Validation("item_invalido", "Cada ítem se marca como bien, mal o no tiene; la nota va hasta 300 letras.").WithField("items")
)

// Exception es una salvedad del recibo: rayón, golpe, objeto (HU-17, criterio 3).
type Exception struct {
	Note    string     `json:"nota"`
	PhotoID *shared.ID `json:"fotoId,omitempty"`
	At      time.Time  `json:"hora"`
}

// Checklist es la recepción del vehículo.
type Checklist struct {
	OrderID     shared.ID
	Kind        string
	Km          *int
	Fuel        *string
	Accessories []string
	Exceptions  []Exception
	Items       []ItemCheck
	DoneBy      *shared.ID
	DoneAt      *time.Time
	Version     int
}

// Validate revisa los valores escritos.
func (c *Checklist) Validate() error {
	if c.Km != nil && *c.Km < 0 {
		return ErrKm
	}
	if c.Fuel != nil {
		if !slices.Contains(Fuels, *c.Fuel) {
			return ErrFuel
		}
		if *c.Fuel == "no_aplica" && c.Kind != "moto" {
			return ErrFuelNA
		}
	}
	clean := c.Accessories[:0]
	for _, a := range c.Accessories {
		if a = strings.TrimSpace(strings.ToLower(a)); a != "" && len(a) <= 60 && !slices.Contains(clean, a) {
			clean = append(clean, a)
		}
	}
	c.Accessories = clean
	for i := range c.Items {
		c.Items[i].Note = strings.TrimSpace(c.Items[i].Note)
		if (c.Items[i].State != "" && !slices.Contains(ItemStates, c.Items[i].State)) || len([]rune(c.Items[i].Note)) > 300 {
			return ErrItemState
		}
	}
	for i := range c.Exceptions {
		n := strings.TrimSpace(c.Exceptions[i].Note)
		if n == "" || len([]rune(n)) > 300 {
			return ErrException
		}
		c.Exceptions[i].Note = n
	}
	return nil
}

// Missing dice qué falta para pasar a Recibido, en palabras del taller (DR-06). sides = lados con foto.
// Las fotos y el kilometraje siguen los parámetros del taller; los ítems revisados no bloquean.
func (c Checklist) Missing(sides []string, rules ReceptionRules) []string {
	var miss []string
	required := CarSides
	if c.Kind == "moto" {
		required = MotoSides
	}
	have := 0
	var missingSides []string
	for _, s := range required {
		if slices.Contains(sides, s) {
			have++
		} else {
			missingSides = append(missingSides, s)
		}
	}
	need := min(rules.MinPhotos, len(required))
	switch {
	case have >= need:
	case need == len(required):
		miss = append(miss, "fotos: "+strings.Join(missingSides, ", "))
	default:
		miss = append(miss, "fotos (mínimo "+strconv.Itoa(need)+" de: "+strings.Join(missingSides, ", ")+")")
	}
	if c.Km == nil && rules.MileageRequired {
		miss = append(miss, "kilometraje")
	}
	if c.Fuel == nil {
		miss = append(miss, "combustible")
	}
	return miss
}
