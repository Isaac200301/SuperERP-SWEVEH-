package domain

import (
	"testing"
	"time"

	"sweveh/internal/shared"
)

func viewQuote(show bool) Quote {
	ref := int64(130000)
	sid := shared.NewID()
	part := Line{ID: shared.NewID(), Kind: KindPartida, Description: "Frenos completos", Qty: 1, UnitPrice: 480000, FreePrice: true, DiscountPct: 10,
		Detail: []DetailItem{{Description: "Pastillas delanteras", Qty: 2, ValueRef: &ref}, {Description: "Rectificar discos", Qty: 1}}}
	srv := Line{ID: shared.NewID(), Kind: "servicio", RefID: &sid, Description: "Alineación", Qty: 1, UnitPrice: 60000, NoStock: true}
	q := quote(0, true, part, srv)
	q.Number, q.State, q.ShowDetail = 2, Sent, show
	return q
}

// V-10 (ADR-0043): el detalle de una partida solo llega al cliente si el taller encendió "Mostrar detalle al cliente",
// y nunca con su valor de referencia.
func TestClientView_DetailOnlyWhenShown(t *testing.T) {
	off := viewQuote(false).ClientView(time.Now())
	if len(off.Lines) != 2 || off.Lines[0].Detail != nil || off.ShowDetail {
		t.Fatalf("detalle apagado: %+v", off.Lines[0])
	}
	on := viewQuote(true).ClientView(time.Now())
	d := on.Lines[0].Detail
	if len(d) != 2 || d[0] != (ClientDetail{Description: "Pastillas delanteras", Qty: 2}) || d[1].Qty != 1 {
		t.Fatalf("detalle encendido: %+v", d)
	}
	if on.Lines[1].Detail != nil {
		t.Fatal("un servicio no lleva detalle")
	}
}

func TestClientView_AmountsAndState(t *testing.T) {
	q := viewQuote(true)
	exp := time.Date(2026, 12, 10, 12, 0, 0, 0, time.UTC)
	q.ExpiresAt = &exp
	v := q.ClientView(exp.Add(-time.Hour))
	if v.Number != 2 || v.State != Sent || v.Expired {
		t.Fatalf("estado: %+v", v)
	}
	if l := v.Lines[0]; l.Subtotal != 480000 || l.Discount != 48000 || l.DiscountPct != 10 || l.UnitPrice != 480000 {
		t.Fatalf("partida: %+v", l)
	}
	if v.Totals.Total != 492000 || v.Totals.Discount != 48000 { // 480.000 − 48.000 + 60.000
		t.Fatalf("totales: %+v", v.Totals)
	}
	if !viewQuote(true).withExpiry(exp).ClientView(exp.Add(time.Hour)).Expired {
		t.Fatal("pasada la vigencia debe salir vencida")
	}
	// Una cotización ya respondida no se marca vencida aunque haya pasado la fecha.
	done := q
	done.State = Approved
	if done.ClientView(exp.Add(time.Hour)).Expired {
		t.Fatal("una aprobada no está vencida")
	}
}

// Solo cuenta lo aprobado cuando el cliente respondió ítem por ítem (HU-30); el ítem no aprobado sale marcado.
func TestClientView_ItemByItem(t *testing.T) {
	q := viewQuote(false)
	yes, no := true, false
	q.Lines[0].Approved, q.Lines[1].Approved = &no, &yes
	v := q.ClientView(time.Now())
	if v.Totals.Total != 60000 || v.Lines[0].Approved == nil || *v.Lines[0].Approved || !*v.Lines[1].Approved {
		t.Fatalf("ítem por ítem: %+v", v)
	}
}

func (q Quote) withExpiry(t time.Time) Quote {
	q.ExpiresAt = &t
	return q
}
