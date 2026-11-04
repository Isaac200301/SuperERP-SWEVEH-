// Package waha es el canal de WhatsApp no oficial para la demo (WAHA, ADR-0027): un número de prueba vinculado
// con QR. Solo demo, nunca con datos reales; config no lo deja arrancar en producción.
// Referencias (consultadas el 2026-11-04):
//   - Envío: https://waha.devlike.pro/docs/how-to/send-messages/
//   - Webhooks y firma: https://waha.devlike.pro/docs/how-to/events/
//
// WAHA no puede mandar botones ("DEPRECATED - Buttons do not work at the moment"): las opciones salen como menú
// numerado (domain.Menu) y messaging/app reconoce la respuesta (domain.MatchReply).
package waha

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"sweveh/internal/messaging/app"
	"sweveh/internal/messaging/domain"
)

// Prefix marca el número del taller conectado por WAHA: "waha:<sesión>". El enrutador manda sus mensajes aquí.
const Prefix = "waha:"

// Client llama a la API de WAHA con su clave (X-Api-Key). La clave nunca sale en errores ni en logs.
type Client struct {
	base, key string
	http      *http.Client
}

// New crea el cliente. base es p. ej. "http://localhost:3000" (WAHA_URL).
func New(base, key string, timeout time.Duration) *Client {
	return &Client{base: strings.TrimRight(base, "/"), key: key, http: &http.Client{Timeout: timeout}}
}

var _ app.Channel = (*Client)(nil)

// APIError es una respuesta no exitosa de WAHA: solo el código HTTP, sin eco del cuerpo (puede traer el número).
type APIError struct{ Status int }

func (e *APIError) Error() string { return fmt.Sprintf("waha: respuesta %d", e.Status) }

// Retryable: límite de tasa o falla de WAHA (sesión reiniciando, WhatsApp caído).
func (e *APIError) Retryable() bool { return e.Status == http.StatusTooManyRequests || e.Status >= 500 }

type netError struct{ err error }

func (e netError) Error() string   { return "waha: " + e.err.Error() }
func (e netError) Unwrap() error   { return e.err }
func (e netError) Retryable() bool { return true }

// Session saca la sesión de WAHA del número del taller ("waha:default" → "default").
func Session(pnid string) string { return strings.TrimPrefix(pnid, Prefix) }

// ChatID pasa el celular a chat de WAHA: "+573001234567" → "573001234567@c.us". Un id que ya trae dominio
// (p. ej. "…@lid") se usa tal cual: es el chat que escribió.
func ChatID(phone string) string {
	if strings.Contains(phone, "@") {
		return phone
	}
	return strings.TrimPrefix(phone, "+") + "@c.us"
}

func (c *Client) SendText(ctx context.Context, pnid, to, text string) (string, error) {
	return c.send(ctx, "/api/sendText", map[string]any{"session": Session(pnid), "chatId": ChatID(to), "text": text})
}

// SendButtons manda las opciones como menú numerado: WAHA no tiene botones que funcionen.
func (c *Client) SendButtons(ctx context.Context, pnid, to, text string, b []domain.Button) (string, error) {
	if err := domain.ValidateButtons(b); err != nil {
		return "", err
	}
	return c.SendText(ctx, pnid, to, domain.Menu(text, b))
}

// SendTemplate: WAHA no tiene plantillas aprobadas por Meta.
func (c *Client) SendTemplate(context.Context, string, string, app.Template) (string, error) {
	return "", app.ErrUnsupported
}

func (c *Client) SendImage(ctx context.Context, pnid, to string, img app.Image) (string, error) {
	return c.send(ctx, "/api/sendImage", map[string]any{"session": Session(pnid), "chatId": ChatID(to), "caption": img.Caption,
		"file": map[string]string{"mimetype": img.MIME, "filename": "foto", "data": base64.StdEncoding.EncodeToString(img.Data)}})
}

func (c *Client) send(ctx context.Context, path string, body any) (string, error) {
	var out struct {
		ID json.RawMessage `json:"id"`
	}
	if err := c.do(ctx, http.MethodPost, path, body, &out); err != nil {
		return "", err
	}
	if id := messageID(out.ID); id != "" {
		return id, nil
	}
	return "", errors.New("waha: respuesta sin id de mensaje")
}

// messageID lee el id del mensaje: según el motor llega como texto ("true_57…@c.us_ABC") o como objeto con
// _serialized. Es el mismo id que trae después message.ack.
func messageID(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var o struct {
		Serialized string `json:"_serialized"`
	}
	if json.Unmarshal(raw, &o) == nil {
		return o.Serialized
	}
	return ""
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, r)
	if err != nil {
		return err
	}
	req.Header.Set("X-Api-Key", c.key)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	if err != nil {
		return netError{err}
	}
	defer func() { _ = res.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		return &APIError{Status: res.StatusCode}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return errors.New("waha: respuesta que no es JSON")
	}
	return nil
}
