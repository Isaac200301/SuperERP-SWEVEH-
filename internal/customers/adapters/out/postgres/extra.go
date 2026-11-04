package postgres

import (
	"context"
	"time"

	"sweveh/internal/customers/app"
	"sweveh/internal/customers/domain"
	"sweveh/internal/platform/db"
	"sweveh/internal/shared"
)

// Extra implementa app.Extra con pgx (ADR-0024).
type Extra struct{}

type cRow struct {
	ID        shared.ID `db:"id"`
	TallerID  shared.ID `db:"taller_id"`
	Celular   string    `db:"celular"`
	Nombre    string    `db:"nombre"`
	Correo    *string   `db:"correo"`
	Estado    string    `db:"autorizacion_estado"`
	Version   int       `db:"version"`
	CreatedAt time.Time `db:"created_at"`
}

const cCols = `id, taller_id, celular, nombre, correo, autorizacion_estado, version, created_at`

func cList(rows []cRow, err error) ([]domain.Customer, error) {
	out := make([]domain.Customer, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.Customer{ID: r.ID, TallerID: r.TallerID, Phone: r.Celular, Name: r.Nombre, Email: r.Correo,
			Consent: domain.ConsentStatus(r.Estado), Version: r.Version, CreatedAt: r.CreatedAt})
	}
	return out, err
}

func (Extra) Search(ctx context.Context, tallerID shared.ID, digits, text string, limit int) ([]domain.Customer, error) {
	if digits != "" {
		return cList(db.Many[cRow](ctx, `SELECT `+cCols+` FROM customers.cliente WHERE taller_id = $1 AND celular LIKE '%' || $2 || '%'
			ORDER BY nombre LIMIT $3`, tallerID, digits, limit))
	}
	return cList(db.Many[cRow](ctx, `SELECT `+cCols+` FROM customers.cliente WHERE taller_id = $1
		AND nombre ILIKE '%' || replace(replace(replace($2, '\', '\\'), '%', '\%'), '_', '\_') || '%' ORDER BY nombre LIMIT $3`, tallerID, text, limit))
}

func (Extra) List(ctx context.Context, tallerID shared.ID, limit int) ([]domain.Customer, error) {
	return cList(db.Many[cRow](ctx, `SELECT `+cCols+` FROM customers.cliente WHERE taller_id = $1 ORDER BY created_at DESC LIMIT $2`, tallerID, limit))
}

func (Extra) SetName(ctx context.Context, tallerID, id shared.ID, name string) error {
	_, err := db.Exec(ctx, `UPDATE customers.cliente SET nombre = $3, version = version + 1, updated_at = now()
		WHERE taller_id = $1 AND id = $2`, tallerID, id, name)
	return err
}

func (Extra) SetConsent(ctx context.Context, tallerID, id shared.ID, st domain.ConsentStatus, r app.ConsentRecord) error {
	if _, err := db.Exec(ctx, `UPDATE customers.cliente SET autorizacion_estado = $3, version = version + 1, updated_at = now()
		WHERE taller_id = $1 AND id = $2`, tallerID, id, string(st)); err != nil {
		return err
	}
	var ref *string
	if r.Ref != "" {
		ref = &r.Ref
	}
	_, err := db.Exec(ctx, `INSERT INTO customers.autorizacion (id, taller_id, cliente_id, estado, texto, version_politica, canal, ref, ocurrida_en, usuario_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`, shared.NewID(), tallerID, id, string(st), r.Text, r.Version, r.Channel, ref, r.At, r.By)
	return err
}
