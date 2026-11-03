// Package domain contiene la orden de servicio y su máquina de estados (docs/02 §10). Sin I/O.
package domain

import (
	"fmt"
	"strings"
	"time"

	"sweveh/internal/shared"
)

// State es uno de los 7 estados del flujo más Cancelada (Q-04, Q-05).
type State string

const (
	StatePending    State = "por_recibir"
	StateReceived   State = "recibido"
	StateDiagnosis  State = "en_diagnostico"
	StateAwaitingOK State = "esperando_aprobacion"
	StateRepair     State = "en_reparacion"
	StateReady      State = "listo"
	StateDelivered  State = "entregado"
	StateCancelled  State = "cancelada"
)

// Open indica si la orden sigue abierta (Entregado y Cancelada la cierran).
func (s State) Open() bool { return s != StateDelivered && s != StateCancelled }

var (
	ErrOrderNotFound   = shared.NotFound("orden_no_encontrada", "No encontramos esa orden en tu taller.")
	ErrSymptomRequired = shared.Validation("sintoma_requerido", "Escribe lo que dice el cliente que tiene el vehículo.").WithField("sintoma")
	ErrSymptomTooLong  = shared.Validation("sintoma_largo", "El síntoma puede tener hasta 2.000 letras.").WithField("sintoma")
	ErrInitialState    = shared.Validation("estado_inicial_invalido", "Una orden nueva empieza en Por recibir o en Recibido.").WithField("estadoInicial")
	// HU-11: "motor pendiente" no bloquea la orden hasta que se intenta pasar a Recibido (HU-20).
	ErrEngineRequired = shared.Rule("motor_pendiente",
		"Falta completar la ficha del vehículo (combustible, caja, cilindraje y turbo o ciclo). Complétala para pasar la orden a Recibido.")
	// HU-16, criterio 2: avisar y dejar abrir otra solo si confirma.
	ErrOpenOrderExists = shared.Conflict("orden_abierta_existente",
		"Este vehículo ya tiene una orden abierta. Confirma si quieres abrir otra.")
)

// VehicleSnapshot son los datos del vehículo que la orden necesita, leídos por la fachada de vehicles.
type VehicleSnapshot struct {
	ID            shared.ID
	CustomerID    shared.ID
	Plate         string
	Kind          string
	EnginePending bool
}

// Order es el agregado orden de servicio.
type Order struct {
	ID          shared.ID
	TallerID    shared.ID
	Number      int
	VehicleID   shared.ID
	CustomerID  shared.ID
	Plate       string
	VehicleKind string
	State       State
	Symptom     string
	Version     int
	CreatedAt   time.Time
	CreatedBy   shared.ID

	MechanicID   *shared.ID
	PromisedAt   *time.Time
	Example      bool
	PickedUpBy   string
	CancelReason string
	ClosedAt     *time.Time
}

// NewOrderInput es lo que llega para abrir una orden (HU-16).
type NewOrderInput struct {
	Vehicle             VehicleSnapshot
	Symptom             string
	InitialState        State
	OpenOrdersOfVehicle int
	ConfirmDuplicate    bool
	CreatedBy           shared.ID
}

// NewOrder abre la orden. El número lo asigna el contador del taller.
func NewOrder(id, tallerID shared.ID, number int, in NewOrderInput, now time.Time) (Order, error) {
	symptom := strings.TrimSpace(in.Symptom)
	if symptom == "" {
		return Order{}, ErrSymptomRequired
	}
	if len([]rune(symptom)) > 2000 {
		return Order{}, ErrSymptomTooLong
	}
	state := in.InitialState
	if state == "" {
		state = StatePending
	}
	if state != StatePending && state != StateReceived {
		return Order{}, ErrInitialState
	}
	if state == StateReceived && in.Vehicle.EnginePending {
		return Order{}, ErrEngineRequired
	}
	if in.OpenOrdersOfVehicle > 0 && !in.ConfirmDuplicate {
		return Order{}, ErrOpenOrderExists
	}
	return Order{
		ID:          id,
		TallerID:    tallerID,
		Number:      number,
		VehicleID:   in.Vehicle.ID,
		CustomerID:  in.Vehicle.CustomerID,
		Plate:       in.Vehicle.Plate,
		VehicleKind: in.Vehicle.Kind,
		State:       state,
		Symptom:     symptom,
		Version:     1,
		CreatedAt:   now,
		CreatedBy:   in.CreatedBy,
	}, nil
}

// ValidateOpen revisa las reglas que no dependen del número, para no gastar un consecutivo
// en una orden que de todos modos se va a rechazar.
func ValidateOpen(in NewOrderInput) error {
	_, err := NewOrder(shared.NilID, shared.NilID, 1, in, time.Time{})
	return err
}

// Code es el número legible por taller: OS-0045.
func (o Order) Code() string { return FormatCode(o.Number) }

// FormatCode da el formato OS-0000.
func FormatCode(n int) string { return fmt.Sprintf("OS-%04d", n) }
