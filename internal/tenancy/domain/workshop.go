// Package domain contiene el taller (inquilino). Sin I/O.
package domain

import (
	"strings"
	"time"

	"sweveh/internal/shared"
)

var (
	ErrWorkshopNotFound = shared.NotFound("taller_no_encontrado", "No encontramos el taller.")
	ErrWorkshopName     = shared.Validation("nombre_taller_invalido", "El nombre del taller puede tener de 1 a 120 letras.").WithField("nombre")
)

// Workshop es el taller que paga la suscripción.
type Workshop struct {
	ID        shared.ID
	Name      string
	Version   int
	CreatedAt time.Time
}

// NewWorkshop valida el nombre.
func NewWorkshop(id shared.ID, rawName string, now time.Time) (Workshop, error) {
	name := strings.Join(strings.Fields(rawName), " ")
	if name == "" || len([]rune(name)) > 120 {
		return Workshop{}, ErrWorkshopName
	}
	return Workshop{ID: id, Name: name, Version: 1, CreatedAt: now}, nil
}
