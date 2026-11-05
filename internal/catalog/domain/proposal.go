package domain

import (
	"strings"
	"time"

	"sweveh/internal/shared"
)

// Lista de precios por foto (HU-93, ADR-0032): cada fila que lee la IA es una propuesta. No es un servicio hasta que el
// dueño la aprueba (con sus correcciones); así un dato inventado por la IA nunca llega solo al catálogo.

var (
	ErrProposalNotFound = shared.NotFound("propuesta_no_encontrada", "No encontramos esa fila de la lista.")
	ErrProposalResolved = shared.Conflict("propuesta_resuelta", "Esa fila ya se aprobó o se descartó.")
	ErrNoRows           = shared.Rule("lista_sin_filas", "No encontramos servicios en la foto. Prueba con una foto más cerca y con buena luz.")
)

// MaxProposals es el máximo de filas que se leen de una foto.
const MaxProposals = 40

// Proposal es una fila leída de la foto de la lista de precios.
type Proposal struct {
	ID, TallerID, BatchID shared.ID
	PhotoKey, PhotoType   string
	Name                  string
	Price                 *int64
	Kind                  string // carro | moto | ambos
	State                 string // pendiente | aprobada | descartada
	ServiceID             *shared.ID
	CreatedAt             time.Time
}

// Row es lo que devuelve la lectura de la foto, antes de validarlo, y también la corrección del dueño al aprobar.
type Row struct {
	Name  string
	Price *int64
	Kind  string
}

// Clean deja una fila lista para guardarse como propuesta: nombre limpio, tipo válido (si no, "ambos") y precio no
// negativo. Las filas sin nombre se descartan (ok=false).
func (r Row) Clean() (Row, bool) {
	r.Name = strings.Join(strings.Fields(r.Name), " ")
	if r.Name == "" || len([]rune(r.Name)) > 120 {
		return r, false
	}
	if !ValidKind(r.Kind) {
		r.Kind = kindBoth
	}
	if r.Price != nil && *r.Price < 0 {
		r.Price = nil
	}
	return r, true
}
