// Package quotes es la fachada de cotizaciones.
package quotes

import (
	"context"

	quotespg "sweveh/internal/quotes/adapters/out/postgres"
	"sweveh/internal/quotes/app"
	"sweveh/internal/quotes/domain"
	"sweveh/internal/shared"
)

type Line struct {
	ID          shared.ID
	Kind        string
	Description string
	Qty         int
	UnitPrice   int64
}

type Summary struct {
	ID     shared.ID
	State  string
	Total  int64
	Number int
	Lines  []Line
}

type Quotes struct{ svc *app.Quotes }

func New(s *app.Quotes) Quotes { return Quotes{svc: s} }

func toSummary(q *domain.Quote) *Summary {
	if q == nil {
		return nil
	}
	s := &Summary{ID: q.ID, State: string(q.State), Total: q.Total(), Number: q.Number}
	for _, l := range q.Lines {
		s.Lines = append(s.Lines, Line{ID: l.ID, Kind: l.Kind, Description: l.Description, Qty: l.Qty, UnitPrice: l.UnitPrice})
	}
	return s
}

// ForOrder devuelve la cotización vigente de una orden o nil.
func (q Quotes) ForOrder(ctx context.Context, tallerID, orderID shared.ID) (*Summary, error) {
	x, err := q.svc.ForOrder(ctx, tallerID, orderID)
	return toSummary(x), err
}

// Pending lista las cotizaciones esperando respuesta (HU-69), en la transacción del llamador.
func (q Quotes) Pending(ctx context.Context, tallerID shared.ID) ([]quotespg.Pending, error) {
	return quotespg.Stats{}.Pending(ctx, tallerID)
}

// ApprovedTotals suma lo aprobado por orden (HU-71), en la transacción del llamador.
func (q Quotes) ApprovedTotals(ctx context.Context, tallerID shared.ID, ids []shared.ID) (map[shared.ID]int64, error) {
	return quotespg.Stats{}.ApprovedTotals(ctx, tallerID, ids)
}

// Respond registra la respuesta del cliente (botón de WhatsApp, HU-28).
func (q Quotes) Respond(ctx context.Context, tallerID, orderID shared.ID, decision, channel, ref string, actor shared.Actor) error {
	_, err := q.svc.Respond(ctx, app.RespondCmd{TallerID: tallerID, OrderID: orderID, Actor: actor,
		Response: domain.Response{Decision: decision, Channel: channel, Ref: ref}})
	return err
}
