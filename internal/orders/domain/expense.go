package domain

import (
	"strings"
	"time"

	"sweveh/internal/shared"
)

// Expense es lo que el taller gastó en una orden (V-08, ADR-0040, HU-106): interno, nunca lo ve el cliente.
type Expense struct {
	ID          shared.ID
	OrderID     shared.ID
	Description string
	Value       int64
	Origin      string // manual | asistente | factura
	PhotoID     *shared.ID
	CreatedBy   *shared.ID
	CreatedAt   time.Time
	Voided      bool
}

const (
	ExpenseManual    = "manual"
	ExpenseAssistant = "asistente"
	ExpenseInvoice   = "factura"
)

var (
	ErrExpense       = shared.Validation("gasto_invalido", "El gasto necesita qué fue (hasta 200 letras) y un valor mayor que cero.")
	ErrExpenseOrigin = shared.Validation("gasto_origen", "El origen del gasto es manual, asistente o factura.").WithField("origen")
	ErrExpenseNF     = shared.NotFound("gasto_no_encontrado", "No encontramos ese gasto en la orden.")
)

// NewExpense valida y limpia un gasto. El valor lo pone el taller (o sale de la factura y la persona lo confirma).
func NewExpense(id, orderID shared.ID, description string, value int64, origin string, photo *shared.ID, by *shared.ID, at time.Time) (Expense, error) {
	description = strings.Join(strings.Fields(description), " ")
	if description == "" || len([]rune(description)) > 200 || value <= 0 {
		return Expense{}, ErrExpense
	}
	if origin == "" {
		origin = ExpenseManual
	}
	if origin != ExpenseManual && origin != ExpenseAssistant && origin != ExpenseInvoice {
		return Expense{}, ErrExpenseOrigin
	}
	return Expense{ID: id, OrderID: orderID, Description: description, Value: value, Origin: origin, PhotoID: photo, CreatedBy: by, CreatedAt: at}, nil
}

// Spend es el resumen de una orden: lo cotizado, lo gastado (sin anulados) y lo que queda (negativo = se pasó).
type Spend struct {
	Quoted int64
	Spent  int64
	Left   int64
}

// SpendOf suma los gastos no anulados contra lo cotizado.
func SpendOf(quoted int64, list []Expense) Spend {
	var spent int64
	for _, e := range list {
		if !e.Voided {
			spent += e.Value
		}
	}
	return Spend{Quoted: quoted, Spent: spent, Left: quoted - spent}
}
