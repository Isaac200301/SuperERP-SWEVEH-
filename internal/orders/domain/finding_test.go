package domain

import (
	"errors"
	"slices"
	"testing"

	"sweveh/internal/shared"
)

func TestNewFinding_ValidatesAndNormalizes(t *testing.T) {
	o := Order{ID: shared.NewID(), TallerID: shared.NewID(), State: StateDiagnosis}
	f, err := NewFinding(shared.NewID(), o, "frenos", "  Pastillas   al 10 % ", "urgente", []string{"p0301", "P0301", " c1234 "}, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if f.Description != "Pastillas al 10 %" || !slices.Equal(f.FaultCodes, []string{"P0301", "C1234"}) || f.OrderID != o.ID {
		t.Fatalf("hallazgo: %+v", f)
	}

	bad := []struct {
		state            State
		system, sev, txt string
		codes            []string
		want             error
	}{
		{StatePending, "frenos", "urgente", "x", nil, ErrFindingState},
		{StateDelivered, "frenos", "urgente", "x", nil, ErrFindingState},
		{StateDiagnosis, "radio", "urgente", "x", nil, ErrFindingSystem},
		{StateDiagnosis, "frenos", "grave", "x", nil, ErrFindingSeverity},
		{StateDiagnosis, "frenos", "urgente", "   ", nil, ErrFindingText},
		{StateDiagnosis, "motor", "urgente", "x", []string{"X0301"}, ErrFaultCode},
	}
	for _, b := range bad {
		o.State = b.state
		if _, err := NewFinding(shared.NewID(), o, b.system, b.txt, b.sev, b.codes, nil, now); !errors.Is(err, b.want) {
			t.Errorf("%+v: %v, se esperaba %v", b, err, b.want)
		}
	}
}

func TestFindings_RemoveOnlyBeforeQuote(t *testing.T) {
	for st, want := range map[State]bool{StateReceived: true, StateDiagnosis: true, StateAwaitingOK: false, StateRepair: false} {
		if got := (Order{State: st}).CanRemoveFindings(); got != want {
			t.Errorf("%s: borrar=%v, se esperaba %v", st, got, want)
		}
	}
	if !(Order{State: StateRepair}).CanAddFindings() {
		t.Error("en reparación se agregan hallazgos para la cotización adicional (HU-31)")
	}
}
