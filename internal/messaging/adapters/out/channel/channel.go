// Package channel implementa el puerto CanalMensajeria: Meta Cloud API y un canal de prueba local.
package channel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"sweveh/internal/messaging/domain"
	"sweveh/internal/shared"
)

// Router elige el canal según el número: "local-*" va al canal de prueba local; el resto, a Meta.
type Router struct {
	Meta  *Meta
	Local *Local
}

func (r Router) pick(pnid string) interface {
	SendText(context.Context, string, string, string) (string, error)
	SendButtons(context.Context, string, string, string, []domain.Button) (string, error)
} {
	if strings.HasPrefix(pnid, "local-") || r.Meta == nil {
		return r.Local
	}
	return r.Meta
}

func (r Router) SendText(ctx context.Context, pnid, to, text string) (string, error) {
	return r.pick(pnid).SendText(ctx, pnid, to, text)
}

func (r Router) SendButtons(ctx context.Context, pnid, to, text string, b []domain.Button) (string, error) {
	return r.pick(pnid).SendButtons(ctx, pnid, to, text, b)
}

// Local no envía nada fuera del servidor: deja el mensaje como enviado para el simulador de la demo.
type Local struct{}

func (Local) SendText(context.Context, string, string, string) (string, error) {
	return "local-" + shared.NewID().String(), nil
}

func (Local) SendButtons(context.Context, string, string, string, []domain.Button) (string, error) {
	return "local-" + shared.NewID().String(), nil
}

// Meta llama a WhatsApp Cloud API (POST /{phone_number_id}/messages) con timeout propio (docs/06 §9.1).
type Meta struct {
	Base  string
	Token string
	HTTP  *http.Client
}

func NewMeta(base, token string) *Meta {
	return &Meta{Base: strings.TrimRight(base, "/"), Token: token, HTTP: &http.Client{Timeout: 10 * time.Second}}
}

func (m *Meta) post(ctx context.Context, pnid string, body any) (string, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.Base+"/"+pnid+"/messages", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+m.Token)
	req.Header.Set("Content-Type", "application/json")
	res, err := m.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("whatsapp: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	if res.StatusCode >= 300 {
		return "", fmt.Errorf("whatsapp: respuesta %d", res.StatusCode) // sin eco del cuerpo: puede traer datos personales
	}
	var out struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || len(out.Messages) == 0 {
		return "", fmt.Errorf("whatsapp: respuesta sin id de mensaje")
	}
	return out.Messages[0].ID, nil
}

func to(e164 string) string { return strings.TrimPrefix(e164, "+") }

func (m *Meta) SendText(ctx context.Context, pnid, dest, text string) (string, error) {
	return m.post(ctx, pnid, map[string]any{"messaging_product": "whatsapp", "to": to(dest), "type": "text",
		"text": map[string]any{"body": text, "preview_url": false}})
}

func (m *Meta) SendButtons(ctx context.Context, pnid, dest, text string, b []domain.Button) (string, error) {
	btns := make([]map[string]any, 0, len(b))
	for _, x := range b {
		btns = append(btns, map[string]any{"type": "reply", "reply": map[string]string{"id": x.ID, "title": x.Title}})
	}
	return m.post(ctx, pnid, map[string]any{"messaging_product": "whatsapp", "to": to(dest), "type": "interactive",
		"interactive": map[string]any{"type": "button", "body": map[string]string{"text": text}, "action": map[string]any{"buttons": btns}}})
}
