package wire

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"sweveh/internal/outbox"
	"sweveh/internal/platform/db"
	"sweveh/internal/platform/reqctx"
	"sweveh/internal/shared"
)

// Register conecta los consumidores del outbox. Todos son idempotentes (entrega "al menos una vez").
// Cada uno fija el taller del evento antes de tocar datos (RLS).
func Register(d *outbox.Dispatcher, s *Services) {
	with := func(fn func(ctx context.Context, m outbox.Message, p map[string]any) error) outbox.Handler {
		return func(ctx context.Context, m outbox.Message) error {
			var p map[string]any
			_ = json.Unmarshal(m.Payload, &p)
			return fn(reqctx.WithTenant(ctx, m.TallerID), m, p)
		}
	}
	str := func(p map[string]any, k string) string { s, _ := p[k].(string); return s }

	d.On("orders.estado_cambiado", with(func(ctx context.Context, m outbox.Message, p map[string]any) error {
		return s.Automations.OnStateChanged(ctx, m.TallerID, m.AggregateID, str(p, "hacia"))
	}))
	d.On("quotes.enviada", with(func(ctx context.Context, m outbox.Message, p map[string]any) error {
		oid, err := shared.ParseID(str(p, "orden_id"))
		if err != nil {
			return nil
		}
		n, _ := p["numero"].(float64) // JSON: los números llegan como float64
		return s.Automations.OnQuoteSent(ctx, m.TallerID, oid, int(n))
	}))
	d.On("inventory.bajo_minimo", with(func(ctx context.Context, m outbox.Message, p map[string]any) error {
		return s.Automations.LowStock(ctx, m.TallerID, m.AggregateID, str(p, "referencia"), "bajo:"+m.AggregateID.String()+":"+time.Now().UTC().Format("2006-01-02"))
	}))
	d.On("messaging.mensaje_recibido", with(func(ctx context.Context, m outbox.Message, _ map[string]any) error {
		return s.Assistant.HandleInbound(ctx, m.TallerID, m.AggregateID)
	}))
	d.On("messaging.enviar", with(func(ctx context.Context, m outbox.Message, _ map[string]any) error {
		return s.Messaging.Deliver(ctx, m.TallerID, m.AggregateID, m.Attempts)
	}))
	d.On("messaging.envio_fallido", with(func(ctx context.Context, m outbox.Message, p map[string]any) error {
		return s.Automations.OnDeliveryFailed(ctx, m.TallerID, m.AggregateID, str(p, "error"))
	}))
}

// Loops corre en el worker: webhooks crudos cada segundo y las tareas diarias (AU-06, AU-07) cada hora,
// con candado consultivo para que solo una instancia las ejecute (R-07).
func Loops(ctx context.Context, pool *pgxpool.Pool, s *Services, log *slog.Logger) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	lastDaily := time.Time{}
	tx := db.NewTxManager(pool)
	for {
		if _, err := s.Messaging.ProcessWebhooks(ctx); err != nil && ctx.Err() == nil {
			log.ErrorContext(ctx, "webhooks: lote fallido", "error", err)
		}
		if time.Since(lastDaily) > time.Hour {
			lastDaily = time.Now()
			if err := daily(ctx, pool, tx, s); err != nil && ctx.Err() == nil {
				log.ErrorContext(ctx, "tareas diarias fallidas", "error", err)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

func daily(ctx context.Context, pool *pgxpool.Pool, tx *db.TxManager, s *Services) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	var ok bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock(7349101)").Scan(&ok); err != nil || !ok {
		return err
	}
	defer func() { _, _ = conn.Exec(context.Background(), "SELECT pg_advisory_unlock(7349101)") }()
	var ids []shared.ID
	if err := tx.DoWithSettings(ctx, nil, func(ctx context.Context) error {
		type r struct {
			ID shared.ID `db:"id"`
		}
		rows, err := db.Many[r](ctx, `SELECT id FROM tenancy.taller`)
		for _, x := range rows {
			ids = append(ids, x.ID)
		}
		return err
	}); err != nil {
		return err
	}
	return s.Automations.Daily(ctx, ids, reqctx.WithTenant)
}
