// Package quotesdir adapta la fachada de quotes al puerto QuoteReader de orders.
package quotesdir

import (
	"context"

	"sweveh/internal/orders/ports"
	"sweveh/internal/quotes"
	"sweveh/internal/shared"
)

type Reader struct{ Q quotes.Quotes }

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
