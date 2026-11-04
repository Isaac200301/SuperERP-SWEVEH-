package domain

import (
	"slices"
	"strings"
	"time"

	"sweveh/internal/shared"
)

// Flow es el orden de los 7 estados (Q-04). Cancelada queda fuera (Q-05).
var Flow = []State{StatePending, StateReceived, StateDiagnosis, StateAwaitingOK, StateRepair, StateReady, StateDelivered}

// FlowOptions son los estados opcionales que activó el taller en sus parámetros.
type FlowOptions struct {
	AwaitingParts bool // "Esperando repuesto": sale de En reparación y vuelve a En reparación
	QualityCheck  bool // "Control de calidad": entre En reparación y Listo
}

// FlowFor arma la secuencia del taller; Esperando repuesto no entra porque es un desvío, no un paso.
func FlowFor(o FlowOptions) []State {
	if !o.QualityCheck {
		return Flow
	}
	out := make([]State, 0, len(Flow)+1)
	for _, s := range Flow {
		if s == StateReady {
			out = append(out, StateQuality)
		}
		out = append(out, s)
	}
	return out
}

var (
	ErrStateUnknown  = shared.Validation("estado_invalido", "Ese estado no existe.").WithField("estado")
	ErrStep          = shared.Rule("salto_de_estado", "Solo puedes avanzar al siguiente estado o devolverte uno.")
	ErrClosed        = shared.Rule("orden_cerrada", "La orden ya está cerrada.")
	ErrChecklist     = shared.Rule("checklist_incompleto", "Completa el checklist de recepción antes de pasar a Recibido.")
	ErrNeedsQuote    = shared.Rule("falta_cotizacion", "Envía la cotización al cliente para pasar a Esperando aprobación.")
	ErrNeedsApproval = shared.Rule("falta_aprobacion", "El cliente no ha aprobado. El dueño puede continuar escribiendo el motivo.")
	ErrPickup        = shared.Validation("entrega_incompleta", "Escribe quién recoge y confirma los accesorios del checklist.").WithField("recogidoPor")
	ErrCancelReason  = shared.Validation("motivo_requerido", "Escribe el motivo.").WithField("motivo")
	ErrStateOff      = shared.Rule("estado_no_activo", "Ese estado no está activo en tu taller. Actívalo en Mi taller → Parámetros.")
	ErrPartsReason   = shared.Validation("motivo_repuesto", "Escribe qué repuesto falta.").WithField("motivo")
)

func ParseState(s string) (State, error) {
	st := State(s)
	if slices.Contains(Flow, st) || st == StateCancelled || st == StateAwaitingParts || st == StateQuality {
		return st, nil
	}
	return "", ErrStateUnknown
}


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
	Options           FlowOptions // estados opcionales del taller
}

// Transition aplica HU-20: un paso adelante o atrás, con las condiciones de cada puerta.
func (o *Order) Transition(in TransitionInput) (State, error) {
	if in.ExpectedVersion != 0 && in.ExpectedVersion != o.Version {
		return "", shared.ErrVersionConflict
	}
	if !o.State.Open() {
		return "", ErrClosed
	}
	if done, err := o.lateral(in); done {
		return o.moveTo(in.To, err)
	}
	if in.To == StateQuality && !in.Options.QualityCheck {
		return "", ErrStateOff
	}
	flow := FlowFor(in.Options)
	from, ti := slices.Index(flow, o.State), slices.Index(flow, in.To)
	if from < 0 && o.State == StateQuality && (in.To == StateReady || in.To == StateRepair) {
		// El taller apagó Control de calidad con esta orden adentro: sale a Listo o vuelve a reparación, sin puerta.
		return o.moveTo(in.To, nil)
	}
	if ti < 0 || (ti != from+1 && ti != from-1) {
		return "", ErrStep
	}
	if ti == from+1 {
		if err := o.forwardGate(in); err != nil {
			return "", err
		}
	}
	return o.moveTo(in.To, nil)
}

// moveTo aplica el cambio de estado si no hubo error y devuelve el estado anterior.
func (o *Order) moveTo(to State, err error) (State, error) {
	if err != nil {
		return "", err
	}
	prev := o.State
	o.State = to
	o.Version++
	return prev, nil
}

// forwardGate son las condiciones para avanzar a cada estado (HU-20, HU-25).
func (o *Order) forwardGate(in TransitionInput) error {
	switch in.To {
	case StateReceived:
		if in.EnginePending {
			return ErrEngineRequired
		}
		if !in.ChecklistComplete {
			return ErrChecklist
		}
	case StateAwaitingOK:
		if !in.FromQuote {
			return ErrNeedsQuote
		}
	case StateRepair:
		if !in.FromQuote && (!in.CanOverride || strings.TrimSpace(in.Reason) == "") {
			return ErrNeedsApproval
		}
	case StateDelivered:
		if strings.TrimSpace(in.PickedUpBy) == "" || !in.AccessoriesOK {
			return ErrPickup
		}
		o.PickedUpBy = strings.TrimSpace(in.PickedUpBy)
		at := in.At
		o.ClosedAt = &at
	}
	return nil
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

// lateral resuelve el desvío "Esperando repuesto": entra desde En reparación (si el taller lo usa, con motivo) y solo
// vuelve a En reparación. Una orden que ya estaba esperando puede volver aunque el taller lo haya apagado.
func (o *Order) lateral(in TransitionInput) (bool, error) {
	switch {
	case in.To == StateAwaitingParts:
		if !in.Options.AwaitingParts {
			return true, ErrStateOff
		}
		if o.State != StateRepair {
			return true, ErrStep
		}
		if strings.TrimSpace(in.Reason) == "" {
			return true, ErrPartsReason
		}
		return true, nil
	case o.State == StateAwaitingParts:
		if in.To != StateRepair {
			return true, ErrStep
		}
		return true, nil
	}
	return false, nil
}
