// Package audit es la bitácora de negocio solo-inserción (HU-88, docs/06 §10.4).
// Se escribe en la misma transacción que el cambio que registra.
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"sweveh/internal/audit/adapters/out/postgres/sqlcgen"
	"sweveh/internal/platform/db"
	"sweveh/internal/platform/reqctx"
	"sweveh/internal/shared"
)

// Entry es un registro de la bitácora: quién, qué, valor anterior, valor nuevo y cuándo.
type Entry struct {
	TallerID   shared.ID
	Actor      shared.Actor
	Action     string // por ejemplo "orden.crear"
	Entity     string // por ejemplo "orden"
	EntityID   shared.ID
	Before     any
	After      any
	OccurredAt time.Time
}

// Log escribe en audit.evento usando la transacción del context.
type Log struct{}

func NewLog() Log { return Log{} }

// Record inserta el registro. Falla si no hay transacción: la auditoría nunca va suelta.
func (Log) Record(ctx context.Context, e Entry) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	before, err := marshalOrNil(e.Before)
	if err != nil {
		return fmt.Errorf("audit: antes: %w", err)
	}
	after, err := marshalOrNil(e.After)
	if err != nil {
		return fmt.Errorf("audit: después: %w", err)
	}
	var actorID *shared.ID
	if e.Actor.ID != shared.NilID {
		id := e.Actor.ID
		actorID = &id
	}
	var reqID *string
	if r := reqctx.RequestIDFrom(ctx); r != "" {
		reqID = &r
	}
	return sqlcgen.New(conn).InsertAuditEvent(ctx, sqlcgen.InsertAuditEventParams{
		ID:         shared.NewID(),
		TallerID:   e.TallerID,
		ActorTipo:  string(e.Actor.Type),
		ActorID:    actorID,
		Accion:     e.Action,
		Entidad:    e.Entity,
		EntidadID:  e.EntityID,
		Antes:      before,
		Despues:    after,
		RequestID:  reqID,
		OcurridoEn: e.OccurredAt,
	})
}

// Record es la vista de lectura de un registro.
type Record struct {
	ID         shared.ID
	ActorType  string
	ActorID    *shared.ID
	Action     string
	Before     json.RawMessage
	After      json.RawMessage
	OccurredAt time.Time
}

// ListByEntity devuelve la bitácora de una entidad del taller actual.
func (Log) ListByEntity(ctx context.Context, tallerID shared.ID, entity string, id shared.ID) ([]Record, error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := sqlcgen.New(conn).ListAuditByEntity(ctx, sqlcgen.ListAuditByEntityParams{
		TallerID: tallerID, Entidad: entity, EntidadID: id,
	})
	if err != nil {
		return nil, err
	}
	out := make([]Record, 0, len(rows))
	for _, r := range rows {
		out = append(out, Record{
			ID: r.ID, ActorType: r.ActorTipo, ActorID: r.ActorID, Action: r.Accion,
			Before: r.Antes, After: r.Despues, OccurredAt: r.OcurridoEn,
		})
	}
	return out, nil
}

// Filter filtra la bitácora (HU-88, criterio 2).
type Filter struct {
	Entity string
	UserID string
	Since  *time.Time
	Limit  int
}

// FullRecord es un registro con su entidad.
type FullRecord struct {
	Record
	Entity   string
	EntityID shared.ID
}

// List lee la bitácora del taller con filtros, en la transacción del llamador.
func (Log) List(ctx context.Context, tallerID shared.ID, f Filter) ([]FullRecord, error) {
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 200
	}
	var uid *shared.ID
	if id, err := shared.ParseID(f.UserID); err == nil {
		uid = &id
	}
	var ent *string
	if f.Entity != "" {
		ent = &f.Entity
	}
	type row struct {
		ID         shared.ID       `db:"id"`
		ActorTipo  string          `db:"actor_tipo"`
		ActorID    *shared.ID      `db:"actor_id"`
		Accion     string          `db:"accion"`
		Entidad    string          `db:"entidad"`
		EntidadID  shared.ID       `db:"entidad_id"`
		Antes      json.RawMessage `db:"antes"`
		Despues    json.RawMessage `db:"despues"`
		OcurridoEn time.Time       `db:"ocurrido_en"`
	}
	rows, err := db.Many[row](ctx, `SELECT id, actor_tipo, actor_id, accion, entidad, entidad_id, antes, despues, ocurrido_en
		FROM audit.evento WHERE taller_id = $1 AND ($2::text IS NULL OR entidad = $2) AND ($3::uuid IS NULL OR actor_id = $3)
		AND ($4::timestamptz IS NULL OR ocurrido_en >= $4) ORDER BY ocurrido_en DESC LIMIT $5`, tallerID, ent, uid, f.Since, f.Limit)
	out := make([]FullRecord, 0, len(rows))
	for _, r := range rows {
		out = append(out, FullRecord{Record: Record{ID: r.ID, ActorType: r.ActorTipo, ActorID: r.ActorID, Action: r.Accion,
			Before: r.Antes, After: r.Despues, OccurredAt: r.OcurridoEn}, Entity: r.Entidad, EntityID: r.EntidadID})
	}
	return out, err
}

func marshalOrNil(v any) (json.RawMessage, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}
