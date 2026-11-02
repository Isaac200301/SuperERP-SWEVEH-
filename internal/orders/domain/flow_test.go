package domain

import (
	"errors"
	"testing"

	"sweveh/internal/shared"
)

func orderIn(t *testing.T, st State) *Order {
	t.Helper()
	o, err := NewOrder(shared.NewID(), shared.NewID(), 1, input(), now)
	if err != nil {
		t.Fatal(err)
	}
	o.State = st
	return &o
}

// HU-20, criterio 1: solo un paso adelante o uno atrás.
func TestTransition_OnlyOneStepForwardOrBack(t *testing.T) {
	o := orderIn(t, StateRepair)
	if _, err := o.Transition(TransitionInput{To: StateDelivered, At: now}); !errors.Is(err, ErrStep) {
		t.Fatalf("salto adelante: %v", err)
	}
	if _, err := o.Transition(TransitionInput{To: StateDiagnosis, At: now}); !errors.Is(err, ErrStep) {
		t.Fatalf("salto atrás de 2: %v", err)
	}
	prev, err := o.Transition(TransitionInput{To: StateAwaitingOK, At: now})
	if err != nil || prev != StateRepair || o.State != StateAwaitingOK || o.Version != 2 {
		t.Fatalf("devolverse un paso: prev=%s err=%v orden=%+v", prev, err, o)
	}
}

// HU-20, criterio 2: sin aprobación no pasa a En reparación; el dueño sí, con motivo.
func TestTransition_RepairNeedsApprovalOrOwnerReason(t *testing.T) {
	cases := []struct {
		name string
		in   TransitionInput
		want error
	}{
		{"sin aprobación", TransitionInput{To: StateRepair}, ErrNeedsApproval},
		{"dueño sin motivo", TransitionInput{To: StateRepair, CanOverride: true, Reason: "  "}, ErrNeedsApproval},
		{"motivo sin permiso", TransitionInput{To: StateRepair, Reason: "aprobó por teléfono"}, ErrNeedsApproval},
		{"dueño con motivo", TransitionInput{To: StateRepair, CanOverride: true, Reason: "aprobó por teléfono"}, nil},
		{"aprobada por el cliente", TransitionInput{To: StateRepair, FromQuote: true}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o := orderIn(t, StateAwaitingOK)
			c.in.At = now
			_, err := o.Transition(c.in)
			if !errors.Is(err, c.want) {
				t.Fatalf("esperaba %v, obtuve %v", c.want, err)
			}
			if c.want != nil && o.State != StateAwaitingOK {
				t.Fatalf("la orden cambió pese al error: %s", o.State)
			}
		})
	}
}

// HU-20, criterio 4 (HU-11): motor pendiente o checklist incompleto bloquean Recibido.
func TestTransition_ReceivedGates(t *testing.T) {
	o := orderIn(t, StatePending)
	if _, err := o.Transition(TransitionInput{To: StateReceived, EnginePending: true, ChecklistComplete: true}); !errors.Is(err, ErrEngineRequired) {
		t.Fatalf("motor pendiente: %v", err)
	}
	if _, err := o.Transition(TransitionInput{To: StateReceived}); !errors.Is(err, ErrChecklist) {
		t.Fatalf("checklist: %v", err)
	}
	if _, err := o.Transition(TransitionInput{To: StateReceived, ChecklistComplete: true}); err != nil {
		t.Fatal(err)
	}
}

func TestTransition_AwaitingOKNeedsQuote(t *testing.T) {
	o := orderIn(t, StateDiagnosis)
	if _, err := o.Transition(TransitionInput{To: StateAwaitingOK}); !errors.Is(err, ErrNeedsQuote) {
		t.Fatalf("sin cotización: %v", err)
	}
	if _, err := o.Transition(TransitionInput{To: StateAwaitingOK, FromQuote: true}); err != nil {
		t.Fatal(err)
	}
}

func TestTransition_DeliveredClosesWithPickup(t *testing.T) {
	o := orderIn(t, StateReady)
	if _, err := o.Transition(TransitionInput{To: StateDelivered, PickedUpBy: "Ana", At: now}); !errors.Is(err, ErrPickup) {
		t.Fatalf("sin accesorios: %v", err)
	}
	if _, err := o.Transition(TransitionInput{To: StateDelivered, PickedUpBy: " Ana ", AccessoriesOK: true, At: now}); err != nil {
		t.Fatal(err)
	}
	if o.PickedUpBy != "Ana" || o.ClosedAt == nil {
		t.Fatalf("cierre: %+v", o)
	}
	if _, err := o.Transition(TransitionInput{To: StateReady, At: now}); !errors.Is(err, ErrClosed) {
		t.Fatalf("orden cerrada: %v", err)
	}
}

func TestTransition_VersionConflictAndUnknownState(t *testing.T) {
	o := orderIn(t, StatePending)
	if _, err := o.Transition(TransitionInput{To: StateReceived, ChecklistComplete: true, ExpectedVersion: 9}); !errors.Is(err, shared.ErrVersionConflict) {
		t.Fatalf("versión: %v", err)
	}
	if _, err := ParseState("volando"); !errors.Is(err, ErrStateUnknown) {
		t.Fatalf("estado desconocido: %v", err)
	}
}

func TestCancel(t *testing.T) {
	o := orderIn(t, StateRepair)
	if _, err := o.Cancel(" ", 0, now); !errors.Is(err, ErrCancelReason) {
		t.Fatalf("sin motivo: %v", err)
	}
	if prev, err := o.Cancel("cliente no volvió", 0, now); err != nil || prev != StateRepair || o.State != StateCancelled || o.ClosedAt == nil {
		t.Fatalf("cancelar: prev=%s err=%v %+v", prev, err, o)
	}
	if _, err := o.Cancel("otra", 0, now); !errors.Is(err, ErrClosed) {
		t.Fatalf("ya cerrada: %v", err)
	}
}
