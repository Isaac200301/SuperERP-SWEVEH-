// Package wire conecta mensajería, automatizaciones, agente, KPIs y administración con los módulos base.
// Es parte del composition root (lo usan cmd/api y cmd/worker); no tiene reglas de negocio propias.
package wire

import (
	assistantpg "sweveh/internal/assistant/adapters/out/postgres"
	assistantapp "sweveh/internal/assistant/app"
	automationspg "sweveh/internal/automations/adapters/out/postgres"
	automationsapp "sweveh/internal/automations/app"
	"sweveh/internal/messaging/adapters/out/channel"
	messagingpg "sweveh/internal/messaging/adapters/out/postgres"
	messagingapp "sweveh/internal/messaging/app"
	"sweveh/internal/platform/config"
	"sweveh/internal/server"
)

// Services son los servicios del motor (mensajería, automatizaciones, agente).
type Services struct {
	M           *server.Modules
	Messaging   *messagingapp.Service
	Automations *automationsapp.Service
	Assistant   *assistantapp.Service
}

// Build arma los servicios del motor sobre los módulos ya cableados.
func Build(m *server.Modules, wa config.WhatsApp, ai config.AI) *Services {
	ch := channel.Router{Local: &channel.Local{}}
	if wa.AccessToken != "" {
		ch.Meta = channel.NewMeta(wa.APIBase, wa.AccessToken)
	}
	s := &Services{M: m}
	s.Messaging = messagingapp.NewService(m.Tx, messagingpg.Repository{}, ch, m.Events, m.Clock)
	s.Automations = automationsapp.NewService(m.Tx, automationspg.Repository{}, autoOrders{m}, autoCustomers{m}, autoWorkshops{m},
		autoQuotes{m}, autoMessenger{s}, m.Audit, m.Clock)
	cap := ai.MonthlyCapMicroUSD
	if cap <= 0 {
		cap = 5_000_000
	}
	s.Assistant = assistantapp.NewService(m.Tx, assistantpg.Repository{}, &taller{s: s}, m.Audit, m.Clock, cap)
	return s
}
