package domain

import (
	"errors"
	"testing"
	"time"

	"sweveh/internal/shared"
)

// V-08 (HU-106): un gasto lleva qué fue y un valor mayor que cero; anulado no suma; quedan negativo = se pasó.
func TestExpense_NewAndSpend(t *testing.T) {
	now := time.Now()
	order := shared.NewID()
	e, err := NewExpense(shared.NewID(), order, "  Pastillas   delanteras ", 60000, "", nil, nil, now)
	if err != nil || e.Description != "Pastillas delanteras" || e.Origin != ExpenseManual {
		t.Fatalf("gasto: %+v %v", e, err)
	}
	for _, bad := range []struct {
		desc  string
		value int64
	}{{"", 1}, {"x", 0}, {"x", -5}, {string(make([]rune, 201)), 1}} {
		if _, err := NewExpense(shared.NewID(), order, bad.desc, bad.value, "", nil, nil, now); !errors.Is(err, ErrExpense) {
			t.Fatalf("%+v: %v", bad, err)
		}
	}
	if _, err := NewExpense(shared.NewID(), order, "x", 1, "magia", nil, nil, now); !errors.Is(err, ErrExpenseOrigin) {
		t.Fatal(err)
	}
	voided := Expense{Value: 55000, Voided: true}
	if s := SpendOf(100000, []Expense{e, voided, {Value: 50000}}); s != (Spend{Quoted: 100000, Spent: 110000, Left: -10000}) {
		t.Fatalf("resumen: %+v", s)
	}
}
