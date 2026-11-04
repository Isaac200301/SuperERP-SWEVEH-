package postgres

import (
	"context"
	"time"

	"sweveh/internal/catalog/domain"
	"sweveh/internal/platform/db"
	"sweveh/internal/shared"
)

// Proposals guarda las filas de la lista de precios por foto (HU-93, ADR-0032).
type Proposals struct{}

type proposalRow struct {
	ID        shared.ID  `db:"id"`
	TallerID  shared.ID  `db:"taller_id"`
	Lote      shared.ID  `db:"lote_id"`
	Clave     string     `db:"foto_clave"`
	Tipo      string     `db:"foto_tipo"`
	Nombre    string     `db:"nombre"`
	Precio    *int64     `db:"precio"`
	Vehiculo  string     `db:"tipo_vehiculo"`
	Estado    string     `db:"estado"`
	Servicio  *shared.ID `db:"servicio_id"`
	CreatedAt time.Time  `db:"created_at"`
}

func (r proposalRow) domain() domain.Proposal {
	return domain.Proposal{ID: r.ID, TallerID: r.TallerID, BatchID: r.Lote, PhotoKey: r.Clave, PhotoType: r.Tipo, Name: r.Nombre,
		Price: r.Precio, Kind: r.Vehiculo, State: r.Estado, ServiceID: r.Servicio, CreatedAt: r.CreatedAt}
}

const proposalCols = `id, taller_id, lote_id, foto_clave, foto_tipo, nombre, precio, tipo_vehiculo, estado, servicio_id, created_at`

func (Proposals) InsertProposals(ctx context.Context, ps []domain.Proposal) error {
	for _, p := range ps {
		if _, err := db.Exec(ctx, `INSERT INTO catalog.propuesta (id, taller_id, lote_id, foto_clave, foto_tipo, nombre, precio, tipo_vehiculo,
			created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`, p.ID, p.TallerID, p.BatchID, p.PhotoKey, p.PhotoType, p.Name, p.Price,
			p.Kind, p.CreatedAt); err != nil {
			return err
		}
	}
	return nil
}

func (Proposals) Proposals(ctx context.Context, tallerID shared.ID, onlyPending bool) ([]domain.Proposal, error) {
	rows, err := db.Many[proposalRow](ctx, `SELECT `+proposalCols+` FROM catalog.propuesta WHERE taller_id = $1
		AND (NOT $2 OR estado = 'pendiente') ORDER BY created_at, nombre LIMIT 200`, tallerID, onlyPending)
	out := make([]domain.Proposal, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.domain())
	}
	return out, err
}

func (Proposals) Proposal(ctx context.Context, tallerID, id shared.ID) (domain.Proposal, error) {
	r, err := db.One[proposalRow](ctx, domain.ErrProposalNotFound, `SELECT `+proposalCols+` FROM catalog.propuesta
		WHERE taller_id = $1 AND id = $2`, tallerID, id)
	return r.domain(), err
}

// ResolveProposal cierra una fila pendiente; false si ya estaba resuelta.
func (Proposals) ResolveProposal(ctx context.Context, tallerID, id shared.ID, state string, serviceID *shared.ID, by shared.ID) (bool, error) {
	n, err := db.Exec(ctx, `UPDATE catalog.propuesta SET estado = $3, servicio_id = $4, resuelto_por = $5, resuelto_en = now()
		WHERE taller_id = $1 AND id = $2 AND estado = 'pendiente'`, tallerID, id, state, serviceID, by)
	return n == 1, err
}
