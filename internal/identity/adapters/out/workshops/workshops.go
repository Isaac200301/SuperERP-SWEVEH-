// Package workshops adapta la fachada de tenancy al puerto Workshops de identity.
package workshops

import (
	"context"
	"time"

	"sweveh/internal/identity/ports"
	"sweveh/internal/shared"
	"sweveh/internal/tenancy"
)

type Adapter struct{ w tenancy.Workshops }

func New(w tenancy.Workshops) Adapter { return Adapter{w: w} }

func (a Adapter) Create(ctx context.Context, id shared.ID, name string, actor shared.Actor, now time.Time) (ports.Workshop, error) {
	ws, err := a.w.Create(ctx, id, name, actor, now)
	return ports.Workshop{ID: ws.ID, Name: ws.Name}, err
}

func (a Adapter) Get(ctx context.Context, id shared.ID) (ports.Workshop, error) {
	ws, err := a.w.Get(ctx, id)
	return ports.Workshop{ID: ws.ID, Name: ws.Name}, err
}
