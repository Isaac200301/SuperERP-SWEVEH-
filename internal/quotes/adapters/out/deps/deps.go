// Package deps adapta las fachadas de orders, catalog e inventory a los puertos de quotes.
package deps

import (
	"context"

	"sweveh/internal/catalog"
	"sweveh/internal/inventory"
	"sweveh/internal/orders"
	"sweveh/internal/quotes/app"
	"sweveh/internal/quotes/domain"
	"sweveh/internal/shared"
	"sweveh/internal/tenancy"
)

// Rules lee los parámetros de cotización del taller desde la fachada de tenancy.
type Rules struct{ A *tenancy.Admin }

func (r Rules) QuoteRules(ctx context.Context, tallerID shared.ID) (domain.Rules, error) {
	p, err := r.A.ParametrosIn(ctx, tallerID)
	return domain.Rules{VATPct: p.VATPct, PricesIncludeVAT: p.PricesIncludeVAT, HourlyRate: p.HourlyRate, MaxDiscountPct: p.MaxDiscountPct,
		ReceptionDiscount: p.ReceptionDiscount, ValidityDays: p.ValidityDays}, err
}

type Orders struct{ O orders.Orders }

func (a Orders) Load(ctx context.Context, tallerID, id shared.ID) (app.Order, error) {
	s, err := a.O.Load(ctx, tallerID, id)
	return app.Order{ID: s.ID, State: s.State, CustomerID: s.CustomerID, Plate: s.Plate, Number: s.Number}, err
}

func (a Orders) Move(ctx context.Context, tallerID, id shared.ID, to string, actor shared.Actor, reason string) error {
	return a.O.Move(ctx, tallerID, id, to, actor, reason)
}

type Catalog struct{ C catalog.Catalog }

func (a Catalog) Get(ctx context.Context, tallerID, id shared.ID) (app.Service, error) {
	s, err := a.C.Get(ctx, tallerID, id)
	return app.Service{Name: s.Name, Price: s.Price}, err
}

func (a Catalog) FillPrice(ctx context.Context, actor shared.Actor, tallerID, id shared.ID, price int64) error {
	return a.C.FillPrice(ctx, actor, tallerID, id, price)
}

type Inventory struct{ I inventory.Inventory }

func (a Inventory) Parts(ctx context.Context, tallerID shared.ID, ids []shared.ID) (map[shared.ID]app.Part, error) {
	m, err := a.I.Parts(ctx, tallerID, ids)
	out := make(map[shared.ID]app.Part, len(m))
	for k, p := range m {
		out[k] = app.Part{Name: p.Name, Ref: p.Ref, Price: p.Price, Stock: p.Stock, Active: p.Active}
	}
	return out, err
}

func (a Inventory) Discount(ctx context.Context, tallerID, orderID shared.ID, lines []app.DiscountLine, by shared.Actor) ([]shared.ID, error) {
	ls := make([]inventory.Line, 0, len(lines))
	for _, l := range lines {
		ls = append(ls, inventory.Line{LineID: l.LineID, PartID: l.PartID, Qty: l.Qty})
	}
	return a.I.Discount(ctx, tallerID, orderID, ls, by)
}
