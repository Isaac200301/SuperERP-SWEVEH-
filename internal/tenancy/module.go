// Package tenancy es la fachada del módulo de talleres. Empieza pequeño (docs/06 §3):
// crear el taller al registrarse y leer su nombre. Horario, plan y suscripción llegan con sus HU.
package tenancy

import (
	"context"
	"time"

	"sweveh/internal/audit"
	"sweveh/internal/shared"
	"sweveh/internal/tenancy/domain"
)

// Repository es el puerto de persistencia del taller.
type Repository interface {
	Insert(ctx context.Context, w domain.Workshop) error
	ByID(ctx context.Context, id shared.ID) (domain.Workshop, error)
}

// AuditLog registra el cambio en la misma transacción.
type AuditLog interface {
	Record(ctx context.Context, e audit.Entry) error
}

// Workshop es lo que otros módulos ven del taller.
type Workshop struct {
	ID   shared.ID
	Name string
}

// Workshops es la fachada pública.
type Workshops struct {
	repo  Repository
	audit AuditLog
}

func NewWorkshops(repo Repository, a AuditLog) Workshops { return Workshops{repo: repo, audit: a} }

// Create crea el taller. Debe correr dentro de una transacción con app.taller_id = id
// (la abre identity al registrar al dueño).
func (w Workshops) Create(ctx context.Context, id shared.ID, name string, actor shared.Actor, now time.Time) (Workshop, error) {
	ws, err := domain.NewWorkshop(id, name, now)
	if err != nil {
		return Workshop{}, err
	}
	if err := w.repo.Insert(ctx, ws); err != nil {
		return Workshop{}, err
	}
	if err := w.audit.Record(ctx, audit.Entry{
		TallerID: id, Actor: actor, Action: "taller.crear", Entity: "taller", EntityID: id,
		After: map[string]any{"nombre": ws.Name}, OccurredAt: now,
	}); err != nil {
		return Workshop{}, err
	}
	return Workshop{ID: ws.ID, Name: ws.Name}, nil
}

// Get lee el taller (dentro de una transacción del mismo taller).
func (w Workshops) Get(ctx context.Context, id shared.ID) (Workshop, error) {
	ws, err := w.repo.ByID(ctx, id)
	if err != nil {
		return Workshop{}, err
	}
	return Workshop{ID: ws.ID, Name: ws.Name}, nil
}
