package postgres

import (
	"context"
	"time"

	"sweveh/internal/orders/domain"
	"sweveh/internal/platform/db"
	"sweveh/internal/shared"
)

// Findings implementa ports.FindingRepository con pgx (ADR-0024).
type Findings struct{}

type findingRow struct {
	ID          shared.ID  `db:"id"`
	TallerID    shared.ID  `db:"taller_id"`
	OrdenID     shared.ID  `db:"orden_id"`
	Sistema     string     `db:"sistema"`
	Descripcion string     `db:"descripcion"`
	Gravedad    string     `db:"gravedad"`
	Codigos     []string   `db:"codigos_falla"`
	AutorID     *shared.ID `db:"autor_id"`
	CreatedAt   time.Time  `db:"created_at"`
}

const findingCols = `id, taller_id, orden_id, sistema, descripcion, gravedad, codigos_falla, autor_id, created_at`

func (r findingRow) domain() domain.Finding {
	return domain.Finding{ID: r.ID, TallerID: r.TallerID, OrderID: r.OrdenID, System: r.Sistema, Description: r.Descripcion,
		Severity: r.Gravedad, FaultCodes: r.Codigos, AuthorID: r.AutorID, CreatedAt: r.CreatedAt}
}

func (Findings) InsertFinding(ctx context.Context, f domain.Finding) error {
	_, err := db.Exec(ctx, `INSERT INTO orders.hallazgo (`+findingCols+`) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		f.ID, f.TallerID, f.OrderID, f.System, f.Description, f.Severity, f.FaultCodes, f.AuthorID, f.CreatedAt)
	return err
}

func (Findings) ListFindings(ctx context.Context, tallerID, orderID shared.ID) ([]domain.Finding, error) {
	rows, err := db.Many[findingRow](ctx, `SELECT `+findingCols+` FROM orders.hallazgo WHERE taller_id = $1 AND orden_id = $2
		ORDER BY created_at, id`, tallerID, orderID)
	out := make([]domain.Finding, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.domain())
	}
	return out, err
}

func (Findings) DeleteFinding(ctx context.Context, tallerID, orderID, id shared.ID) (domain.Finding, error) {
	r, err := db.One[findingRow](ctx, domain.ErrFindingNotFound, `DELETE FROM orders.hallazgo
		WHERE taller_id = $1 AND orden_id = $2 AND id = $3 RETURNING `+findingCols, tallerID, orderID, id)
	return r.domain(), err
}
