package ports

import (
	"context"

	"sweveh/internal/orders/domain"
	"sweveh/internal/shared"
)

// Settings son los parámetros del taller que usa la orden (recepción y estados opcionales).
type Settings struct {
	Reception        domain.ReceptionRules
	CarItems         []domain.ItemCheck
	MotoItems        []domain.ItemCheck
	UseAwaitingParts bool
	UseQualityCheck  bool
}

// ItemsFor devuelve la lista vigente de ítems a revisar según el tipo de vehículo.
func (s Settings) ItemsFor(kind string) []domain.ItemCheck {
	src := s.CarItems
	if kind == "moto" {
		src = s.MotoItems
	}
	return append([]domain.ItemCheck{}, src...)
}

// SettingsReader lee los parámetros dentro de la transacción en curso (fachada de tenancy).
type SettingsReader interface {
	Settings(ctx context.Context, tallerID shared.ID) (Settings, error)
}

// DefaultSettings se usa si el composition root no conectó un lector (pruebas de unidad).
var DefaultSettings = Settings{Reception: domain.DefaultReceptionRules}
