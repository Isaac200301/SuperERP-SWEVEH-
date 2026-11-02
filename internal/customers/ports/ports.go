// Package ports declara lo que los casos de uso de customers necesitan del exterior.
package ports

import (
	"context"
	"time"

	"sweveh/internal/audit"
	"sweveh/internal/customers/domain"
	"sweveh/internal/shared"
)

// CustomerRepository guarda y lee clientes del taller actual.
type CustomerRepository interface {
	ByPhone(ctx context.Context, tallerID shared.ID, phone string) (domain.Customer, error)
	ByID(ctx context.Context, tallerID, id shared.ID) (domain.Customer, error)
	ByIDs(ctx context.Context, tallerID shared.ID, ids []shared.ID) ([]domain.Customer, error)
	// Insert devuelve false si ya existía un cliente con ese celular en el taller.
	Insert(ctx context.Context, c domain.Customer, createdBy shared.ID) (bool, error)
}

// UnitOfWork abre una transacción con el taller del context fijado (RLS).
type UnitOfWork interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

// AuditLog registra el cambio en la misma transacción.
type AuditLog interface {
	Record(ctx context.Context, e audit.Entry) error
}

// EventPublisher escribe el evento en el outbox en la misma transacción.
type EventPublisher interface {
	Publish(ctx context.Context, tallerID shared.ID, e shared.Event) error
}

// Clock da la hora.
type Clock interface{ Now() time.Time }
