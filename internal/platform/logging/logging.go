// Package logging configura slog en JSON y da utilidades para no registrar
// datos prohibidos (docs/08 §6: nada de tokens, OTP, cookies ni celulares completos).
package logging

import (
	"context"
	"io"
	"log/slog"
	"strings"

	"sweveh/internal/platform/reqctx"
)

// New crea un logger JSON que agrega request_id y taller_id cuando existen en el context.
func New(w io.Writer, level slog.Level) *slog.Logger {
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})
	return slog.New(contextHandler{Handler: h})
}

type contextHandler struct{ slog.Handler }

func (h contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := reqctx.RequestIDFrom(ctx); id != "" {
		r.AddAttrs(slog.String("request_id", id))
	}
	if t, ok := reqctx.TenantFrom(ctx); ok {
		r.AddAttrs(slog.String("taller_id", t.String()))
	}
	return h.Handler.Handle(ctx, r)
}

func (h contextHandler) WithAttrs(a []slog.Attr) slog.Handler {
	return contextHandler{Handler: h.Handler.WithAttrs(a)}
}

func (h contextHandler) WithGroup(n string) slog.Handler {
	return contextHandler{Handler: h.Handler.WithGroup(n)}
}

// MaskPhone deja ver solo el indicativo y los 4 últimos dígitos: +57300***4567.
func MaskPhone(e164 string) string {
	if len(e164) < 8 {
		return "***"
	}
	return e164[:6] + strings.Repeat("*", 3) + e164[len(e164)-4:]
}
