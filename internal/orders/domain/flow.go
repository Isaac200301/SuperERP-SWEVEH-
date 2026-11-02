package domain

import (
	"slices"
	"strings"
	"time"

	"sweveh/internal/shared"
)

// Flow es el orden de los 7 estados (Q-04). Cancelada queda fuera (Q-05).
var Flow = []State{StatePending, StateReceived, StateDiagnosis, StateAwaitingOK, StateRepair, StateReady, StateDelivered}

var (
	ErrStateUnknown  = shared.Validation("estado_invalido", "Ese estado no existe.").WithField("estado")
	ErrStep          = shared.Rule("salto_de_estado", "Solo puedes avanzar al siguiente estado o devolverte uno.")
	ErrClosed        = shared.Rule("orden_cerrada", "La orden ya está cerrada.")
	ErrChecklist     = shared.Rule("checklist_incompleto", "Completa el checklist de recepción antes de pasar a Recibido.")
	ErrNeedsQuote    = shared.Rule("falta_cotizacion", "Envía la cotización al cliente para pasar a Esperando aprobación.")
	ErrNeedsApproval = shared.Rule("falta_aprobacion", "El cliente no ha aprobado. El dueño puede continuar escribiendo el motivo.")
	ErrPickup        = shared.Validation("entrega_incompleta", "Escribe quién recoge y confirma los accesorios del checklist.").WithField("recogidoPor")
	ErrCancelReason  = shared.Validation("motivo_requerido", "Escribe el motivo.").WithField("motivo")
)

func ParseState(s string) (State, error) {
	st := State(s)
	if slices.Contains(Flow, st) || st == StateCancelled {
		return st, nil
	}
	return "", ErrStateUnknown
}

func index(s State) int { return slices.Index(Flow, s) }

// TransitionInput son los hechos que la regla necesita, reunidos por el caso de uso.
type TransitionInput struct {
	To                State
	EnginePending     bool
	ChecklistComplete bool
	FromQuote         bool // la mueve la cotización (enviada o aprobada por el cliente), no una persona
	CanOverride       bool // permiso del dueño para avanzar sin aprobación (HU-20, criterio 2)
	Reason            string
	PickedUpBy        string
	AccessoriesOK     bool
	ExpectedVersion   int
	At                time.Time
}

// Transition aplica HU-20: un paso adelante o atrás, con las condiciones de cada puerta.
func (o *Order) Transition(in TransitionInput) (State, error) {
	if in.ExpectedVersion != 0 && in.ExpectedVersion != o.Version {
		return "", shared.ErrVersionConflict
	}
	if !o.State.Open() {
		return "", ErrClosed
	}
	from, ti := index(o.State), index(in.To)
	if ti < 0 || (ti != from+1 && ti != from-1) {
		return "", ErrStep
	}
	forward := ti == from+1
	if forward {
		switch in.To {
		case StateReceived:
			if in.EnginePending {
				return "", ErrEngineRequired
			}
			if !in.ChecklistComplete {
				return "", ErrChecklist
			}
		case StateAwaitingOK:
			if !in.FromQuote {
				return "", ErrNeedsQuote
			}
		case StateRepair:
			if !in.FromQuote && (!in.CanOverride || strings.TrimSpace(in.Reason) == "") {
				return "", ErrNeedsApproval
			}
		case StateDelivered:
			if strings.TrimSpace(in.PickedUpBy) == "" || !in.AccessoriesOK {
				return "", ErrPickup
			}
			o.PickedUpBy = strings.TrimSpace(in.PickedUpBy)
			at := in.At
			o.ClosedAt = &at
		}
	}
	prev := o.State
	o.State = in.To
	o.Version++
	return prev, nil
}

// Cancel saca la orden del flujo con motivo (HU-26).
func (o *Order) Cancel(reason string, expectedVersion int, at time.Time) (State, error) {
	if expectedVersion != 0 && expectedVersion != o.Version {
		return "", shared.ErrVersionConflict
	}
	if !o.State.Open() {
		return "", ErrClosed
	}
	if strings.TrimSpace(reason) == "" {
		return "", ErrCancelReason
	}
	prev := o.State
	o.State, o.CancelReason, o.ClosedAt = StateCancelled, strings.TrimSpace(reason), &at
	o.Version++
	return prev, nil
}
