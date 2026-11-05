package app

import (
	"context"
	"time"

	"sweveh/internal/orders/domain"
	"sweveh/internal/shared"
)

// Since dice desde cuándo una orden está en su estado actual.
type Since struct {
	OrderID shared.ID
	At      time.Time
}

// Filter acota las cifras del tablero: Kind "" (todos), "carro" o "moto" (HU-68, HU-70); Example=true suma también
// las órdenes de ejemplo (registro, HU-05, y taller ficticio, HU-97), que el tablero marca "ejemplo" (HU-71).
type Filter struct {
	Kind    string
	Example bool
}

// Cycle es una orden entregada: cuándo se abrió y cuándo se cerró (HU-70).
type Cycle struct{ Opened, Closed time.Time }

// Stats son las cifras de órdenes que leen otros módulos (KPIs, avisos), siempre dentro de su transacción.
type Stats interface {
	StateCounts(ctx context.Context, tallerID shared.ID, f Filter) (map[string]int, error)
	Cycles(ctx context.Context, tallerID shared.ID, since time.Time, f Filter) ([]Cycle, error)
	Returning(ctx context.Context, tallerID shared.ID, since time.Time, f Filter) (int, int, error)
	DeliveredSince(ctx context.Context, tallerID shared.ID, since time.Time, f Filter) ([]shared.ID, error)
	StateSince(ctx context.Context, tallerID shared.ID, state string) ([]Since, error)
	// HasExample dice si el taller tiene órdenes de ejemplo (sus cifras van marcadas "ejemplo", HU-71).
	HasExample(ctx context.Context, tallerID shared.ID) (bool, error)
}

func (f *Flow) SetStats(s Stats) { f.stats = s }
func (f *Flow) Stats() Stats     { return f.stats }

// Info es la orden con lo que llenan los avisos: checklist y desde cuándo está en su estado.
type Info struct {
	Order       domain.Order
	Vehicle     string
	Km          *int
	Exceptions  []string
	Accessories []string
	StateSince  time.Time
}

// Info se llama dentro de la transacción del llamador.
func (f *Flow) Info(ctx context.Context, tallerID, orderID shared.ID) (Info, error) {
	o, err := f.repo.Load(ctx, tallerID, orderID)
	if err != nil {
		return Info{}, err
	}
	return f.info(ctx, o)
}

func (f *Flow) info(ctx context.Context, o domain.Order) (Info, error) {
	out := Info{Order: o, StateSince: o.CreatedAt, Vehicle: "vehículo"}
	if v, err := f.vehicles.ByID(ctx, o.TallerID, o.VehicleID); err == nil {
		out.Vehicle = describe(v.Brand, v.Line, v.Kind)
	}
	cl, ok, err := f.repo.GetChecklist(ctx, o.TallerID, o.ID)
	if err != nil {
		return out, err
	}
	if ok {
		out.Km, out.Accessories = cl.Km, cl.Accessories
		for _, e := range cl.Exceptions {
			out.Exceptions = append(out.Exceptions, e.Note)
		}
	}
	if f.stats != nil {
		ss, err := f.stats.StateSince(ctx, o.TallerID, string(o.State))
		if err != nil {
			return out, err
		}
		for _, s := range ss {
			if s.OrderID == o.ID {
				out.StateSince = s.At
			}
		}
	}
	return out, nil
}

// InState lista las órdenes de un estado con su información (AU-06, AU-07).
func (f *Flow) InState(ctx context.Context, tallerID shared.ID, state string) ([]Info, error) {
	st := domain.State(state)
	list, err := f.repo.List(ctx, portsFilter(tallerID, &st))
	if err != nil {
		return nil, err
	}
	out := make([]Info, 0, len(list))
	for _, o := range list {
		i, err := f.info(ctx, o)
		if err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, nil
}

func describe(brand, line *string, kind string) string {
	s := ""
	if brand != nil {
		s = *brand
	}
	if line != nil {
		if s != "" {
			s += " "
		}
		s += *line
	}
	if s == "" {
		if kind == "moto" {
			return "moto"
		}
		return "carro"
	}
	return s
}
