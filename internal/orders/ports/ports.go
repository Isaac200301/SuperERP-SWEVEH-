// Package ports declara lo que los casos de uso de orders necesitan del exterior.
package ports

import (
	"context"
	"time"

	"sweveh/internal/audit"
	"sweveh/internal/orders/domain"
	"sweveh/internal/shared"
)

// OrderRepository guarda y lee órdenes del taller actual.
type OrderRepository interface {
	// NextNumber reserva el siguiente consecutivo del taller (bloquea su contador hasta el commit).
	NextNumber(ctx context.Context, tallerID shared.ID) (int, error)
	CountOpenByVehicle(ctx context.Context, tallerID, vehicleID shared.ID) (int, error)
	Insert(ctx context.Context, o domain.Order) error
	InsertHistory(ctx context.Context, o domain.Order, from *domain.State, by shared.ID, at time.Time) error
	ByID(ctx context.Context, tallerID, id shared.ID) (domain.Order, error)
	Recent(ctx context.Context, tallerID shared.ID, limit int, cursor *Cursor) ([]domain.Order, error)
	History(ctx context.Context, tallerID, id shared.ID) ([]HistoryEntry, error)
}

// Cursor para paginar por (created_at, id).
type Cursor struct {
	CreatedAt time.Time
	ID        shared.ID
}

// HistoryEntry es un paso del historial de estados.
type HistoryEntry struct {
	From       *domain.State
	To         domain.State
	UserID     *shared.ID
	OccurredAt time.Time
}

// Vehicle es lo que orders necesita de vehicles.
type Vehicle struct {
	ID            shared.ID
	CustomerID    shared.ID
	Plate         string
	Kind          string
	Brand         *string
	Line          *string
	Year          *int
	EnginePending bool
}

// VehicleDirectory es la fachada de vehicles vista desde orders.
type VehicleDirectory interface {
	ByID(ctx context.Context, tallerID, id shared.ID) (Vehicle, error)
	ByPlate(ctx context.Context, tallerID shared.ID, plate string) (Vehicle, error)
	ByIDs(ctx context.Context, tallerID shared.ID, ids []shared.ID) (map[shared.ID]Vehicle, error)
}

// Customer es lo que orders necesita de customers.
type Customer struct {
	ID   shared.ID
	Name string
}

// CustomerDirectory es la fachada de customers vista desde orders.
type CustomerDirectory interface {
	ByIDs(ctx context.Context, tallerID shared.ID, ids []shared.ID) (map[shared.ID]Customer, error)
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
