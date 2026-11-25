// Package quotesdir adapta la fachada de quotes al puerto QuoteReader de orders.
package quotesdir

import (
	"context"

	"sweveh/internal/orders/ports"
	"sweveh/internal/quotes"
	"sweveh/internal/shared"
)

type Reader struct{ Q quotes.Quotes }

// Quoted es lo cotizado de la orden (V-08): lo aprobado, sumando las adicionales (HU-31); si aún no hay nada aprobado,
// el total de la cotización vigente.
func (r Reader) Quoted(ctx context.Context, tallerID, orderID shared.ID) (int64, error) {
	approved, err := r.Q.ApprovedTotals(ctx, tallerID, []shared.ID{orderID})
	if err != nil {
		return 0, err
	}
	if v := approved[orderID]; v > 0 {
		return v, nil
	}
	s, err := r.Q.ForOrder(ctx, tallerID, orderID)
	if err != nil || s == nil {
		return 0, err
	}
	return s.Total, nil
}

func (r Reader) ForOrder(ctx context.Context, tallerID, orderID shared.ID) (*ports.QuoteSummary, error) {
	s, err := r.Q.ForOrder(ctx, tallerID, orderID)
	if err != nil || s == nil {
		return nil, err
	}
	out := &ports.QuoteSummary{State: s.State, Total: s.Total}
	for _, l := range s.Lines {
		out.Lines = append(out.Lines, ports.QuoteLine{Kind: l.Kind, Description: l.Description, Qty: l.Qty, UnitPrice: l.UnitPrice})
	}
	return out, nil
}
