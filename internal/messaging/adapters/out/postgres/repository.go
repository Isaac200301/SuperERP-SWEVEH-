// Package postgres guarda canales, conversaciones, mensajes y webhooks (pgx, ADR-0024).
package postgres

import (
	"context"
	"encoding/json"
	"time"

	"sweveh/internal/messaging/app"
	"sweveh/internal/platform/db"
	"sweveh/internal/shared"
)

type Repository struct{}

var errConv = shared.NotFound("conversacion_no_encontrada", "No encontramos esa conversación.")
var errMsg = shared.NotFound("mensaje_no_encontrado", "No encontramos ese mensaje.")

func (Repository) SaveWebhook(ctx context.Context, body []byte) error {
	_, err := db.Exec(ctx, `INSERT INTO messaging.webhook_evento (id, cuerpo) VALUES ($1, $2)`, shared.NewID(), json.RawMessage(body))
	return err
}

func (Repository) PendingWebhooks(ctx context.Context, limit int) ([]app.WebhookEvent, error) {
	type row struct {
		ID     shared.ID       `db:"id"`
		Cuerpo json.RawMessage `db:"cuerpo"`
	}
	rows, err := db.Many[row](ctx, `SELECT id, cuerpo FROM messaging.webhook_evento WHERE procesado_en IS NULL
		ORDER BY recibido_en LIMIT $1 FOR UPDATE SKIP LOCKED`, limit)
	out := make([]app.WebhookEvent, 0, len(rows))
	for _, r := range rows {
		out = append(out, app.WebhookEvent{ID: r.ID, Body: r.Cuerpo})
	}
	return out, err
}

func (Repository) MarkWebhook(ctx context.Context, id shared.ID, e *string) error {
	_, err := db.Exec(ctx, `UPDATE messaging.webhook_evento SET procesado_en = now(), error = $2 WHERE id = $1`, id, e)
	return err
}

type chRow struct {
	TallerID shared.ID `db:"taller_id"`
	Prov     string    `db:"proveedor"`
	PNID     string    `db:"phone_number_id"`
	Visible  *string   `db:"numero_visible"`
}

func (r chRow) info() app.ChannelInfo {
	return app.ChannelInfo{TallerID: r.TallerID, Provider: r.Prov, PhoneNumberID: r.PNID, Visible: r.Visible}
}

func (Repository) ChannelByNumber(ctx context.Context, pnid string) (app.ChannelInfo, bool, error) {
	rows, err := db.Many[chRow](ctx, `SELECT taller_id, proveedor, phone_number_id, numero_visible FROM messaging.canal
		WHERE phone_number_id = $1 AND activo`, pnid)
	if err != nil || len(rows) == 0 {
		return app.ChannelInfo{}, false, err
	}
	return rows[0].info(), true, nil
}

func (Repository) ChannelOf(ctx context.Context, tallerID shared.ID) (app.ChannelInfo, bool, error) {
	rows, err := db.Many[chRow](ctx, `SELECT taller_id, proveedor, phone_number_id, numero_visible FROM messaging.canal
		WHERE taller_id = $1 AND activo ORDER BY created_at DESC LIMIT 1`, tallerID)
	if err != nil || len(rows) == 0 {
		return app.ChannelInfo{}, false, err
	}
	return rows[0].info(), true, nil
}

func (Repository) UpsertChannel(ctx context.Context, c app.ChannelInfo) error {
	if _, err := db.Exec(ctx, `UPDATE messaging.canal SET activo = false WHERE taller_id = $1 AND phone_number_id <> $2`, c.TallerID, c.PhoneNumberID); err != nil {
		return err
	}
	_, err := db.Exec(ctx, `INSERT INTO messaging.canal (id, taller_id, proveedor, phone_number_id, numero_visible) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (phone_number_id) DO UPDATE SET proveedor = EXCLUDED.proveedor, numero_visible = EXCLUDED.numero_visible, activo = true`,
		shared.NewID(), c.TallerID, c.Provider, c.PhoneNumberID, c.Visible)
	return err
}

type convRow struct {
	ID          shared.ID  `db:"id"`
	TallerID    shared.ID  `db:"taller_id"`
	ClienteID   *shared.ID `db:"cliente_id"`
	Celular     string     `db:"celular"`
	Ultimo      *time.Time `db:"ultimo_entrante_en"`
	Modo        string     `db:"modo"`
	PideHumano  bool       `db:"pide_humano"`
}

