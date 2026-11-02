// Package domain contiene los repuestos y sus movimientos (HU-32 a HU-38). Sin I/O.
package domain

import (
	"strings"

	"sweveh/internal/shared"
)

var (
	ErrNotFound  = shared.NotFound("repuesto_no_encontrado", "No encontramos ese repuesto en tu taller.")
	ErrRef       = shared.Validation("repuesto_referencia", "La referencia usa letras, números y guiones (máx. 40).").WithField("referencia")
	ErrRefTaken  = shared.Conflict("repuesto_referencia_repetida", "Ya tienes un repuesto con esa referencia.").WithField("referencia")
	ErrName      = shared.Validation("repuesto_nombre", "El repuesto necesita un nombre de hasta 120 letras.").WithField("nombre")
	ErrMoney     = shared.Validation("repuesto_valor", "Costo y precio van en pesos, sin decimales y no negativos.")
	ErrQty       = shared.Validation("repuesto_cantidad", "La cantidad debe ser un número entero mayor que cero.").WithField("cantidad")
	ErrKind      = shared.Validation("repuesto_tipo", "Indica si es de carro, de moto o de ambos.").WithField("tipoVehiculo")
)

// Part es un repuesto del inventario. Cost solo lo ve quien tiene permiso (RF-21).
type Part struct {
	ID        shared.ID
	TallerID  shared.ID
	Ref       string
	Name      string
	Kind      string
	Cost      int64
	Price     int64
	Stock     int
	Min       int
	Active    bool
	Version   int
}

// LowStock indica si está en el mínimo o por debajo (HU-36).
func (p Part) LowStock() bool { return p.Active && p.Stock <= p.Min }

// Normalize limpia y valida.
func (p *Part) Normalize() error {
	p.Ref = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(p.Ref), " ", "-"))
	p.Name = strings.Join(strings.Fields(p.Name), " ")
	if p.Ref == "" || len(p.Ref) > 40 || strings.Trim(p.Ref, "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-") != "" || p.Ref[0] == '-' {
		return ErrRef
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
