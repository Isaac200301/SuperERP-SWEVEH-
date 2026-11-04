package domain

import (
	"errors"
	"testing"
)

func at(s State) Order { return Order{State: s, Version: 1} }

func TestOptionalStates_AwaitingPartsIsASideTrip(t *testing.T) {
	on := FlowOptions{AwaitingParts: true}
	o := at(StateRepair)
	if _, err := o.Transition(TransitionInput{To: StateAwaitingParts, Options: on}); !errors.Is(err, ErrPartsReason) {
		t.Fatalf("sin motivo: %v", err)
	}
	if _, err := o.Transition(TransitionInput{To: StateAwaitingParts, Options: on, Reason: "Llega el kit de embrague"}); err != nil || o.State != StateAwaitingParts {
		t.Fatalf("a esperando repuesto: %v %s", err, o.State)
	}
	if _, err := o.Transition(TransitionInput{To: StateReady, Options: on}); !errors.Is(err, ErrStep) {
		t.Fatalf("de esperando repuesto solo se vuelve a reparación: %v", err)
	}
	if _, err := o.Transition(TransitionInput{To: StateRepair}); err != nil || o.State != StateRepair {
		t.Fatalf("vuelve a reparación aunque el taller lo haya apagado: %v %s", err, o.State)
	}
	off := at(StateRepair)
	if _, err := off.Transition(TransitionInput{To: StateAwaitingParts, Reason: "x"}); !errors.Is(err, ErrStateOff) {
		t.Fatalf("apagado no entra: %v", err)
	}
	diag := at(StateDiagnosis)
	if _, err := diag.Transition(TransitionInput{To: StateAwaitingParts, Options: on, Reason: "x"}); !errors.Is(err, ErrStep) {
		t.Fatalf("solo entra desde reparación: %v", err)
	}
}

func TestOptionalStates_QualityCheckBetweenRepairAndReady(t *testing.T) {
	on := FlowOptions{QualityCheck: true}
	o := at(StateRepair)
	if _, err := o.Transition(TransitionInput{To: StateReady, Options: on}); !errors.Is(err, ErrStep) {
		t.Fatalf("con control de calidad no se salta a Listo: %v", err)
	}
	if _, err := o.Transition(TransitionInput{To: StateQuality, Options: on}); err != nil {
		t.Fatal(err)
	}
	if _, err := o.Transition(TransitionInput{To: StateReady, Options: on}); err != nil || o.State != StateReady {
		t.Fatalf("de control a listo: %v %s", err, o.State)
	}
	// Apagado: el flujo de siempre; una orden que quedó en control puede salir a Listo o volver a reparación.
	plain := at(StateRepair)
	if _, err := plain.Transition(TransitionInput{To: StateQuality}); !errors.Is(err, ErrStateOff) {
		t.Fatalf("apagado no entra: %v", err)
	}
	for _, to := range []State{StateReady, StateRepair} {
		left := at(StateQuality)
		if _, err := left.Transition(TransitionInput{To: to}); err != nil {
			t.Fatalf("salir de control apagado a %s: %v", to, err)
		}
	}
	if got := FlowFor(on); len(got) != len(Flow)+1 || got[5] != StateQuality {
		t.Fatalf("secuencia con control: %v", got)
	}
}
