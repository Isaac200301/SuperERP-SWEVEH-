// Package postgres guarda interruptores y ejecuciones de automatizaciones (pgx, ADR-0024).
package postgres

import (
	"context"
	"time"

	"sweveh/internal/automations/app"
	"sweveh/internal/platform/db"
	"sweveh/internal/shared"
)

type Repository struct{}

func (Repository) Rules(ctx context.Context, tallerID shared.ID) (map[string]app.Rule, error) {
	type row struct {
		Codigo string  `db:"codigo"`
		Activa bool    `db:"activa"`
		Texto  *string `db:"texto"`
	}
	rows, err := db.Many[row](ctx, `SELECT codigo, activa, texto FROM automation.regla WHERE taller_id = $1`, tallerID)
	out := map[string]app.Rule{}
	for _, r := range rows {
		out[r.Codigo] = app.Rule{Code: r.Codigo, Active: r.Activa, Text: r.Texto}
	}
	return out, err
}

func (Repository) SaveRule(ctx context.Context, tallerID shared.ID, r app.Rule) error {
	_, err := db.Exec(ctx, `INSERT INTO automation.regla (taller_id, codigo, activa, texto) VALUES ($1, $2, $3, $4)
		ON CONFLICT (taller_id, codigo) DO UPDATE SET activa = EXCLUDED.activa, texto = EXCLUDED.texto, updated_at = now()`,
		tallerID, r.Code, r.Active, r.Text)
	return err
}

func (Repository) RecordRun(ctx context.Context, tallerID shared.ID, code, key string, orderID, customerID *shared.ID, result, detail string, msgID *shared.ID) (bool, error) {
	var d *string
	if detail != "" {
		d = &detail
	}
	n, err := db.Exec(ctx, `INSERT INTO automation.ejecucion (id, taller_id, codigo, clave, orden_id, cliente_id, resultado, detalle, mensaje_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) ON CONFLICT ON CONSTRAINT ejecucion_unica DO NOTHING`,
		shared.NewID(), tallerID, code, key, orderID, customerID, result, d, msgID)
	return n == 1, err
}

func (Repository) HasRun(ctx context.Context, tallerID shared.ID, code, key string) (bool, error) {
	return db.Scalar[bool](ctx, `SELECT EXISTS (SELECT 1 FROM automation.ejecucion WHERE taller_id = $1 AND codigo = $2 AND clave = $3)`, tallerID, code, key)
}

func (Repository) FailRunByMessage(ctx context.Context, tallerID, msgID shared.ID, detail string) error {
	_, err := db.Exec(ctx, `UPDATE automation.ejecucion SET resultado = 'fallido', detalle = $3
		WHERE taller_id = $1 AND mensaje_id = $2`, tallerID, msgID, detail)
	return err
}

func (Repository) WeekCounts(ctx context.Context, tallerID shared.ID, since time.Time) (map[string]int, error) {
	type row struct {
		Codigo string `db:"codigo"`
		N      int    `db:"n"`
	}
	rows, err := db.Many[row](ctx, `SELECT codigo, count(*)::int AS n FROM automation.ejecucion WHERE taller_id = $1 AND created_at >= $2
		AND resultado = 'enviado' GROUP BY codigo`, tallerID, since)
	out := map[string]int{}
	for _, r := range rows {
		out[r.Codigo] = r.N
	}
	return out, err
}

func (Repository) Runs(ctx context.Context, tallerID shared.ID, limit int) ([]app.Run, error) {
	type row struct {
		Codigo    string     `db:"codigo"`
		Clave     string     `db:"clave"`
		OrdenID   *shared.ID `db:"orden_id"`
		Resultado string     `db:"resultado"`
		Detalle   *string    `db:"detalle"`
		CreatedAt time.Time  `db:"created_at"`
	}
	rows, err := db.Many[row](ctx, `SELECT codigo, clave, orden_id, resultado, detalle, created_at FROM automation.ejecucion
		WHERE taller_id = $1 ORDER BY created_at DESC LIMIT $2`, tallerID, limit)
	out := make([]app.Run, 0, len(rows))
	for _, r := range rows {
		out = append(out, app.Run{Code: r.Codigo, Key: r.Clave, OrderID: r.OrdenID, Result: r.Resultado, Detail: r.Detalle, CreatedAt: r.CreatedAt})
	}
	return out, err
}
