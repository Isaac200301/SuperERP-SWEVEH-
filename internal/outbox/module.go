// Package outbox guarda los eventos de dominio en la misma transacción que el cambio
// (docs/06 §8.6) y los despacha desde el proceso worker con FOR UPDATE SKIP LOCKED.
package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"sweveh/internal/outbox/adapters/out/postgres/sqlcgen"
	"sweveh/internal/platform/clock"
	"sweveh/internal/platform/db"
	"sweveh/internal/shared"
)

// Publisher escribe eventos en outbox.evento con la transacción del context.
type Publisher struct{}

func NewPublisher() Publisher { return Publisher{} }

// Publish falla si no hay transacción: nunca hay evento sin estado ni estado sin evento.
func (Publisher) Publish(ctx context.Context, tallerID shared.ID, e shared.Event) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(e.Payload)
	if err != nil {
		return fmt.Errorf("outbox: payload: %w", err)
	}
	if e.Payload == nil {
		payload = json.RawMessage(`{}`)
	}
	return sqlcgen.New(conn).InsertOutboxEvent(ctx, sqlcgen.InsertOutboxEventParams{
		ID:         shared.NewID(),
		TallerID:   tallerID,
		Tipo:       e.Type,
		AgregadoID: e.AggregateID,
		Payload:    payload,
		CreatedAt:  e.OccurredAt,
	})
}

// Message es un evento tomado por el worker.
type Message struct {
	ID          shared.ID
	TallerID    shared.ID
	Type        string
	AggregateID shared.ID
	Payload     json.RawMessage
	Attempts    int
}

// Handler procesa un evento. Debe ser idempotente: la entrega es "al menos una vez".
type Handler func(ctx context.Context, m Message) error

// Dispatcher reparte los eventos pendientes a sus manejadores por tipo.
type Dispatcher struct {
	pool     *pgxpool.Pool // conexión con el rol sweveh_worker
	clock    clock.Clock
	log      *slog.Logger
	handlers map[string]Handler
	batch    int32
}

func NewDispatcher(pool *pgxpool.Pool, c clock.Clock, log *slog.Logger, batch int32) *Dispatcher {
	return &Dispatcher{pool: pool, clock: c, log: log, handlers: map[string]Handler{}, batch: batch}
}

// On registra el manejador de un tipo de evento.
func (d *Dispatcher) On(eventType string, h Handler) { d.handlers[eventType] = h }

// RunOnce toma un lote, lo procesa y devuelve cuántos eventos tocó.
// Un evento sin manejador se marca procesado: hoy ningún módulo consume eventos todavía.
func (d *Dispatcher) RunOnce(ctx context.Context) (int, error) {
	n := 0
	err := pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		q := sqlcgen.New(tx)
		now := d.clock.Now()
		rows, err := q.ClaimOutboxBatch(ctx, sqlcgen.ClaimOutboxBatchParams{ProximoIntento: now, Limit: d.batch})
		if err != nil {
			return err
		}
		for _, r := range rows {
			m := Message{ID: r.ID, TallerID: r.TallerID, Type: r.Tipo, AggregateID: r.AgregadoID,
				Payload: r.Payload, Attempts: int(r.Intentos)}
			if h, ok := d.handlers[r.Tipo]; ok {
				if herr := h(ctx, m); herr != nil {
					msg := herr.Error()
					next := now.Add(backoff(m.Attempts))
					d.log.WarnContext(ctx, "outbox: fallo al procesar", "evento", r.Tipo, "intentos", m.Attempts+1, "error", msg)
					if err := q.MarkOutboxFailed(ctx, sqlcgen.MarkOutboxFailedParams{ID: r.ID, UltimoError: &msg, ProximoIntento: next}); err != nil {
						return err
					}
					continue
				}
			} else {
				d.log.InfoContext(ctx, "outbox: evento sin consumidor", "evento", r.Tipo)
			}
			if err := q.MarkOutboxProcessed(ctx, sqlcgen.MarkOutboxProcessedParams{ID: r.ID, ProcesadoEn: &now}); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	return n, err
}

// Run sondea hasta que el context se cancele (apagado limpio, docs/06 §9.3).
func (d *Dispatcher) Run(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		if _, err := d.RunOnce(ctx); err != nil && ctx.Err() == nil {
			d.log.ErrorContext(ctx, "outbox: lote fallido", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// backoff exponencial con tope de 10 minutos.
func backoff(attempts int) time.Duration {
	d := time.Second << min(attempts, 10)
	return min(d, 10*time.Minute)
}
