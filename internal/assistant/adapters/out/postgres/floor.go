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

// Floor guarda las conversaciones del asistente del piso (R8, migración 00036). Usa el mismo registro de llamadas a
// la IA y el mismo tope mensual que el asesor (Repository).
type Floor struct{ Repository }

var _ app.FloorRepository = Floor{}

type chatRow struct {
	ID         shared.ID       `db:"id"`
	TallerID   shared.ID       `db:"taller_id"`
	UsuarioID  shared.ID       `db:"usuario_id"`
	OrdenID    *shared.ID      `db:"orden_id"`
	Propuesta  json.RawMessage `db:"propuesta"`
	Version    int             `db:"version"`
	CreadoEn   time.Time       `db:"creado_en"`
	Actualizad time.Time       `db:"actualizado_en"`
}

const chatCols = `id, taller_id, usuario_id, orden_id, propuesta, version, creado_en, actualizado_en`

func (r chatRow) chat() (app.FloorChat, error) {
	c := app.FloorChat{ID: r.ID, TallerID: r.TallerID, UserID: r.UsuarioID, OrderID: r.OrdenID, Version: r.Version,
		CreatedAt: r.CreadoEn, UpdatedAt: r.Actualizad}
	if len(r.Propuesta) > 0 {
		if err := json.Unmarshal(r.Propuesta, &c.Proposal); err != nil {
			return c, err
		}
	}
	return c, nil
}

var errChat = shared.NotFound("chat_no_encontrado", "No encontramos esa conversación.")