func (r convRow) conv() app.Conversation {
	return app.Conversation{ID: r.ID, TallerID: r.TallerID, CustomerID: r.ClienteID, Phone: r.Celular, LastInbound: r.Ultimo,
		Mode: r.Modo, WantsHuman: r.PideHumano}
}

const convCols = `id, taller_id, cliente_id, celular, ultimo_entrante_en, modo, pide_humano`

func (Repository) Conversation(ctx context.Context, tallerID shared.ID, phone string) (app.Conversation, error) {
	if _, err := db.Exec(ctx, `INSERT INTO messaging.conversacion (id, taller_id, celular) VALUES ($1, $2, $3)
		ON CONFLICT ON CONSTRAINT conversacion_celular_unica DO NOTHING`, shared.NewID(), tallerID, phone); err != nil {
		return app.Conversation{}, err
	}
	r, err := db.One[convRow](ctx, errConv, `SELECT `+convCols+` FROM messaging.conversacion WHERE taller_id = $1 AND celular = $2`, tallerID, phone)
	return r.conv(), err
}

func (Repository) ConversationByID(ctx context.Context, tallerID, id shared.ID) (app.Conversation, error) {
	r, err := db.One[convRow](ctx, errConv, `SELECT `+convCols+` FROM messaging.conversacion WHERE taller_id = $1 AND id = $2`, tallerID, id)
	return r.conv(), err
}

func (Repository) TouchInbound(ctx context.Context, tallerID, convID shared.ID, at time.Time, customerID *shared.ID) error {
	_, err := db.Exec(ctx, `UPDATE messaging.conversacion SET ultimo_entrante_en = GREATEST(COALESCE(ultimo_entrante_en, $3), $3),
		cliente_id = COALESCE($4, cliente_id) WHERE taller_id = $1 AND id = $2`, tallerID, convID, at, customerID)
	return err
}

func (Repository) SetMode(ctx context.Context, tallerID, convID shared.ID, mode string, wants bool) error {
	_, err := db.Exec(ctx, `UPDATE messaging.conversacion SET modo = $3, pide_humano = $4 WHERE taller_id = $1 AND id = $2`, tallerID, convID, mode, wants)
	return err
}

type msgRow struct {
	ID         shared.ID       `db:"id"`
	TallerID   shared.ID       `db:"taller_id"`
	ConvID     shared.ID       `db:"conversacion_id"`
	Direccion  string          `db:"direccion"`
	WAMID      *string         `db:"wamid"`
	Tipo       string          `db:"tipo"`
	Cuerpo     *string         `db:"cuerpo"`
	Payload    json.RawMessage `db:"payload"`
	Estado     string          `db:"estado"`
	Origen     string          `db:"origen"`
	Categoria  *string         `db:"categoria"`
	OrdenID    *shared.ID      `db:"orden_id"`
	Automat    *string         `db:"automatizacion"`
	Error      *string         `db:"error"`
	CreatedAt  time.Time       `db:"created_at"`
	EnviadoEn  *time.Time      `db:"enviado_en"`
}

func (r msgRow) msg() app.Message {
	m := app.Message{ID: r.ID, TallerID: r.TallerID, ConversationID: r.ConvID, Direction: r.Direccion, WAMID: r.WAMID, Kind: r.Tipo,
		Payload: r.Payload, State: r.Estado, Origin: r.Origen, Category: r.Categoria, OrderID: r.OrdenID, Automation: r.Automat,
		Error: r.Error, CreatedAt: r.CreatedAt, SentAt: r.EnviadoEn}
	if r.Cuerpo != nil {
		m.Body = *r.Cuerpo
	}
	return m
}

const msgCols = `id, taller_id, conversacion_id, direccion, wamid, tipo, cuerpo, payload, estado, origen, categoria, orden_id,
	automatizacion, error, created_at, enviado_en`

func msgs(rows []msgRow, err error) ([]app.Message, error) {
	out := make([]app.Message, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.msg())
	}
	return out, err
}

func (Repository) InsertMessage(ctx context.Context, m app.Message) (bool, error) {
	n, err := db.Exec(ctx, `INSERT INTO messaging.mensaje (`+msgCols+`) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		ON CONFLICT (wamid) WHERE wamid IS NOT NULL DO NOTHING`,
		m.ID, m.TallerID, m.ConversationID, m.Direction, m.WAMID, m.Kind, m.Body, m.Payload, m.State, m.Origin, m.Category, m.OrderID,
		m.Automation, m.Error, m.CreatedAt, m.SentAt)
	return n == 1, err
}

