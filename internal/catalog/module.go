// Package catalog es la fachada del catálogo de servicios del taller.
package catalog

import (
	"context"

	"sweveh/internal/catalog/app"
	"sweveh/internal/shared"
)

// Item es lo que otros módulos (cotización, agente) ven de un servicio.
type Item struct {
	ID          shared.ID
	Name        string
	Kind        string
	Price       *int64 // nil = sin precio: "el taller te confirma el valor"
	DurationMin *int
	Active      bool
}

type Catalog struct{ svc *app.Service }

func New(svc *app.Service) Catalog { return Catalog{svc: svc} }

func (c Catalog) Get(ctx context.Context, tallerID, id shared.ID) (Item, error) {
	s, err := c.svc.Get(ctx, tallerID, id)
	return Item{ID: s.ID, Name: s.Name, Kind: s.Kind, Price: s.Price, DurationMin: s.DurationMin, Active: s.Active}, err
}

func (c Catalog) Active(ctx context.Context, tallerID shared.ID) ([]Item, error) {
	list, err := c.svc.List(ctx, tallerID, false)
	out := make([]Item, 0, len(list))
	for _, s := range list {
		out = append(out, Item{ID: s.ID, Name: s.Name, Kind: s.Kind, Price: s.Price, DurationMin: s.DurationMin, Active: true})
	}
	return out, err
}

// Preload carga los servicios iniciales (HU-04) dentro de la transacción en curso.
func (c Catalog) Preload(ctx context.Context, tallerID shared.ID, attends string, specialties []string) error {
	return c.svc.Preload(ctx, tallerID, attends, specialties)
}
