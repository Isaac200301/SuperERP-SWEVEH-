package postgres

import (
	"context"
	"fmt"
	"time"

	"sweveh/internal/orders/app"
	"sweveh/internal/platform/db"
	"sweveh/internal/shared"
)

// Stats implementa app.Stats: cifras de órdenes para los KPIs (HU-68, HU-70, HU-72) y avisos (AU-07).
// Las órdenes de ejemplo del registro (HU-05) solo cuentan con Filter.Example (el tablero las marca, HU-71).
type Stats struct{}

// filtro: las reales y, si se piden, también las de ejemplo; y solo un tipo de vehículo si viene (HU-68, HU-70, HU-71).
const filtro = ` AND (NOT ejemplo OR $%d) AND ($%d = '' OR tipo_vehiculo = $%d)`

func where(n int) string { return fmt.Sprintf(filtro, n, n+1, n+1) }

func (Stats) StateCounts(ctx context.Context, tallerID shared.ID, f app.Filter) (map[string]int, error) {
	type r struct {
		Estado string `db:"estado"`
		N      int    `db:"n"`
	}
	rows, err := db.Many[r](ctx, `SELECT estado, count(*)::int AS n FROM orders.orden WHERE taller_id = $1`+where(2)+`
		AND estado NOT IN ('entregado', 'cancelada') GROUP BY estado`, tallerID, f.Example, f.Kind)
	out := map[string]int{}
	for _, x := range rows {
		out[x.Estado] = x.N
	}
	return out, err
}

func (Stats) Cycles(ctx context.Context, tallerID shared.ID, since time.Time, f app.Filter) ([]app.Cycle, error) {
	type r struct {
		Abierta time.Time `db:"created_at"`
		Cerrada time.Time `db:"cerrada_en"`
	}
	rows, err := db.Many[r](ctx, `SELECT created_at, cerrada_en FROM orders.orden WHERE taller_id = $1 AND estado = 'entregado'
		AND cerrada_en >= $2`+where(3), tallerID, since, f.Example, f.Kind)
	out := make([]app.Cycle, 0, len(rows))
	for _, x := range rows {
		out = append(out, app.Cycle{Opened: x.Abierta, Closed: x.Cerrada})
	}
	return out, err
}

func (Stats) Returning(ctx context.Context, tallerID shared.ID, since time.Time, f app.Filter) (int, int, error) {
	type r struct {
		Vuelven int `db:"vuelven"`
		Total   int `db:"total"`
	}
	rows, err := db.Many[r](ctx, `SELECT count(*) FILTER (WHERE n >= 2)::int AS vuelven, count(*)::int AS total FROM (
		SELECT vehiculo_id, count(*) AS n FROM orders.orden WHERE taller_id = $1 AND estado <> 'cancelada'
		AND created_at >= $2`+where(3)+` GROUP BY vehiculo_id) x`, tallerID, since, f.Example, f.Kind)
	if err != nil || len(rows) == 0 {
		return 0, 0, err
	}
	return rows[0].Vuelven, rows[0].Total, nil
}

func (Stats) DeliveredSince(ctx context.Context, tallerID shared.ID, since time.Time, f app.Filter) ([]shared.ID, error) {
	type r struct {
		ID shared.ID `db:"id"`
	}
	rows, err := db.Many[r](ctx, `SELECT id FROM orders.orden WHERE taller_id = $1 AND estado = 'entregado' AND cerrada_en >= $2`+where(3),
		tallerID, since, f.Example, f.Kind)
	out := make([]shared.ID, 0, len(rows))
	for _, x := range rows {
		out = append(out, x.ID)
	}
	return out, err
}

func (Stats) HasExample(ctx context.Context, tallerID shared.ID) (bool, error) {
	return db.Scalar[bool](ctx, `SELECT EXISTS (SELECT 1 FROM orders.orden WHERE taller_id = $1 AND ejemplo)`, tallerID)
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
