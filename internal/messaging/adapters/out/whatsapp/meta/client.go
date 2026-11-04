// Package meta es el adaptador de WhatsApp Cloud API (Meta, oficial): envío por la Graph API y lectura del webhook.
// Referencias (docs/06 §16.1):
//   - Envío de mensajes: https://developers.facebook.com/docs/whatsapp/cloud-api/reference/messages
//   - Subida de media: https://developers.facebook.com/docs/whatsapp/cloud-api/reference/media
//   - Webhooks: https://developers.facebook.com/docs/whatsapp/cloud-api/webhooks/components
//   - Códigos de error: https://developers.facebook.com/docs/whatsapp/cloud-api/support/error-codes
package meta

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"time"

	"sweveh/internal/messaging/app"
	"sweveh/internal/messaging/domain"
)

// Client llama a la Graph API (POST /{phone_number_id}/messages y /media) con timeout propio (docs/06 §9.1).
type Client struct {
	base  string
	token string
	http  *http.Client
}

// New crea el cliente. base es p. ej. "https://graph.facebook.com/v21.0" (WHATSAPP_API_BASE).
func New(base, token string, timeout time.Duration) *Client {
	return &Client{base: strings.TrimRight(base, "/"), token: token, http: &http.Client{Timeout: timeout}}
}

var _ app.Channel = (*Client)(nil)

// APIError es una respuesta de error de la Graph API. Solo guarda el estado HTTP y el código de Meta:
// el mensaje de Meta puede traer el número o el texto del cliente y no se registra (datos personales).
type APIError struct {
	Status int
	Code   int
}

func (e *APIError) Error() string {
	return fmt.Sprintf("whatsapp: respuesta %d (código %d)", e.Status, e.Code)
}

// Retryable: límites de tasa (HTTP 429, códigos 4, 80007, 130429, 131056) y fallas del lado de Meta (5xx, 131000).
func (e *APIError) Retryable() bool {
	switch e.Code {
	case 4, 80007, 130429, 131000, 131056:
		return true
	}
	return e.Status == http.StatusTooManyRequests || e.Status >= 500
}

// Unwrap: 131047 es "re-engagement message" (más de 24 h desde el último mensaje del cliente): la misma regla
// de la ventana que el caso de uso revisa antes de enviar (HU-47).
func (e *APIError) Unwrap() error {
	if e.Code == 131047 {
		return app.ErrOutsideWindow
	}
	return nil
}

// netError es una falla de red o de timeout: siempre vale la pena reintentar.
type netError struct{ err error }

func (e netError) Error() string   { return "whatsapp: " + e.err.Error() }
func (e netError) Unwrap() error   { return e.err }
func (e netError) Retryable() bool { return true }

func to(e164 string) string { return strings.TrimPrefix(e164, "+") }

func (c *Client) SendText(ctx context.Context, pnid, dest, text string) (string, error) {
	return c.send(ctx, pnid, map[string]any{"messaging_product": "whatsapp", "recipient_type": "individual", "to": to(dest),
		"type": "text", "text": map[string]any{"body": text, "preview_url": false}})
}

func (c *Client) SendButtons(ctx context.Context, pnid, dest, text string, b []domain.Button) (string, error) {
	if err := domain.ValidateButtons(b); err != nil {
		return "", err
	}
	interactive := map[string]any{"type": "button", "body": map[string]string{"text": text}}
	if domain.IsList(b) { // más de 3 opciones: mensaje de lista (HU-57)
		rows := make([]map[string]string, 0, len(b))
		for _, x := range b {
			rows = append(rows, map[string]string{"id": x.ID, "title": x.Title})
		}
		interactive["type"] = "list"
		interactive["action"] = map[string]any{"button": "Ver opciones", "sections": []map[string]any{{"title": "Opciones", "rows": rows}}}
	} else {
		btns := make([]map[string]any, 0, len(b))
		for _, x := range b {
			btns = append(btns, map[string]any{"type": "reply", "reply": map[string]string{"id": x.ID, "title": x.Title}})
		}
		interactive["action"] = map[string]any{"buttons": btns}
	}
	return c.send(ctx, pnid, map[string]any{"messaging_product": "whatsapp", "recipient_type": "individual", "to": to(dest),
		"type": "interactive", "interactive": interactive})
}

func (c *Client) SendTemplate(ctx context.Context, pnid, dest string, t app.Template) (string, error) {
	tpl := map[string]any{"name": t.Name, "language": map[string]string{"code": t.Language}}
	if len(t.Params) > 0 {
		params := make([]map[string]string, 0, len(t.Params))
		for _, p := range t.Params {
			params = append(params, map[string]string{"type": "text", "text": p})
		}
		tpl["components"] = []map[string]any{{"type": "body", "parameters": params}}
	}
	return c.send(ctx, pnid, map[string]any{"messaging_product": "whatsapp", "recipient_type": "individual", "to": to(dest),
		"type": "template", "template": tpl})
}

// SendImage sube la foto a Meta (POST /{phone_number_id}/media) y la envía por su id: así la foto no queda
// expuesta en una URL pública del taller.
func (c *Client) SendImage(ctx context.Context, pnid, dest string, img app.Image) (string, error) {
	id, err := c.upload(ctx, pnid, img)
	if err != nil {
		return "", err
	}
	image := map[string]string{"id": id}
	if img.Caption != "" {
		image["caption"] = img.Caption
	}
	return c.send(ctx, pnid, map[string]any{"messaging_product": "whatsapp", "recipient_type": "individual", "to": to(dest),
		"type": "image", "image": image})
}

func (c *Client) send(ctx context.Context, pnid string, body any) (string, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	var out struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}
	if err := c.do(ctx, pnid+"/messages", "application/json", bytes.NewReader(b), &out); err != nil {
		return "", err
	}
	if len(out.Messages) == 0 || out.Messages[0].ID == "" {
		return "", errors.New("whatsapp: respuesta sin id de mensaje")
	}
	return out.Messages[0].ID, nil
}

func (c *Client) upload(ctx context.Context, pnid string, img app.Image) (string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("messaging_product", "whatsapp")
	_ = w.WriteField("type", img.MIME)
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", `form-data; name="file"; filename="foto"`)
	h.Set("Content-Type", img.MIME)
	part, err := w.CreatePart(h)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(img.Data); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := c.do(ctx, pnid+"/media", w.FormDataContentType(), &buf, &out); err != nil {
		return "", err
	}
	if out.ID == "" {
		return "", errors.New("whatsapp: la subida de la foto no devolvió id")
	}
	return out.ID, nil
}

func (c *Client) do(ctx context.Context, path, contentType string, body io.Reader, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/"+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", contentType)
	res, err := c.http.Do(req)
	if err != nil {
		return netError{err}
	}
	defer func() { _ = res.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	if res.StatusCode >= 300 {
		var e struct {
			Error struct {
				Code int `json:"code"`
			} `json:"error"`
		}
		_ = json.Unmarshal(raw, &e)
		return &APIError{Status: res.StatusCode, Code: e.Error.Code}
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return errors.New("whatsapp: respuesta que no es JSON")
	}
	return nil
}
