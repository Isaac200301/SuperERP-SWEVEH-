package app

import (
	"context"
	"time"

	"sweveh/internal/audit"
	"sweveh/internal/shared"
	"sweveh/internal/vehicles/domain"
)

// Extra son las lecturas y cambios que se agregaron con HU-12 y HU-14.
type Extra interface {
	SearchPlate(ctx context.Context, tallerID shared.ID, q string, limit int) ([]domain.Vehicle, error)
	ByCustomer(ctx context.Context, tallerID, customerID shared.ID) ([]domain.Vehicle, error)
	ChangeOwner(ctx context.Context, v domain.Vehicle, prevOwner, newOwner shared.ID, by *shared.ID, at time.Time) error
}

func (s *Service) SetExtra(e Extra) { s.extra = e }

func (s *Service) SearchPlate(ctx context.Context, tallerID shared.ID, q string) ([]domain.Vehicle, error) {
	var out []domain.Vehicle
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.extra.SearchPlate(ctx, tallerID, domain.NormalizePlate(q), 20)
		return err
	})
	return out, err
}

// ByCustomer lista los vehículos de un cliente (HU-12, criterio 1).
func (s *Service) ByCustomer(ctx context.Context, tallerID, customerID shared.ID) ([]domain.Vehicle, error) {
	var out []domain.Vehicle
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.extra.ByCustomer(ctx, tallerID, customerID)
		return err
	})
	return out, err
}

// ChangeOwner asigna el vehículo a otro cliente conservando el historial y la fecha del cambio (HU-12, criterio 2).
func (s *Service) ChangeOwner(ctx context.Context, tallerID, vehicleID, newOwner shared.ID, actor shared.Actor, version int) (domain.Vehicle, error) {
	var out domain.Vehicle
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		v, err := s.repo.ByID(ctx, tallerID, vehicleID)
		if err != nil {
			return err
		}
		if v.Version != version {
			return shared.ErrVersionConflict
		}
		ok, err := s.customers.Exists(ctx, tallerID, newOwner)
		if err != nil {
			return err
		}
		if !ok {
			return ErrCustomerRequired
		}
		if newOwner == v.CustomerID {
			out = v
			return nil
		}
		prev := v.CustomerID
		v.CustomerID = newOwner
		v.Version++
		now := s.clock.Now()
		var by *shared.ID
		if actor.ID != shared.NilID {
			by = &actor.ID
		}
		if err := s.extra.ChangeOwner(ctx, v, prev, newOwner, by, now); err != nil {
			return err
		}
		if err := s.audit.Record(ctx, audit.Entry{TallerID: tallerID, Actor: actor, Action: "vehiculo.cambio_dueno", Entity: "vehiculo",
			EntityID: v.ID, Before: map[string]any{"cliente_id": prev.String()}, After: map[string]any{"cliente_id": newOwner.String()},
			OccurredAt: now}); err != nil {
			return err
		}
		out = v
		return s.events.Publish(ctx, tallerID, shared.Event{Type: "vehicles.cambio_dueno", AggregateID: v.ID, OccurredAt: now,
			Payload: map[string]any{"anterior": prev.String(), "nuevo": newOwner.String()}})
	})
	return out, err
}
