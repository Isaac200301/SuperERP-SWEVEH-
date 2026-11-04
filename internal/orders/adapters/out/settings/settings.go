// Package settings adapta los parámetros de tenancy al puerto SettingsReader de orders.
package settings

import (
	"context"

	"sweveh/internal/orders/domain"
	"sweveh/internal/orders/ports"
	"sweveh/internal/shared"
	"sweveh/internal/tenancy"
	tenancydomain "sweveh/internal/tenancy/domain"
)

type Reader struct{ A *tenancy.Admin }

func items(in []tenancydomain.ChecklistItem) []domain.ItemCheck {
	out := make([]domain.ItemCheck, 0, len(in))
	for _, it := range in {
		out = append(out, domain.ItemCheck{Key: it.Key, Name: it.Name})
	}
	return out
}

func (r Reader) Settings(ctx context.Context, tallerID shared.ID) (ports.Settings, error) {
	p, err := r.A.ParametrosIn(ctx, tallerID)
	if err != nil {
		return ports.Settings{}, err
	}
	return ports.Settings{Reception: domain.ReceptionRules{MinPhotos: p.MinPhotos, MileageRequired: p.MileageRequired},
		CarItems: items(p.CarItems), MotoItems: items(p.MotoItems), UseAwaitingParts: p.UseAwaitingParts, UseQualityCheck: p.UseQualityCheck}, nil
}
