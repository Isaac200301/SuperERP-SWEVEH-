package postgres

import (
	"context"
	"time"

	"sweveh/internal/orders/domain"
	"sweveh/internal/platform/db"
	"sweveh/internal/shared"
)

// Expenses implementa ports.ExpenseRepository con pgx (V-08).
type Expenses struct{}

type expenseRow struct {
	ID          shared.ID  `db:"id"`
	OrdenID     shared.ID  `db:"orden_id"`
	Descripcion string     `db:"descripcion"`
	Valor       int64      `db:"valor"`
	Origen      string     `db:"origen"`
	FotoID      *shared.ID `db:"foto_id"`
	CreadoPor   *shared.ID `db:"creado_por"`
	CreadoEn    time.Time  `db:"creado_en"`
	Anulado     bool       `db:"anulado"`
}

const expenseCols = `id, orden_id, descripcion, valor, origen, foto_id, creado_por, creado_en, anulado`

func (r expenseRow) domain() domain.Expense {
	return domain.Expense{ID: r.ID, OrderID: r.OrdenID, Description: r.Descripcion, Value: r.Valor, Origin: r.Origen,
		PhotoID: r.FotoID, CreatedBy: r.CreadoPor, CreatedAt: r.CreadoEn, Voided: r.Anulado}
}

func (Expenses) InsertExpense(ctx context.Context, tallerID shared.ID, e domain.Expense) (bool, error) {
	n, err := db.Exec(ctx, `INSERT INTO orders.gasto (id, taller_id, orden_id, descripcion, valor, origen, foto_id, creado_por, creado_en)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) ON CONFLICT (id) DO NOTHING`,
		e.ID, tallerID, e.OrderID, e.Description, e.Value, e.Origin, e.PhotoID, e.CreatedBy, e.CreatedAt)
	return n == 1, err
}

func (Expenses) GetExpense(ctx context.Context, tallerID, orderID, id shared.ID) (domain.Expense, error) {
	r, err := db.One[expenseRow](ctx, domain.ErrExpenseNF, `SELECT `+expenseCols+` FROM orders.gasto
		WHERE taller_id = $1 AND orden_id = $2 AND id = $3`, tallerID, orderID, id)
	return r.domain(), err
}

func (Expenses) VoidExpense(ctx context.Context, tallerID, orderID, id shared.ID, by *shared.ID) (bool, error) {
	n, err := db.Exec(ctx, `UPDATE orders.gasto SET anulado = true, anulado_en = now(), anulado_por = $4
		WHERE taller_id = $1 AND orden_id = $2 AND id = $3 AND NOT anulado`, tallerID, orderID, id, by)
	return n == 1, err
}

func (Expenses) ListExpenses(ctx context.Context, tallerID, orderID shared.ID) ([]domain.Expense, error) {
	rows, err := db.Many[expenseRow](ctx, `SELECT `+expenseCols+` FROM orders.gasto WHERE taller_id = $1 AND orden_id = $2
		ORDER BY creado_en, id LIMIT 500`, tallerID, orderID)
	out := make([]domain.Expense, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.domain())
	}
	return out, err
}

type spentRow struct {
	OrdenID shared.ID `db:"orden_id"`
	Gastado int64     `db:"gastado"`
}

func (Expenses) SpentByOpenOrder(ctx context.Context, tallerID shared.ID) (map[shared.ID]int64, error) {
	rows, err := db.Many[spentRow](ctx, `SELECT g.orden_id, sum(g.valor)::bigint AS gastado FROM orders.gasto g
		JOIN orders.orden o ON o.id = g.orden_id AND o.taller_id = g.taller_id
		WHERE g.taller_id = $1 AND NOT g.anulado AND o.estado NOT IN ('entregado', 'cancelada')
		GROUP BY g.orden_id`, tallerID)
	out := map[shared.ID]int64{}
	for _, r := range rows {
		out[r.OrdenID] = r.Gastado
	}
	return out, err
}

func (Expenses) EvidenceInOrder(ctx context.Context, tallerID, orderID, evidenceID shared.ID) (bool, error) {
	n, err := db.Scalar[int](ctx, `SELECT count(*) FROM orders.evidencia WHERE taller_id = $1 AND orden_id = $2 AND id = $3`,
		tallerID, orderID, evidenceID)
	return n == 1, err
}
