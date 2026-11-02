package shared

import "time"

// Event es un hecho de dominio que se guarda en el outbox en la misma transacción
// que el cambio de estado (R-03 punto 3, docs/06 §8.6).
type Event struct {
	Type        string // por ejemplo "orders.orden_creada"
	AggregateID ID
	OccurredAt  time.Time
	Payload     map[string]any
}

// Actor es quien ejecuta una acción, para la auditoría (docs/06 §10.4).
type Actor struct {
	Type ActorType
	ID   ID
}

type ActorType string

const (
	ActorUser   ActorType = "usuario"
	ActorAgent  ActorType = "agente"
	ActorSystem ActorType = "sistema"
)
