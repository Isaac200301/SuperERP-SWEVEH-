package postgres

import (
	"context"
	"time"

	"sweveh/internal/platform/db"
	"sweveh/internal/shared"
)

// Pending es una cotización esperando respuesta (HU-69).
type Pending struct {
	OrderID shared.ID `db:"orden_id"`
	SentAt  time.Time `db:"enviada_en"`
}

// Stats son las cifras de cotizaciones para los KPIs.
type Stats struct{}

func (Stats) Pending(ctx context.Context, tallerID shared.ID) ([]Pending, error) {
	return db.Many[Pending](ctx, `SELECT orden_id, enviada_en FROM quotes.cotizacion WHERE taller_id = $1 AND estado IN ('enviada', 'dudas')
		AND enviada_en IS NOT NULL ORDER BY enviada_en`, tallerID)
}

// ApprovedTotals suma lo aprobado por orden (ticket promedio, HU-71).
func (Stats) ApprovedTotals(ctx context.Context, tallerID shared.ID, orderIDs []shared.ID) (map[shared.ID]int64, error) {
	type r struct {
		OrdenID shared.ID `db:"orden_id"`
		Total   int64     `db:"total"`
	}
	rows, err := db.Many[r](ctx, `SELECT orden_id, sum(total)::bigint AS total FROM quotes.cotizacion WHERE taller_id = $1
		AND estado = 'aprobada' AND orden_id = ANY($2) GROUP BY orden_id`, tallerID, orderIDs)
	out := map[shared.ID]int64{}
	for _, x := range rows {
		out[x.OrdenID] = x.Total
	}
	return out, err
}
