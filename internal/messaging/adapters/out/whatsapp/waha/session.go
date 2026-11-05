package waha

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
)

// Estado de la sesión en WAHA: STOPPED, STARTING, SCAN_QR_CODE, WORKING o FAILED. NOT_FOUND si no existe.
type SessionInfo struct {
	Status string `json:"status"`
	Me     *struct {
		ID       string `json:"id"`
		PushName string `json:"pushName"`
	} `json:"me"`
}

// SessionStatus consulta la sesión (GET /api/sessions/{session}).
func (c *Client) SessionStatus(ctx context.Context, session string) (SessionInfo, error) {
	var out SessionInfo
	err := c.do(ctx, http.MethodGet, "/api/sessions/"+url.PathEscape(session), nil, &out)
	var e *APIError
	if errors.As(err, &e) && e.Status == http.StatusNotFound {
		return SessionInfo{Status: "NOT_FOUND"}, nil
	}
	return out, err
}

// StartSession crea y arranca la sesión; si ya existe, solo la arranca. Después hay que escanear el QR.
func (c *Client) StartSession(ctx context.Context, session string) error {
	st, err := c.SessionStatus(ctx, session)
	if err != nil {
		return err
	}
	switch st.Status {
	case "NOT_FOUND":
		return c.do(ctx, http.MethodPost, "/api/sessions", map[string]any{"name": session, "start": true}, nil)
	case "STOPPED", "FAILED":
		return c.do(ctx, http.MethodPost, "/api/sessions/"+url.PathEscape(session)+"/start", nil, nil)
	}
	return nil // ya arrancando, esperando QR o funcionando
}

// QR devuelve la imagen PNG del código para vincular el número (GET /api/{session}/auth/qr). El QR cambia cada
// 20 a 60 segundos: la pantalla lo vuelve a pedir.
func (c *Client) QR(ctx context.Context, session string) ([]byte, error) {
	var out struct {
		Data string `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/"+url.PathEscape(session)+"/auth/qr", nil, &out); err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(out.Data)
}