func (Floor) InsertChat(ctx context.Context, c app.FloorChat) error {
	raw, err := json.Marshal(c.Proposal)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, `INSERT INTO assistant.chat_piso (`+chatCols+`) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		c.ID, c.TallerID, c.UserID, c.OrderID, raw, c.Version, c.CreatedAt, c.UpdatedAt)
	return err
}

func (Floor) Chat(ctx context.Context, tallerID, id shared.ID) (app.FloorChat, error) {
	r, err := db.One[chatRow](ctx, errChat, `SELECT `+chatCols+` FROM assistant.chat_piso WHERE taller_id = $1 AND id = $2`, tallerID, id)
	if err != nil {
		return app.FloorChat{}, err
	}
	return r.chat()
}

func (Floor) ChatForOrder(ctx context.Context, tallerID, orderID shared.ID) (*app.FloorChat, error) {
	rows, err := db.Many[chatRow](ctx, `SELECT `+chatCols+` FROM assistant.chat_piso WHERE taller_id = $1 AND orden_id = $2
		ORDER BY actualizado_en DESC LIMIT 1`, tallerID, orderID)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	c, err := rows[0].chat()
	return &c, err
}

func (Floor) LatestForUser(ctx context.Context, tallerID, userID shared.ID, since time.Time) (*app.FloorChat, error) {
	rows, err := db.Many[chatRow](ctx, `SELECT `+chatCols+` FROM assistant.chat_piso WHERE taller_id = $1 AND usuario_id = $2
		AND actualizado_en >= $3 ORDER BY actualizado_en DESC LIMIT 1`, tallerID, userID, since)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	c, err := rows[0].chat()
	return &c, err
}

// UpdateChat guarda la propuesta y la orden con versión optimista (R-07): false si otra persona la cambió antes.
func (Floor) UpdateChat(ctx context.Context, c app.FloorChat, prevVersion int) (bool, error) {
	raw, err := json.Marshal(c.Proposal)
	if err != nil {
		return false, err
	}
	n, err := db.Exec(ctx, `UPDATE assistant.chat_piso SET propuesta = $3, orden_id = $4, version = $5, actualizado_en = $6
		WHERE taller_id = $1 AND id = $2 AND version = $7`, c.TallerID, c.ID, raw, c.OrderID, c.Version, c.UpdatedAt, prevVersion)
	return n == 1, err
}

type msgRow struct {
	ID        shared.ID       `db:"id"`
	ChatID    shared.ID       `db:"chat_id"`
	Rol       string          `db:"rol"`
	Texto     string          `db:"texto"`
	Adjuntos  json.RawMessage `db:"adjuntos"`
	Opciones  json.RawMessage `db:"opciones"`
	Propuesta json.RawMessage `db:"propuesta"`
	CreadoEn  time.Time       `db:"creado_en"`
}

func (Floor) InsertMessage(ctx context.Context, tallerID shared.ID, m app.FloorMessage) error {
	adj, err := json.Marshal(nonNil(m.Attachments))
	if err != nil {
		return err
	}
	opt, err := json.Marshal(nonNil(m.Choices))
	if err != nil {
		return err
	}
	var prop []byte
	if m.Proposal != nil {
		if prop, err = json.Marshal(m.Proposal); err != nil {
			return err
		}
	}
	_, err = db.Exec(ctx, `INSERT INTO assistant.chat_piso_mensaje (id, taller_id, chat_id, rol, texto, adjuntos, opciones, propuesta, creado_en)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`, m.ID, tallerID, m.ChatID, m.Role, m.Text, adj, opt, prop, m.CreatedAt)
	return err
}

func (Floor) MessageChat(ctx context.Context, tallerID, msgID shared.ID) (shared.ID, bool, error) {
	rows, err := db.Many[struct {
		ChatID shared.ID `db:"chat_id"`
	}](ctx, `SELECT chat_id FROM assistant.chat_piso_mensaje WHERE taller_id = $1 AND id = $2`, tallerID, msgID)
	if err != nil || len(rows) == 0 {
		return shared.NilID, false, err
	}
	return rows[0].ChatID, true, nil
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// Messages devuelve los últimos limit mensajes, del más viejo al más nuevo.
func (Floor) Messages(ctx context.Context, tallerID, chatID shared.ID, limit int) ([]app.FloorMessage, error) {
	rows, err := db.Many[msgRow](ctx, `SELECT * FROM (SELECT id, chat_id, rol, texto, adjuntos, opciones, propuesta, creado_en
		FROM assistant.chat_piso_mensaje WHERE taller_id = $1 AND chat_id = $2 ORDER BY creado_en DESC, id DESC LIMIT $3) m
		ORDER BY creado_en, id`, tallerID, chatID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]app.FloorMessage, 0, len(rows))
	for _, r := range rows {
		m := app.FloorMessage{ID: r.ID, ChatID: r.ChatID, Role: r.Rol, Text: r.Texto, CreatedAt: r.CreadoEn}
		_ = json.Unmarshal(r.Adjuntos, &m.Attachments)
		_ = json.Unmarshal(r.Opciones, &m.Choices)
		if len(r.Propuesta) > 0 && string(r.Propuesta) != "null" {
			var p domain.FloorProposal
			if json.Unmarshal(r.Propuesta, &p) == nil {
				m.Proposal = &p
			}
		}
		out = append(out, m)
	}
	return out, nil
}

func (Floor) Confirmation(ctx context.Context, tallerID shared.ID, key string) (json.RawMessage, shared.ID, bool, error) {
	rows, err := db.Many[struct {
		Resultado json.RawMessage `db:"resultado"`
		ChatID    shared.ID       `db:"chat_id"`
	}](ctx, `SELECT resultado, chat_id FROM assistant.chat_piso_confirmacion WHERE taller_id = $1 AND clave = $2`, tallerID, key)
	if err != nil || len(rows) == 0 {
		return nil, shared.NilID, false, err
	}
	return rows[0].Resultado, rows[0].ChatID, true, nil
}

func (Floor) InsertConfirmation(ctx context.Context, tallerID, chatID shared.ID, key string, by shared.ID, result json.RawMessage, at time.Time) (bool, error) {
	n, err := db.Exec(ctx, `INSERT INTO assistant.chat_piso_confirmacion (taller_id, clave, chat_id, resultado, creado_por, creado_en)
		VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (taller_id, clave) DO NOTHING`, tallerID, key, chatID, result, by, at)
	return n == 1, err
}

// AnonymizeOrders borra los textos de las conversaciones del piso de esas órdenes (HU-87: pueden traer el nombre o el
// celular del titular). La propuesta y las fotos del chat también se quitan.
func (Floor) AnonymizeOrders(ctx context.Context, tallerID shared.ID, orders []shared.ID) (int, error) {
	if len(orders) == 0 {
		return 0, nil
	}
	n, err := db.Exec(ctx, `UPDATE assistant.chat_piso_mensaje m SET texto = '(borrado a pedido del titular)', adjuntos = '[]', propuesta = NULL
		FROM assistant.chat_piso c WHERE c.id = m.chat_id AND c.taller_id = $1 AND m.taller_id = $1 AND c.orden_id = ANY($2)`, tallerID, orders)
	if err != nil {
		return 0, err
	}
	_, err = db.Exec(ctx, `UPDATE assistant.chat_piso SET propuesta = '{}' WHERE taller_id = $1 AND orden_id = ANY($2)`, tallerID, orders)
	return int(n), err
}
