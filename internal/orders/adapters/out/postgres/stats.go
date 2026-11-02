package postgres

import (
	"context"
	"time"

	"sweveh/internal/orders/app"
	"sweveh/internal/platform/db"
	"sweveh/internal/shared"
)

// Stats implementa app.Stats: cifras de órdenes para los KPIs (HU-68, HU-70, HU-72) y avisos (AU-07).
// Las órdenes de ejemplo del registro (HU-05) no cuentan.
type Stats struct{}

func (Stats) StateCounts(ctx context.Context, tallerID shared.ID) (map[string]int, error) {
	type r struct {
		Estado string `db:"estado"`
		N      int    `db:"n"`
	}
	rows, err := db.Many[r](ctx, `SELECT estado, count(*)::int AS n FROM orders.orden WHERE taller_id = $1 AND NOT ejemplo
		AND estado NOT IN ('entregado', 'cancelada') GROUP BY estado`, tallerID)
	out := map[string]int{}
	for _, x := range rows {
		out[x.Estado] = x.N
	}
	return out, err
}

func (Stats) AvgCycleDays(ctx context.Context, tallerID shared.ID, since time.Time) (*float64, int, error) {
	type r struct {
		Dias *float64 `db:"dias"`
		N    int      `db:"n"`
	}
	rows, err := db.Many[r](ctx, `SELECT avg(EXTRACT(EPOCH FROM (cerrada_en - created_at)) / 86400) AS dias, count(*)::int AS n
		FROM orders.orden WHERE taller_id = $1 AND NOT ejemplo AND estado = 'entregado' AND cerrada_en >= $2`, tallerID, since)
	if err != nil || len(rows) == 0 {
		return nil, 0, err
	}
	return rows[0].Dias, rows[0].N, nil
}

func (Stats) Returning(ctx context.Context, tallerID shared.ID, since time.Time) (int, int, error) {
	type r struct {
		Vuelven int `db:"vuelven"`
		Total   int `db:"total"`
	}
	rows, err := db.Many[r](ctx, `SELECT count(*) FILTER (WHERE n >= 2)::int AS vuelven, count(*)::int AS total FROM (
		SELECT vehiculo_id, count(*) AS n FROM orders.orden WHERE taller_id = $1 AND NOT ejemplo AND estado <> 'cancelada'
		AND created_at >= $2 GROUP BY vehiculo_id) x`, tallerID, since)
	if err != nil || len(rows) == 0 {
		return 0, 0, err
	}
	return rows[0].Vuelven, rows[0].Total, nil
}

func (Stats) DeliveredSince(ctx context.Context, tallerID shared.ID, since time.Time) ([]shared.ID, error) {
	type r struct {
		ID shared.ID `db:"id"`
	}
	rows, err := db.Many[r](ctx, `SELECT id FROM orders.orden WHERE taller_id = $1 AND NOT ejemplo AND estado = 'entregado' AND cerrada_en >= $2`,
		tallerID, since)
	out := make([]shared.ID, 0, len(rows))
	for _, x := range rows {
		out = append(out, x.ID)
	}
	return out, err
}

// StateSince devuelve desde cuándo cada orden abierta del estado dado está en él.
func (Stats) StateSince(ctx context.Context, tallerID shared.ID, state string) ([]app.Since, error) {
	type r struct {
		ID    shared.ID `db:"id"`
		Desde time.Time `db:"desde"`
	}
	rows, err := db.Many[r](ctx, `SELECT o.id, COALESCE((SELECT max(h.ocurrido_en) FROM orders.estado_historial h
		WHERE h.taller_id = o.taller_id AND h.orden_id = o.id AND h.hacia = o.estado), o.created_at) AS desde
		FROM orders.orden o WHERE o.taller_id = $1 AND o.estado = $2`, tallerID, state)
	out := make([]app.Since, 0, len(rows))
	for _, x := range rows {
		out = append(out, app.Since{OrderID: x.ID, At: x.Desde})
	}
	return out, err
}
