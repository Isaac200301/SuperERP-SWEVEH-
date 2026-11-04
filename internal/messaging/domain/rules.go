// Package domain contiene las reglas de WhatsApp: ventana de 24 h y límites de botones (R-09). Sin I/O.
package domain

import (
	"time"
	"unicode/utf8"

	"sweveh/internal/shared"
)

// Window es la ventana de servicio de 24 h desde el último mensaje del cliente (HU-47).
const Window = 24 * time.Hour

// InWindow indica si se puede responder con texto libre (sin plantilla aprobada).
func InWindow(lastInbound *time.Time, now time.Time) bool {
	return lastInbound != nil && now.Sub(*lastInbound) < Window
}

// Button es un botón de respuesta rápida (máx. 3, título de máx. 20 caracteres, DR-08).
type Button struct {
	ID    string `json:"id"`
	Title string `json:"titulo"`
}

var ErrButtons = shared.Validation("botones_invalidos", "WhatsApp admite hasta 3 botones de 20 caracteres o una lista de 10 opciones de 24.")

// MaxButtons es lo que cabe como botones; de 4 a MaxListRows opciones el canal las manda como lista (HU-57).
const (
	MaxButtons  = 3
	MaxListRows = 10
)

// IsList dice si las opciones van como lista y no como botones.
func IsList(b []Button) bool { return len(b) > MaxButtons }

func ValidateButtons(b []Button) error {
	if len(b) > MaxListRows {
		return ErrButtons
	}
	max := 20
	if IsList(b) {
		max = 24
	}
	for _, x := range b {
		if x.ID == "" || x.Title == "" || utf8.RuneCountInString(x.Title) > max || len(x.ID) > 200 {
			return ErrButtons
		}
	}
	return nil
}

// Botones de sistema (el id viaja en la respuesta del cliente).
const (
	BtnConsentYes = "consent:si"
	BtnConsentNo  = "consent:no"
	BtnQuoteOK    = "quote:aprobar:"
	BtnQuoteAsk   = "quote:dudas:"
	BtnHuman      = "humano"
)
