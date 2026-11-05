// Package inventory es la fachada del inventario.
package inventory

import (
	"context"

	"sweveh/internal/inventory/app"
	"sweveh/internal/shared"
)

// Part es lo que otros módulos ven de un repuesto: nunca el costo.
type Part struct {
	ID     shared.ID
	Ref    string
	Name   string
	Price  int64
	Stock  int
	Active bool
}

// Line es una línea aprobada que consume inventario.
type Line = app.LineToDiscount

type Inventory struct{ svc *app.Service }

func New(svc *app.Service) Inventory { return Inventory{svc: svc} }

// Parts lee repuestos dentro de la transacción del llamador.
func (i Inventory) Parts(ctx context.Context, tallerID shared.ID, ids []shared.ID) (map[shared.ID]Part, error) {
	list, err := i.svc.GetMany(ctx, tallerID, ids)
	out := make(map[shared.ID]Part, len(list))
	for _, p := range list {
		out[p.ID] = Part{ID: p.ID, Ref: p.Ref, Name: p.Name, Price: p.Price, Stock: p.Stock, Active: p.Active}
	}
	return out, err
}

// Discount descuenta repuestos de una orden aprobada (HU-33) en la transacción del llamador.
func (i Inventory) Discount(ctx context.Context, tallerID, orderID shared.ID, lines []Line, by shared.Actor) ([]shared.ID, error) {
	return i.svc.Discount(ctx, tallerID, orderID, lines, by)
}

// ReturnAll devuelve a bodega lo que la orden descontó y no se ha devuelto (HU-26, al cancelar).
func (i Inventory) ReturnAll(ctx context.Context, tallerID, orderID shared.ID, by shared.Actor) (int, error) {
	return i.svc.ReturnAll(ctx, by, tallerID, orderID)
}
