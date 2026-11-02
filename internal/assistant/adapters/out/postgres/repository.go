// Package postgres guarda borradores y llamadas a la IA (pgx, ADR-0024).
package postgres

import (
	"context"
	"encoding/json"
	"time"

	"sweveh/internal/assistant/app"
	"sweveh/internal/assistant/domain"
	"sweveh/internal/platform/db"
	"sweveh/internal/shared"
)

type Repository struct{}

type row struct {
	ID        shared.ID       `db:"id"`
	TallerID  shared.ID       `db:"taller_id"`
	ConvID    shared.ID       `db:"conversacion_id"`
	InboundID shared.ID       `db:"mensaje_entrante_id"`
	Intencion string          `db:"intencion"`
	Propuesto string          `db:"texto_propuesto"`
	Final     *string         `db:"texto_final"`
	Fuentes   json.RawMessage `db:"fuentes"`
	Accion    json.RawMessage `db:"accion"`
	Estado    string          `db:"estado"`
	Origen    string          `db:"origen"`
	CreatedAt time.Time       `db:"created_at"`
	Resuelto  *time.Time      `db:"resuelto_en"`
	Entrante  *string         `db:"entrante"`
	Celular   string          `db:"celular"`
}

func (r row) record() app.DraftRecord {
	d := app.DraftRecord{ID: r.ID, TallerID: r.TallerID, ConversationID: r.ConvID, InboundID: r.InboundID, Intent: r.Intencion,
		Proposed: r.Propuesto, Final: r.Final, State: r.Estado, Origin: r.Origen, CreatedAt: r.CreatedAt, ResolvedAt: r.Resuelto, Phone: r.Celular}
	_ = json.Unmarshal(r.Fuentes, &d.Sources)
	if d.Sources == nil {
		d.Sources = []domain.Source{}
	}
	if len(r.Accion) > 0 && string(r.Accion) != "null" {
		var a app.Action
		if json.Unmarshal(r.Accion, &a) == nil {
			d.Action = &a
		}
	}
	if r.Entrante != nil {
		d.InboundText = *r.Entrante
	}
	return d
}

const sel = `SELECT b.id, b.taller_id, b.conversacion_id, b.mensaje_entrante_id, b.intencion, b.texto_propuesto, b.texto_final, b.fuentes,
	b.accion, b.estado, b.origen, b.created_at, b.resuelto_en, b.texto_entrante AS entrante, b.celular
	FROM assistant.borrador b`

func (Repository) InsertDraft(ctx context.Context, d app.DraftRecord) error {
	src, _ := json.Marshal(d.Sources)
	var act any
	if d.Action != nil {
		b, _ := json.Marshal(d.Action)
		act = json.RawMessage(b)
	}
	_, err := db.Exec(ctx, `INSERT INTO assistant.borrador (id, taller_id, conversacion_id, mensaje_entrante_id, intencion, texto_propuesto,
		fuentes, accion, estado, origen, created_at, celular, texto_entrante) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT ON CONSTRAINT borrador_por_mensaje DO NOTHING`,
		d.ID, d.TallerID, d.ConversationID, d.InboundID, d.Intent, d.Proposed, json.RawMessage(src), act, d.State, d.Origin, d.CreatedAt,
		d.Phone, d.InboundText)
	return err
}

func (Repository) Draft(ctx context.Context, tallerID, id shared.ID) (app.DraftRecord, error) {
	r, err := db.One[row](ctx, app.ErrDraftNF, sel+` WHERE b.taller_id = $1 AND b.id = $2`, tallerID, id)
	return r.record(), err
}

func (Repository) Drafts(ctx context.Context, tallerID shared.ID, onlyPending bool, limit int) ([]app.DraftRecord, error) {
	rows, err := db.Many[row](ctx, sel+` WHERE b.taller_id = $1 AND (NOT $2 OR b.estado = 'pendiente') ORDER BY b.created_at DESC LIMIT $3`,
		tallerID, onlyPending, limit)
	out := make([]app.DraftRecord, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.record())
	}
	return out, err
}

func (Repository) Resolve(ctx context.Context, tallerID, id shared.ID, state string, final *string, by shared.ID, at time.Time) (bool, error) {
	n, err := db.Exec(ctx, `UPDATE assistant.borrador SET estado = $3, texto_final = $4, resuelto_por = $5, resuelto_en = $6
		WHERE taller_id = $1 AND id = $2 AND estado = 'pendiente'`, tallerID, id, state, final, by, at)
	return n == 1, err
}

func (Repository) LogCall(ctx context.Context, tallerID shared.ID, c app.AICall) error {
	_, err := db.Exec(ctx, `INSERT INTO assistant.llamada_ia (id, taller_id, proveedor, modelo, proposito, tokens_entrada, tokens_salida,
		costo_micro_usd, latencia_ms, resultado) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		shared.NewID(), tallerID, c.Provider, c.Model, c.Purpose, c.TokensIn, c.TokensOut, c.CostMicroUSD, c.LatencyMs, c.Result)
	return err
}

func (Repository) MonthCost(ctx context.Context, tallerID shared.ID, since time.Time) (int64, error) {
	return db.Scalar[int64](ctx, `SELECT COALESCE(sum(costo_micro_usd), 0)::bigint FROM assistant.llamada_ia WHERE taller_id = $1 AND created_at >= $2`, tallerID, since)
}

func (Repository) EditStats(ctx context.Context, tallerID shared.ID, since time.Time) (int, int, error) {
	type r struct {
		Enviados int `db:"enviados"`
		Editados int `db:"editados"`
	}
	rows, err := db.Many[r](ctx, `SELECT count(*) FILTER (WHERE estado IN ('enviado', 'editado_enviado'))::int AS enviados,
		count(*) FILTER (WHERE estado = 'editado_enviado')::int AS editados FROM assistant.borrador WHERE taller_id = $1 AND created_at >= $2`, tallerID, since)
	if err != nil || len(rows) == 0 {
		return 0, 0, err
	}
	return rows[0].Enviados, rows[0].Editados, nil
}
