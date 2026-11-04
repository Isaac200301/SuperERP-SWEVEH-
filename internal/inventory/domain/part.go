// Package domain contiene los repuestos y sus movimientos (HU-32 a HU-38). Sin I/O.
package domain

import (
	"slices"
	"strings"
	"time"

	"sweveh/internal/shared"
)

var (
	ErrNotFound = shared.NotFound("repuesto_no_encontrado", "No encontramos ese repuesto en tu taller.")
	ErrRef      = shared.Validation("repuesto_referencia", "La referencia usa letras, números y guiones (máx. 40).").WithField("referencia")
	ErrRefTaken = shared.Conflict("repuesto_referencia_repetida", "Ya tienes un repuesto con esa referencia.").WithField("referencia")
	ErrName     = shared.Validation("repuesto_nombre", "El repuesto necesita un nombre de hasta 120 letras.").WithField("nombre")
	ErrMoney    = shared.Validation("repuesto_valor", "Costo y precio van en pesos, sin decimales y no negativos.")
	ErrQty      = shared.Validation("repuesto_cantidad", "La cantidad debe ser un número entero mayor que cero.").WithField("cantidad")
	ErrKind     = shared.Validation("repuesto_tipo", "Indica si es de carro, de moto o de ambos.").WithField("tipoVehiculo")
	ErrCategory = shared.Validation("repuesto_categoria", "Escoge una categoría de la lista.").WithField("categoria")
	ErrBrand    = shared.Validation("repuesto_marca", "La marca puede tener hasta 60 letras.").WithField("marca")
	ErrLocation = shared.Validation("repuesto_ubicacion", "La ubicación puede tener hasta 40 letras, por ejemplo Estante A3.").WithField("ubicacion")
	ErrOEM      = shared.Validation("repuesto_oem", "La referencia original usa letras, números y guiones (máx. 40).").WithField("referenciaOem")
)

// Categories coinciden con los sistemas del diagnóstico (orders/domain.Systems) más filtros y lubricantes,
// así un hallazgo de frenos filtra los repuestos de frenos.
var Categories = []string{"motor", "frenos", "suspension", "direccion", "electrico", "transmision", "refrigeracion", "llantas",
	"carroceria", "filtros", "lubricantes", "otro"}

// Part es un repuesto del inventario. Cost solo lo ve quien tiene permiso (RF-21).
type Part struct {
	ID       shared.ID
	TallerID shared.ID
	Ref      string
	Name     string
	Kind     string
	Cost     int64
	Price    int64
	Stock    int
	Min      int
	Active   bool
	Version  int

	Brand    string // "" = sin dato
	Category string
	Location string
	OEM      string // referencia original o equivalente, para buscar al cotizar
}

// LowStock indica si está en el mínimo o por debajo (HU-36).
func (p Part) LowStock() bool { return p.Active && p.Stock <= p.Min }

// Normalize limpia y valida.
func (p *Part) Normalize() error {
	p.Ref = normRef(p.Ref)
	p.Name = strings.Join(strings.Fields(p.Name), " ")
	if !validRef(p.Ref) {
		return ErrRef
	}
	if err := p.normalizeSheet(); err != nil {
		return err
	}
	if p.Name == "" || len([]rune(p.Name)) > 120 {
		return ErrName
	}
	if p.Kind == "" {
		p.Kind = "ambos"
	}
	if p.Kind != "carro" && p.Kind != "moto" && p.Kind != "ambos" {
		return ErrKind
	}
	if p.Cost < 0 || p.Price < 0 || p.Min < 0 || p.Stock < 0 {
		return ErrMoney
	}
	return nil
}

func normRef(s string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(s), " ", "-"))
}

func validRef(s string) bool {
	return s != "" && len(s) <= 40 && strings.Trim(s, "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-") == "" && s[0] != '-'
}

// normalizeSheet valida marca, categoría, ubicación y referencia original (todas opcionales).
func (p *Part) normalizeSheet() error {
	p.Brand = strings.Join(strings.Fields(p.Brand), " ")
	p.Location = strings.Join(strings.Fields(p.Location), " ")
	p.Category = strings.ToLower(strings.TrimSpace(p.Category))
	p.OEM = normRef(p.OEM)
	switch {
	case len([]rune(p.Brand)) > 60:
		return ErrBrand
	case len([]rune(p.Location)) > 40:
		return ErrLocation
	case p.Category != "" && !slices.Contains(Categories, p.Category):
		return ErrCategory
	case p.OEM != "" && !validRef(p.OEM):
		return ErrOEM
	}
	return nil
}

// OrderPart resume lo que una orden movió de un repuesto (HU-33, HU-38).
type OrderPart struct {
	PartID     shared.ID
	Ref, Name  string
	Discounted int // salió de bodega al aprobar
	Returned   int // volvió a bodega
	Pending    int // aprobado sin existencia: no se descontó (Q-11)
}

// ToReturn es lo que todavía se puede devolver.
func (o OrderPart) ToReturn() int { return o.Discounted - o.Returned }

// Movement es una línea del libro de inventario de un repuesto (HU-35, HU-38).
type Movement struct {
	ID        shared.ID
	Qty       int    // positivo entra, negativo sale
	Reason    string // entrada | salida_orden | pendiente_orden | devolucion | ajuste | importacion
	OrderID   *shared.ID
	UserID    *shared.ID
	CreatedAt time.Time
}
