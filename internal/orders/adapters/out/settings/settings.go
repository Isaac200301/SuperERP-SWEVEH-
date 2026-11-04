// Package settings adapta los parámetros de tenancy al puerto SettingsReader de orders.
// Solo usa la fachada de tenancy (arch_test.go): los ítems se copian campo por campo.
package settings

import (
	"context"

	"sweveh/internal/orders/domain"
	"sweveh/internal/orders/ports"
	"sweveh/internal/shared"
	"sweveh/internal/tenancy"
)

type Reader struct{ A *tenancy.Admin }

func (r Reader) Settings(ctx context.Context, tallerID shared.ID) (ports.Settings, error) {
	p, err := r.A.ParametrosIn(ctx, tallerID)
	if err != nil {
		return ports.Settings{}, err
	}
	out := ports.Settings{Reception: domain.ReceptionRules{MinPhotos: p.MinPhotos, MileageRequired: p.MileageRequired},
		UseAwaitingParts: p.UseAwaitingParts, UseQualityCheck: p.UseQualityCheck}
	for _, it := range p.CarItems {
		out.CarItems = append(out.CarItems, domain.ItemCheck{Key: it.Key, Name: it.Name})
	}
	for _, it := range p.MotoItems {
		out.MotoItems = append(out.MotoItems, domain.ItemCheck{Key: it.Key, Name: it.Name})
	}
	return out, nil
}