func (Repository) GetMessage(ctx context.Context, tallerID, id shared.ID) (app.Message, error) {
	r, err := db.One[msgRow](ctx, errMsg, `SELECT `+msgCols+` FROM messaging.mensaje WHERE taller_id = $1 AND id = $2`, tallerID, id)
	return r.msg(), err
}

func (Repository) UpdateDelivery(ctx context.Context, tallerID, id shared.ID, state string, wamid, e *string, at time.Time) error {
	_, err := db.Exec(ctx, `UPDATE messaging.mensaje SET estado = $3, wamid = COALESCE($4, wamid), error = $5, intentos = intentos + 1,
		enviado_en = CASE WHEN $3 = 'enviado' THEN $6 ELSE enviado_en END WHERE taller_id = $1 AND id = $2`, tallerID, id, state, wamid, e, at)
	return err
}

// StatusByWAMID actualiza la entrega reportada por Meta (con el taller del número ya fijado).
func (Repository) StatusByWAMID(ctx context.Context, wamid, state string) error {
	_, err := db.Exec(ctx, `UPDATE messaging.mensaje SET estado = $2 WHERE wamid = $1 AND direccion = 'saliente'`, wamid, state)
	return err
}

func (Repository) Messages(ctx context.Context, tallerID, convID shared.ID, limit int) ([]app.Message, error) {
	return msgs(db.Many[msgRow](ctx, `SELECT * FROM (SELECT `+msgCols+` FROM messaging.mensaje WHERE taller_id = $1 AND conversacion_id = $2
		ORDER BY created_at DESC LIMIT $3) x ORDER BY created_at`, tallerID, convID, limit))
}

// MessagesByOrder: los mensajes ligados a la orden más los de su cliente desde que se abrió (HU-24).
func (Repository) MessagesByOrder(ctx context.Context, tallerID, orderID shared.ID) ([]app.Message, error) {
	return msgs(db.Many[msgRow](ctx, `SELECT `+msgCols+` FROM messaging.mensaje WHERE taller_id = $1 AND orden_id = $2 ORDER BY created_at`, tallerID, orderID))
}

func (Repository) Conversations(ctx context.Context, tallerID shared.ID, limit int) ([]app.ConversationSummary, error) {
	type row struct {
		convRow
		Ultimo  *string   `db:"ultimo_cuerpo"`
		UltimoA time.Time `db:"ultimo_en"`
	}
	rows, err := db.Many[row](ctx, `SELECT c.id, c.taller_id, c.cliente_id, c.celular, c.ultimo_entrante_en, c.modo, c.pide_humano,
		m.cuerpo AS ultimo_cuerpo, m.created_at AS ultimo_en
		FROM messaging.conversacion c
		JOIN LATERAL (SELECT cuerpo, created_at FROM messaging.mensaje WHERE taller_id = c.taller_id AND conversacion_id = c.id
			ORDER BY created_at DESC LIMIT 1) m ON true
		WHERE c.taller_id = $1 ORDER BY m.created_at DESC LIMIT $2`, tallerID, limit)
	out := make([]app.ConversationSummary, 0, len(rows))
	for _, r := range rows {
		s := app.ConversationSummary{Conversation: r.conv(), LastAt: r.UltimoA}
		if r.Ultimo != nil {
			s.LastBody = *r.Ultimo
		}
		out = append(out, s)
	}
	return out, err
}

// ResponseMinutes: por cada mensaje del cliente, minutos hasta la siguiente respuesta del taller (HU-73).
func (Repository) ResponseMinutes(ctx context.Context, tallerID shared.ID, since time.Time) ([]float64, error) {
	type row struct {
		Min float64 `db:"minutos"`
	}
	rows, err := db.Many[row](ctx, `SELECT EXTRACT(EPOCH FROM (r.created_at - e.created_at)) / 60 AS minutos
		FROM messaging.mensaje e
		JOIN LATERAL (SELECT created_at FROM messaging.mensaje s WHERE s.taller_id = e.taller_id AND s.conversacion_id = e.conversacion_id
			AND s.direccion = 'saliente' AND s.origen IN ('persona', 'agente') AND s.created_at > e.created_at ORDER BY s.created_at LIMIT 1) r ON true
		WHERE e.taller_id = $1 AND e.direccion = 'entrante' AND e.created_at >= $2`, tallerID, since)
	out := make([]float64, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Min)
	}
	return out, err
}
