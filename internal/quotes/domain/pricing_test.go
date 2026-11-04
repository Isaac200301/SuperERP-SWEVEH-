package domain

import (
	"errors"
	"testing"
	"time"

	"sweveh/internal/shared"
)

func quote(vat int, included bool, lines ...Line) Quote {
	return Quote{State: Draft, VATPct: vat, PricesIncludeVAT: included, Lines: lines}
}

func TestTotals_VATIncludedOrAddedAndDiscounts(t *testing.T) {
	part := Line{ID: shared.NewID(), Kind: "repuesto", Qty: 2, UnitPrice: 65000, DiscountPct: 10}
	labor := Line{ID: shared.NewID(), Kind: "servicio", Qty: 1, UnitPrice: 2000}
	if got := quote(19, false, part, labor).Totals(); got != (Totals{Gross: 132000, Discount: 13000, Base: 119000, VAT: 22610, Total: 141610}) {
		t.Errorf("IVA aparte: %+v", got)
	}
	if got := quote(19, true, Line{Qty: 1, UnitPrice: 119000}).Totals(); got.Total != 119000 || got.VAT != 19000 {
		t.Errorf("IVA incluido: %+v", got)
	}
	if got := quote(0, true, labor); got.Total() != 2000 {
		t.Errorf("sin IVA: %d", got.Total())
	}
	rejected := false
	if got := quote(19, false, Line{Qty: 1, UnitPrice: 100000, Approved: &rejected}, labor).Totals(); got.Base != 2000 {
		t.Errorf("lo no aprobado no suma: %+v", got)
	}
}

func TestDiscountLaborAndValidity(t *testing.T) {
	l := Line{ID: shared.NewID(), Kind: "repuesto", Qty: 1, UnitPrice: 1000}
	q := quote(0, true, l)
	if _, err := q.SetDiscount(l.ID, 11, 10); !errors.Is(err, ErrDiscount) {
		t.Errorf("sobre el máximo: %v", err)
	}
	if _, err := q.SetDiscount(l.ID, 10, 10); err != nil || q.Total() != 900 {
		t.Errorf("10 %%: %v %d", err, q.Total())
	}
	rate := int64(80000)
	if p, err := LaborPrice(150, &rate); err != nil || p != 120000 {
		t.Errorf("1,5 h × 80.000: %d %v", p, err)
	}
	for _, bad := range []int{0, 30, 20025} {
		if _, err := LaborPrice(bad, &rate); !errors.Is(err, ErrHours) {
			t.Errorf("%d centésimas: %v", bad, err)
		}
	}
	if _, err := LaborPrice(100, nil); !errors.Is(err, ErrNoRate) {
		t.Errorf("sin tarifa: %v", err)
	}

	now := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	if err := q.Send(now, 15); err != nil || !q.ExpiresAt.Equal(now.AddDate(0, 0, 15)) {
		t.Fatalf("vigencia al enviar: %v %v", err, q.ExpiresAt)
	}
	late := now.AddDate(0, 0, 16)
	if _, err := q.Respond(Response{Decision: "aprobar", Channel: "telefono", At: late}); !errors.Is(err, ErrExpired) {
		t.Fatalf("vencida: %v", err)
	}
	if err := q.Renew(late, 15); err != nil {
		t.Fatal(err)
	}
	if ok, err := q.Respond(Response{Decision: "aprobar", Channel: "telefono", At: late}); err != nil || !ok {
		t.Fatalf("renovada se aprueba: %v %v", ok, err)
	}
}
