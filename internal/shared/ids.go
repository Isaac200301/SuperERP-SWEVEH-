// Package shared contiene solo tipos neutros que todos los módulos pueden usar:
// identificadores, errores base y eventos. No contiene reglas de negocio.
package shared

import "github.com/google/uuid"

// ID es el identificador de cualquier entidad (UUIDv7, docs/08 §4).
type ID = uuid.UUID

// NilID es el identificador vacío.
var NilID = uuid.Nil

// NewID genera un UUIDv7: ordenado en el tiempo, mejor localidad de índice.
func NewID() ID {
	id, err := uuid.NewV7()
	if err != nil {
		// NewV7 solo falla si el sistema no entrega aleatoriedad; no hay forma segura de seguir.
		panic("shared: no se pudo generar un UUIDv7: " + err.Error())
	}
	return id
}

// ParseID convierte texto a ID.
func ParseID(s string) (ID, error) {
	return uuid.Parse(s)
}
