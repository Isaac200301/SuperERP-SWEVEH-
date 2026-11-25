package domain

import "time"

// ClientDetail es un renglón del detalle de una partida tal como lo ve el cliente: sin el valor de referencia,
// que es interno del taller (ADR-0040, ADR-0043).
type ClientDetail struct {
	Description string
	Qty         int
}

// ClientLine es un ítem de la cotización para el cliente. No lleva la referencia al catálogo, ni si el precio lo
// puso la persona, ni las existencias, ni las horas de mano de obra: nada de eso es del cliente.
type ClientLine struct {
	Kind        string
	Description string
	Qty         int
	UnitPrice   int64
	Subtotal    int64
	DiscountPct int
	Discount    int64
	Approved    *bool          // respuesta ítem por ítem (HU-30); nil si todavía no hay respuesta
	Detail      []ClientDetail // solo partidas, y solo si el taller encendió "Mostrar detalle al cliente"
}

// ClientView es lo único de la cotización que sale de la pantalla del taller hacia el cliente (V-10): la vista en
// pantalla y el PDF se arman con esto, nunca con la cotización completa.
type ClientView struct {
	Number           int
	State            State
	Lines            []ClientLine
	Totals           Totals
	VATPct           int
	PricesIncludeVAT bool
	SentAt           *time.Time
	ExpiresAt        *time.Time
	Expired          bool
	ShowDetail       bool
}

// ClientView arma la vista del cliente a la hora now. El detalle interno de las partidas solo va si ShowDetail.
func (q Quote) ClientView(now time.Time) ClientView {
	v := ClientView{Number: q.Number, State: q.State, Lines: make([]ClientLine, 0, len(q.Lines)), Totals: q.Totals(),
		VATPct: q.VATPct, PricesIncludeVAT: q.PricesIncludeVAT, SentAt: q.SentAt, ExpiresAt: q.ExpiresAt, ShowDetail: q.ShowDetail,
		Expired: (q.State == Sent || q.State == Doubts) && q.Expired(now)}
	for _, l := range q.Lines {
		cl := ClientLine{Kind: l.Kind, Description: l.Description, Qty: l.Qty, UnitPrice: l.UnitPrice, Subtotal: l.Subtotal(),
			DiscountPct: l.DiscountPct, Discount: l.LineDiscount(), Approved: l.Approved}
		if l.Kind == KindPartida && q.ShowDetail {
			for _, d := range l.Detail {
				cl.Detail = append(cl.Detail, ClientDetail{Description: d.Description, Qty: d.Qty})
			}
		}
		v.Lines = append(v.Lines, cl)
	}
	return v
}
