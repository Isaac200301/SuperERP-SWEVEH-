// Package postgres guarda cotizaciones y sus líneas (pgx, ADR-0024).
package postgres

import (
	"context"
	"time"

	"sweveh/internal/platform/db"
	"sweveh/internal/quotes/domain"
	"sweveh/internal/shared"
)

type Repository struct{}

type qRow struct {
	ID         shared.ID  `db:"id"`
	TallerID   shared.ID  `db:"taller_id"`
	OrdenID    shared.ID  `db:"orden_id"`
	Numero     int        `db:"numero"`
	Estado     string     `db:"estado"`
	EnviadaEn  *time.Time `db:"enviada_en"`
	Respondida *time.Time `db:"respondida_en"`
	Canal      *string    `db:"canal_respuesta"`
	Ref        *string    `db:"respuesta_ref"`
	Version    int        `db:"version"`
	IVA        int        `db:"iva_pct"`
	ConIVA     bool       `db:"precios_con_iva"`
	VenceEn    *time.Time `db:"vence_en"`
}

type lRow struct {
	ID          shared.ID  `db:"id"`
	Tipo        string     `db:"tipo"`
	RefID       *shared.ID `db:"ref_id"`
	Descripcion string     `db:"descripcion"`
	Cantidad    int        `db:"cantidad"`
	Precio      int64      `db:"precio_unitario"`
	SinStock    bool       `db:"sin_stock"`
	Aprobada    *bool      `db:"aprobada"`
	Libre       bool       `db:"precio_libre"`
	Descuento   int        `db:"descuento_pct"`
	Horas       *int       `db:"horas_cent"`
}

const qCols = `id, taller_id, orden_id, numero, estado, enviada_en, respondida_en, canal_respuesta, respuesta_ref, version,
	iva_pct, precios_con_iva, vence_en`

func (Repository) load(ctx context.Context, r qRow) (domain.Quote, error) {
	q := domain.Quote{ID: r.ID, TallerID: r.TallerID, OrderID: r.OrdenID, Number: r.Numero, State: domain.State(r.Estado),
		SentAt: r.EnviadaEn, RespondedAt: r.Respondida, Channel: r.Canal, ResponseRef: r.Ref, Version: r.Version,
		VATPct: r.IVA, PricesIncludeVAT: r.ConIVA, ExpiresAt: r.VenceEn}
	lines, err := db.Many[lRow](ctx, `SELECT id, tipo, ref_id, descripcion, cantidad, precio_unitario, sin_stock, aprobada, precio_libre,
		descuento_pct, horas_cent FROM quotes.linea WHERE taller_id = $1 AND cotizacion_id = $2 ORDER BY created_at, id`, r.TallerID, r.ID)
	for _, l := range lines {
		q.Lines = append(q.Lines, domain.Line{ID: l.ID, Kind: l.Tipo, RefID: l.RefID, Description: l.Descripcion, Qty: l.Cantidad,
			UnitPrice: l.Precio, NoStock: l.SinStock, Approved: l.Aprobada, FreePrice: l.Libre, DiscountPct: l.Descuento, HoursCent: l.Horas})
	}
	return q, err
}

func (r Repository) Latest(ctx context.Context, tallerID, orderID shared.ID) (domain.Quote, bool, error) {
	rows, err := db.Many[qRow](ctx, `SELECT `+qCols+` FROM quotes.cotizacion WHERE taller_id = $1 AND orden_id = $2
		ORDER BY numero DESC LIMIT 1`, tallerID, orderID)
	if err != nil || len(rows) == 0 {
		return domain.Quote{}, false, err
	}
	q, err := r.load(ctx, rows[0])
	return q, true, err
}

func (r Repository) Get(ctx context.Context, tallerID, id shared.ID) (domain.Quote, error) {
	row, err := db.One[qRow](ctx, domain.ErrNotFound, `SELECT `+qCols+` FROM quotes.cotizacion WHERE taller_id = $1 AND id = $2`, tallerID, id)
	if err != nil {
		return domain.Quote{}, err
	}
	return r.load(ctx, row)
}

func (r Repository) PendingByCustomerOrder(ctx context.Context, tallerID, orderID shared.ID) (domain.Quote, bool, error) {
	q, ok, err := r.Latest(ctx, tallerID, orderID)
	if !ok || err != nil || (q.State != domain.Sent && q.State != domain.Doubts) {
		return domain.Quote{}, false, err
	}
	return q, true, nil
}

func (Repository) Insert(ctx context.Context, q domain.Quote) error {
	_, err := db.Exec(ctx, `INSERT INTO quotes.cotizacion (id, taller_id, orden_id, numero, estado, total, iva_pct, precios_con_iva)
		VALUES ($1, $2, $3, $4, $5, 0, $6, $7)`, q.ID, q.TallerID, q.OrderID, q.Number, string(q.State), q.VATPct, q.PricesIncludeVAT)
	return err
}

// Save reemplaza la cabecera (con versión) y las líneas.
func (Repository) Save(ctx context.Context, q domain.Quote, prev int) error {
	t := q.Totals()
	n, err := db.Exec(ctx, `UPDATE quotes.cotizacion SET estado = $3, total = $4, enviada_en = $5, respondida_en = $6,
		canal_respuesta = $7, respuesta_ref = $8, version = $9, subtotal = $11, descuento = $12, iva = $13, vence_en = $14
		WHERE taller_id = $1 AND id = $2 AND version = $10`,
		q.TallerID, q.ID, string(q.State), t.Total, q.SentAt, q.RespondedAt, q.Channel, q.ResponseRef, q.Version, prev,
		t.Gross, t.Discount, t.VAT, q.ExpiresAt)
	if err != nil {
		return err
	}
	if n == 0 {
		return shared.ErrVersionConflict
	}
	if _, err := db.Exec(ctx, `DELETE FROM quotes.linea WHERE taller_id = $1 AND cotizacion_id = $2`, q.TallerID, q.ID); err != nil {
		return err
	}
	for i, l := range q.Lines {
		if _, err := db.Exec(ctx, `INSERT INTO quotes.linea (id, taller_id, cotizacion_id, tipo, ref_id, descripcion, cantidad,
			precio_unitario, sin_stock, aprobada, precio_libre, descuento_pct, horas_cent, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, now() + make_interval(secs => $14))`,
			l.ID, q.TallerID, q.ID, l.Kind, l.RefID, l.Description, l.Qty, l.UnitPrice, l.NoStock, l.Approved, l.FreePrice,
			l.DiscountPct, l.HoursCent, float64(i)/1000); err != nil {
			return err
		}
	}
	return nil
}
