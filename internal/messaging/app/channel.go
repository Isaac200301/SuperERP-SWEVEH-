package app

import (
	"context"
	"errors"

	"sweveh/internal/messaging/domain"
)

// Channel es el puerto CanalMensajeria (RNF-07): el proveedor (Meta Cloud API, simulador local y, para la demo,
// WAHA) se elige en el composition root; cambiar de proveedor o de número no toca los casos de uso.
// Cada envío devuelve el id del mensaje en el proveedor (wamid en Meta).
type Channel interface {
	SendText(ctx context.Context, phoneNumberID, to, text string) (string, error)
	SendButtons(ctx context.Context, phoneNumberID, to, text string, buttons []domain.Button) (string, error)
	// SendTemplate envía una plantilla aprobada: la única forma de escribir fuera de la ventana de 24 h (HU-47, Q-07).
	SendTemplate(ctx context.Context, phoneNumberID, to string, t Template) (string, error)
	// SendImage envía una foto (constancia de recibo, HU-19).
	SendImage(ctx context.Context, phoneNumberID, to string, img Image) (string, error)
}

// Template es una plantilla aprobada con sus parámetros del cuerpo, en orden ({{1}}, {{2}}…).
type Template struct {
	Name     string
	Language string // p. ej. "es_CO"
	Params   []string
}

// Image es una foto para enviar: los bytes y su tipo; el texto va como pie de foto.
type Image struct {
	Data    []byte
	MIME    string
	Caption string
}

// InboundParser interpreta el cuerpo crudo del webhook del proveedor: mensajes que llegan y estados de entrega.
type InboundParser interface {
	Parse(body []byte) ([]Inbound, []Status, error)
}

// Status es una actualización de entrega de un mensaje saliente.
type Status struct {
	PhoneNumberID string
	WAMID         string
	State         string // enviado | entregado | leido | fallido
}

// ErrUnsupported: el proveedor no sabe enviar ese tipo de mensaje (p. ej. el simulador con plantillas reales).
var ErrUnsupported = errors.New("el canal de WhatsApp no soporta este tipo de mensaje")

// IsRetryable dice si un error del canal es pasajero (límite de tasa, caída del proveedor, red) y vale la pena
// reintentar. Los adaptadores marcan sus errores con un método Retryable() bool.
func IsRetryable(err error) bool {
	var r interface{ Retryable() bool }
	return errors.As(err, &r) && r.Retryable()
}
