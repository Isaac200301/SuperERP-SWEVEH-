// Package ports declara lo que los casos de uso de vehicles necesitan del exterior.
package ports

import (
	"context"
	"time"

	"sweveh/internal/audit"
	"sweveh/internal/shared"
	"sweveh/internal/vehicles/domain"
)

// VehicleRepository guarda y lee vehículos del taller actual.
type VehicleRepository interface {
	ByPlate(ctx context.Context, tallerID shared.ID, plate string) (domain.Vehicle, error)
	ByID(ctx context.Context, tallerID, id shared.ID) (domain.Vehicle, error)
	ByIDs(ctx context.Context, tallerID shared.ID, ids []shared.ID) ([]domain.Vehicle, error)
	// Insert devuelve false si la placa ya existía en el taller.
	Insert(ctx context.Context, v domain.Vehicle, createdBy shared.ID) (bool, error)
	// UpdateEngine falla con shared.ErrVersionConflict si la versión cambió.
	UpdateEngine(ctx context.Context, v domain.Vehicle, prevVersion int, at time.Time) error
}

// CustomerChecker confirma que el cliente existe en el taller (fachada de customers).
type CustomerChecker interface {
	Exists(ctx context.Context, tallerID, id shared.ID) (bool, error)
}

type UnitOfWork interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

type AuditLog interface {
	Record(ctx context.Context, e audit.Entry) error
}

type EventPublisher interface {
	Publish(ctx context.Context, tallerID shared.ID, e shared.Event) error
}

type Clock interface{ Now() time.Time }
