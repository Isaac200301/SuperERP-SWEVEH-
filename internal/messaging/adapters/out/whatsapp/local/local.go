// Package local es el canal de prueba: no envía nada fuera del servidor. Cada mensaje queda guardado como
// enviado y el simulador de la web lo muestra (demo y pruebas sin cuenta de Meta). Prohibido en producción
// (internal/platform/config).
package local

import (
	"context"

	"sweveh/internal/messaging/app"
	"sweveh/internal/messaging/domain"
	"sweveh/internal/shared"
)

// Prefix marca los números del simulador ("local-…"): el enrutador manda sus mensajes a este canal.
const Prefix = "local-"

type Channel struct{}

var _ app.Channel = Channel{}

func id() string { return Prefix + shared.NewID().String() }

func (Channel) SendText(context.Context, string, string, string) (string, error) { return id(), nil }

func (Channel) SendButtons(_ context.Context, _, _, _ string, b []domain.Button) (string, error) {
	if err := domain.ValidateButtons(b); err != nil {
		return "", err
	}
	return id(), nil
}

func (Channel) SendTemplate(context.Context, string, string, app.Template) (string, error) {
	return id(), nil
}

func (Channel) SendImage(context.Context, string, string, app.Image) (string, error) {
	return id(), nil
}
