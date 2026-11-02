package domain

import (
	"errors"
	"strings"
	"testing"
	"time"

	"sweveh/internal/shared"
)

var now = time.Date(2026, 11, 1, 15, 0, 0, 0, time.UTC)

func input() NewOrderInput {
	return NewOrderInput{
		Vehicle: VehicleSnapshot{ID: shared.NewID(), CustomerID: shared.NewID(), Plate: "KLM482", Kind: "carro"},
		Symptom: "Suena al frenar",
	}
}

// HU-16, criterio 1: número consecutivo, estado Por recibir por defecto, síntoma y hora.
func TestNewOrder_DefaultsToPendingWithNumberAndTime(t *testing.T) {
	o, err := NewOrder(shared.NewID(), shared.NewID(), 45, input(), now)
	if err != nil {
		t.Fatal(err)
	}
	if o.Code() != "OS-0045" || o.State != StatePending || o.Symptom != "Suena al frenar" || !o.CreatedAt.Equal(now) {
		t.Fatalf("orden: %+v", o)
	}
}

func TestNewOrder_ReceivedRequiresEngine(t *testing.T) {
	in := input()
	in.InitialState = StateReceived
	in.Vehicle.EnginePending = true
	// HU-11, criterio 4: motor pendiente no bloquea... hasta pasar a Recibido.
	if _, err := NewOrder(shared.NewID(), shared.NewID(), 1, in, now); !errors.Is(err, ErrEngineRequired) {
		t.Fatalf("Recibido con motor pendiente: %v", err)
	}
	in.Vehicle.EnginePending = false
	o, err := NewOrder(shared.NewID(), shared.NewID(), 1, in, now)
	if err != nil || o.State != StateReceived {
		t.Fatalf("Recibido con motor completo: %+v %v", o, err)
	}
	in.InitialState = StatePending
	in.Vehicle.EnginePending = true
	if _, err := NewOrder(shared.NewID(), shared.NewID(), 1, in, now); err != nil {
		t.Fatalf("Por recibir con motor pendiente debe poder abrirse: %v", err)
	}
}

// HU-16, criterio 2: con una orden abierta, avisa y deja abrir otra solo si confirma.
func TestNewOrder_DuplicateOpenOrderNeedsConfirm(t *testing.T) {
	in := input()
	in.OpenOrdersOfVehicle = 1
	if _, err := NewOrder(shared.NewID(), shared.NewID(), 1, in, now); !errors.Is(err, ErrOpenOrderExists) {
		t.Fatalf("sin confirmar: %v", err)
	}
	in.ConfirmDuplicate = true
	if _, err := NewOrder(shared.NewID(), shared.NewID(), 1, in, now); err != nil {
		t.Fatalf("confirmada: %v", err)
	}
}

func TestNewOrder_Validation(t *testing.T) {
	cases := []struct {
		mut func(*NewOrderInput)
		err *shared.Error
	}{
		{func(i *NewOrderInput) { i.Symptom = "   " }, ErrSymptomRequired},
		{func(i *NewOrderInput) { i.Symptom = strings.Repeat("a", 2001) }, ErrSymptomTooLong},
		{func(i *NewOrderInput) { i.InitialState = StateRepair }, ErrInitialState},
		{func(i *NewOrderInput) { i.InitialState = "inventado" }, ErrInitialState},
	}
	for _, c := range cases {
		in := input()
		c.mut(&in)
		if err := ValidateOpen(in); !errors.Is(err, c.err) {
			t.Errorf("quería %v, dio %v", c.err, err)
		}
	}
}

func TestStateOpen(t *testing.T) {
	for _, s := range []State{StatePending, StateReceived, StateDiagnosis, StateAwaitingOK, StateRepair, StateReady} {
		if !s.Open() {
			t.Errorf("%s debía estar abierta", s)
		}
	}
	for _, s := range []State{StateDelivered, StateCancelled} {
		if s.Open() {
			t.Errorf("%s debía estar cerrada", s)
		}
	}
}

func TestFormatCode(t *testing.T) {
	if FormatCode(7) != "OS-0007" || FormatCode(12345) != "OS-12345" {
		t.Fatal(FormatCode(7), FormatCode(12345))
	}
}
