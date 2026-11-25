package domain

import (
	"errors"
	"testing"

	"sweveh/internal/shared"
)

// V-07 (ADR-0040): la partida suma su valor; el detalle interno no cambia nada del total, con IVA incluido o aparte y
// con descuento.
func TestPartida_TotalsIgnoreDetail(t *testing.T) {
	ref := int64(999999)
	part := Line{ID: shared.NewID(), Kind: KindPartida, Description: "Frenos completos", Qty: 1, UnitPrice: 480000, FreePrice: true, DiscountPct: 5,
		Detail: []DetailItem{{Description: "Pastillas", Qty: 2, ValueRef: &ref}, {Description: "Rectificar discos", Qty: 1}}}
	line := Line{ID: shared.NewID(), Kind: "servicio", Description: "Alineación", Qty: 1, UnitPrice: 60000}
	// 480.000 − 24.000 + 60.000 = 516.000; con IVA aparte, 19 % = 98.040.
	if got := quote(19, false, part, line).Totals(); got != (Totals{Gross: 540000, Discount: 24000, Base: 516000, VAT: 98040, Total: 614040}) {
		t.Fatalf("IVA aparte: %+v", got)
	}
	if got := quote(19, true, part, line).Totals(); got.Total != 516000 || got.VAT != 82387 {
		t.Fatalf("IVA incluido: %+v", got)
	}
	noDetail := part
	noDetail.Detail = nil
	if quote(19, false, part, line).Total() != quote(19, false, noDetail, line).Total() {
		t.Fatal("el detalle no debe cambiar el total")
	}
	// No es un repuesto: aprobar no descuenta inventario por una partida.
	yes := true
	part.Approved = &yes
	if len(quote(19, false, part).ApprovedParts()) != 0 {
		t.Fatal("una partida no descuenta inventario")
	}
}

func TestPartida_RulesAndDetail(t *testing.T) {
	q := quote(19, false)
	if err := q.AddLine(Line{ID: shared.NewID(), Kind: KindPartida, Description: "Pintura", Qty: 1, UnitPrice: 0}); !errors.Is(err, ErrPartidaValue) {
		t.Fatalf("partida sin valor: %v", err)
	}
	id := shared.NewID()
	if err := q.AddLine(Line{ID: id, Kind: KindPartida, Description: "Pintura capó", Qty: 1, UnitPrice: 350000, FreePrice: true}); err != nil {
		t.Fatal(err)
	}
	zero := int64(0)
	if _, err := q.UpdateLine(id, nil, &zero); !errors.Is(err, ErrPartidaValue) {
		t.Fatalf("valor en cero: %v", err)
	}
	if l, err := q.Rename(id, "  Pintura   capó y guardabarros "); err != nil || l.Description != "Pintura capó y guardabarros" {
		t.Fatalf("renombrar: %v %v", l, err)
	}
	l, err := q.SetDetail(id, []DetailItem{{Description: " Lija  400 ", Qty: 0}, {Description: "Pintura", Qty: 2}})
	if err != nil || len(l.Detail) != 2 || l.Detail[0].Description != "Lija 400" || l.Detail[0].Qty != 1 {
		t.Fatalf("detalle: %+v %v", l, err)
	}
	for _, bad := range [][]DetailItem{{{Description: ""}}, {{Description: "x", Qty: -1}}, make([]DetailItem, 41)} {
		if _, err := q.SetDetail(id, bad); !errors.Is(err, ErrDetailInvalid) {
			t.Fatalf("detalle inválido %v: %v", bad, err)
		}
	}
	other := shared.NewID()
	_ = q.AddLine(Line{ID: other, Kind: "servicio", Description: "Alineación", Qty: 1, UnitPrice: 60000})
	if _, err := q.SetDetail(other, nil); !errors.Is(err, ErrNotPartida) {
		t.Fatalf("detalle en un servicio: %v", err)
	}
	if _, err := q.Rename(other, "Otro"); err == nil {
		t.Fatal("el nombre de un servicio de lista no se cambia aquí")
	}
	if _, err := q.SetDetail(shared.NewID(), nil); !errors.Is(err, ErrLineNotFound) {
		t.Fatal(err)
	}
	if err := q.SetShowDetail(true); err != nil || !q.ShowDetail {
		t.Fatal("mostrar detalle")
	}
	q.State = Sent
	if err := q.SetShowDetail(false); !errors.Is(err, ErrNotEditable) {
		t.Fatal("enviada no cambia lo que vio el cliente")
	}
	if _, err := q.SetDetail(id, nil); !errors.Is(err, ErrNotEditable) {
		t.Fatal(err)
	}
	if _, err := q.Rename(id, "x"); !errors.Is(err, ErrNotEditable) {
		t.Fatal(err)
	}
}
