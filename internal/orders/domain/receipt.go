package domain

import (
	"time"

	"sweveh/internal/shared"
)

// Respuestas del cliente a la constancia de recibo (HU-19).
const (
	ReceiptOK       = "conforme"  // "Está bien"
	ReceiptObserved = "observado" // "Algo no cuadra": el dueño lo revisa
)

var ErrReceiptNotReceived = shared.Rule("recibo_sin_recibir",
	"La orden todavía no se ha recibido: no hay constancia que confirmar.")

// AnswerReceipt guarda la respuesta del cliente a la constancia. Manda la primera: si el cliente vuelve a pulsar
// (o pulsa el otro botón), no cambia y devuelve false. Solo después de recibir el vehículo.
func (o *Order) AnswerReceipt(ok bool, at time.Time) (bool, error) {
	if o.State == StatePending {
		return false, ErrReceiptNotReceived
	}
	if o.ReceiptAnswer != "" {
		return false, nil
	}
	o.ReceiptAnswer = ReceiptObserved
	if ok {
		o.ReceiptAnswer = ReceiptOK
	}
	at = at.UTC()
	o.ReceiptAnsweredAt = &at
	return true, nil
}
