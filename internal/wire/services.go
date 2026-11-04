// Package wire conecta mensajería, automatizaciones, agente, KPIs y administración con los módulos base.
// Es parte del composition root (lo usan cmd/api y cmd/worker); no tiene reglas de negocio propias.
package wire

import (
	"time"

	"sweveh/internal/assistant/adapters/out/llm/openrouter"
	"sweveh/internal/assistant/adapters/out/llm/rules"
	assistantpg "sweveh/internal/assistant/adapters/out/postgres"
	assistantapp "sweveh/internal/assistant/app"
	automationspg "sweveh/internal/automations/adapters/out/postgres"
	automationsapp "sweveh/internal/automations/app"
	messagingpg "sweveh/internal/messaging/adapters/out/postgres"
	"sweveh/internal/messaging/adapters/out/whatsapp"
	"sweveh/internal/messaging/adapters/out/whatsapp/local"
	"sweveh/internal/messaging/adapters/out/whatsapp/meta"
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
	return BuildWith(m, Channel(wa), Interpreter(ai), ai)
}

// Interpreter elige el intérprete del agente según AI_PROVIDER (ADR-0026). config ya validó la combinación.
func Interpreter(ai config.AI) assistantapp.Interpreter {
	if ai.Provider == "openrouter" {
		return openrouter.New(ai.APIBase, ai.APIKey, ai.Model, assistantapp.AITimeout)
	}
	return rules.Interpreter{}
}

// Channel elige el proveedor de WhatsApp según la configuración (WHATSAPP_PROVIDER). Los números del
// simulador ("local-…") siempre van al canal local, para la demo sin cuenta.
func Channel(wa config.WhatsApp) messagingapp.Channel {
	r := whatsapp.Router{Local: local.Channel{}}
	if wa.Provider == "meta" {
		r.Provider = meta.New(wa.APIBase, wa.AccessToken, 10*time.Second)
	}
	return r
}

// Parser interpreta el webhook del proveedor. Meta y el simulador comparten formato (meta.SimulatedBody).
func Parser() messagingapp.InboundParser { return meta.Parser{} }

// BuildWith es Build con el canal y el intérprete ya elegidos: las pruebas inyectan un canal que falla o que
// registra lo enviado, y una IA falsa.
func BuildWith(m *server.Modules, ch messagingapp.Channel, it assistantapp.Interpreter, ai config.AI) *Services {
	s := &Services{M: m}
	s.Messaging = messagingapp.NewService(m.Tx, messagingpg.Repository{}, ch, Parser(), m.Events, m.Clock)
	s.Automations = automationsapp.NewService(m.Tx, automationspg.Repository{}, autoOrders{m}, autoCustomers{m}, autoWorkshops{m},
		autoQuotes{m}, autoMessenger{s}, m.Audit, m.Clock)
	cap := ai.MonthlyCapMicroUSD
	if cap <= 0 {
		cap = 5_000_000
	}
	s.Assistant = assistantapp.NewService(m.Tx, assistantpg.Repository{}, &taller{s: s}, m.Audit, m.Clock, it, rules.Interpreter{}, cap)
	return s
}
