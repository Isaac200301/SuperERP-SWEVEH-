package ports

import (
	"context"

	"sweveh/internal/orders/domain"
	"sweveh/internal/shared"
)

// ExpenseRepository guarda lo gastado en cada orden (V-08, HU-106).
type ExpenseRepository interface {
	// InsertExpense es idempotente por id (lo genera la pantalla): si ya existe, no hace nada y devuelve false.
	InsertExpense(ctx context.Context, tallerID shared.ID, e domain.Expense) (bool, error)
	GetExpense(ctx context.Context, tallerID, orderID, id shared.ID) (domain.Expense, error)
	// VoidExpense marca anulado solo si no lo estaba; devuelve si cambió algo.
	VoidExpense(ctx context.Context, tallerID, orderID, id shared.ID, by *shared.ID) (bool, error)
	ListExpenses(ctx context.Context, tallerID, orderID shared.ID) ([]domain.Expense, error)
	// SpentByOpenOrder suma lo gastado (sin anulados) en las órdenes abiertas que tienen gastos (KPI 5).
	SpentByOpenOrder(ctx context.Context, tallerID shared.ID) (map[shared.ID]int64, error)
	// EvidenceInOrder dice si la foto es de esa orden (anti-IDOR al adjuntar la factura).
	EvidenceInOrder(ctx context.Context, tallerID, orderID, evidenceID shared.ID) (bool, error)
}

// QuotedTotal es lo cotizado de una orden: lo aprobado (sumando las adicionales) o, si aún no hay aprobación, el total
// de la cotización vigente. Lo da la fachada de quotes.
type QuotedTotal interface {
	Quoted(ctx context.Context, tallerID, orderID shared.ID) (int64, error)
}
