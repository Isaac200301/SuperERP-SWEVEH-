// Package domain contiene la cotización y su aprobación (HU-27 a HU-31). Sin I/O.
package domain

import (
	"strings"
	"time"

	"sweveh/internal/shared"
)

type State string

const (
	Draft    State = "borrador"
	Sent     State = "enviada"
	Approved State = "aprobada"
	Rejected State = "rechazada"
	Doubts   State = "dudas"
)

var (
	ErrNotFound     = shared.NotFound("cotizacion_no_encontrada", "No encontramos la cotización de esa orden.")
	ErrLineNotFound = shared.NotFound("linea_no_encontrada", "No encontramos ese ítem en la cotización.")
	ErrNotEditable  = shared.Rule("cotizacion_no_editable", "La cotización ya se envió; ya no se puede editar.")
	ErrEmpty        = shared.Rule("cotizacion_vacia", "Agrega al menos un servicio o repuesto antes de enviar.")
	ErrNotSent      = shared.Rule("cotizacion_no_enviada", "La cotización no está esperando respuesta del cliente.")
	ErrLine         = shared.Validation("linea_invalida", "Cada ítem necesita descripción, cantidad mayor que cero y precio no negativo.")
	ErrOrderStage   = shared.Rule("orden_no_cotizable", "Solo se cotiza con la orden en diagnóstico (o una adicional en reparación).")
	ErrNoApproved   = shared.Validation("sin_items_aprobados", "Marca al menos un ítem aprobado o rechaza la cotización.")
)

// Line es un ítem con el precio de venta congelado al cotizar (HU-07, criterio 3).
type Line struct {
	ID          shared.ID
	Kind        string // servicio | repuesto
	RefID       *shared.ID
	Description string
	Qty         int
	UnitPrice   int64
	NoStock     bool
	Approved    *bool
}

func (l Line) Subtotal() int64 { return int64(l.Qty) * l.UnitPrice }

type Quote struct {
	ID          shared.ID
	TallerID    shared.ID
	OrderID     shared.ID
	Number      int
	State       State
	Lines       []Line
	SentAt      *time.Time
	RespondedAt *time.Time
	Channel     *string
	ResponseRef *string
	Version     int
}

// Total en pesos sin decimales. Si hay respuesta ítem por ítem, solo cuenta lo aprobado.
func (q Quote) Total() int64 {
	var t int64
	for _, l := range q.Lines {
		if l.Approved != nil && !*l.Approved {
			continue
		}
		t += l.Subtotal()
	}
	return t
}

func (q *Quote) AddLine(l Line) error {
	if q.State != Draft {
		return ErrNotEditable
	}
	l.Description = strings.Join(strings.Fields(l.Description), " ")
	if l.Description == "" || len([]rune(l.Description)) > 200 || l.Qty <= 0 || l.UnitPrice < 0 ||
		(l.Kind != "servicio" && l.Kind != "repuesto") {
		return ErrLine
	}
	q.Lines = append(q.Lines, l)
	return nil
}

func (q *Quote) RemoveLine(id shared.ID) error {
	if q.State != Draft {
		return ErrNotEditable
	}
	for i, l := range q.Lines {
		if l.ID == id {
			q.Lines = append(q.Lines[:i], q.Lines[i+1:]...)
			return nil
		}
	}
	return ErrLineNotFound
}

// Send la deja esperando respuesta del cliente.
func (q *Quote) Send(at time.Time) error {
	if q.State != Draft {
		return ErrNotEditable
	}
	if len(q.Lines) == 0 {
		return ErrEmpty
	}
	q.State, q.SentAt = Sent, &at
	return nil
}

// Response es la respuesta del cliente o la que registra el dueño (HU-28, HU-29, HU-30).
type Response struct {
	Decision string             // aprobar | rechazar | dudas
	Items    map[shared.ID]bool // vacío = toda la cotización (D-11: completa en el MVP)
	Channel  string             // whatsapp | telefono | mostrador
	Ref      string             // id del mensaje de WhatsApp, para idempotencia
	At       time.Time
}

// Respond aplica la respuesta. Devuelve true si quedó aprobada (total o parcial).
func (q *Quote) Respond(r Response) (bool, error) {
	if q.State != Sent && q.State != Doubts {
		return false, ErrNotSent
	}
	ch, ref := r.Channel, r.Ref
	q.Channel, q.RespondedAt = &ch, &r.At
	if ref != "" {
		q.ResponseRef = &ref
	}
	switch r.Decision {
	case "rechazar":
		q.State = Rejected
		return false, nil
	case "dudas":
		q.State = Doubts
		return false, nil
	case "aprobar":
		someOK := false
		for i := range q.Lines {
			ok := true
			if len(r.Items) > 0 {
				ok = r.Items[q.Lines[i].ID]
			}
			v := ok
			q.Lines[i].Approved = &v
			someOK = someOK || ok
		}
		if !someOK {
			return false, ErrNoApproved
		}
		q.State = Approved
		return true, nil
	}
	return false, shared.Validation("decision_invalida", "La respuesta debe ser aprobar, rechazar o dudas.")
}

// ApprovedParts devuelve los repuestos aprobados para descontar del inventario (HU-33).
func (q Quote) ApprovedParts() []Line {
	var out []Line
	for _, l := range q.Lines {
		if l.Kind == "repuesto" && l.RefID != nil && l.Approved != nil && *l.Approved {
			out = append(out, l)
		}
	}
	return out
}
