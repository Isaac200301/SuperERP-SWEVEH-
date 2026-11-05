// Package wire conecta mensajería, automatizaciones, agente, KPIs y administración con los módulos base.
// Es parte del composition root (lo usan cmd/api y cmd/worker); no tiene reglas de negocio propias.
package wire

import (
	"context"
	"time"

	"sweveh/internal/assistant/adapters/out/llm/openrouter"
	"sweveh/internal/assistant/adapters/out/llm/rules"
	assistantpg "sweveh/internal/assistant/adapters/out/postgres"
	assistantapp "sweveh/internal/assistant/app"
	automationspg "sweveh/internal/automations/adapters/out/postgres"
	automationsapp "sweveh/internal/automations/app"
	"sweveh/internal/catalog/adapters/out/vision"
	messagingpg "sweveh/internal/messaging/adapters/out/postgres"
	"sweveh/internal/messaging/adapters/out/whatsapp"
	"sweveh/internal/messaging/adapters/out/whatsapp/local"
	"sweveh/internal/messaging/adapters/out/whatsapp/meta"
	"sweveh/internal/messaging/adapters/out/whatsapp/waha"
	messagingapp "sweveh/internal/messaging/app"
	"sweveh/internal/platform/config"
	"sweveh/internal/server"
	"sweveh/internal/shared"
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
	s := BuildWith(m, Channel(wa), Interpreter(ai), ai)
	if ai.Provider == "openrouter" { // el asesor con tools necesita un modelo; con reglas queda el flujo de plantillas
		s.Assistant.SetAdvisor(openrouter.New(ai.APIBase, ai.APIKey, ai.Model, 20*time.Second))
		// HU-93: la lista de precios por foto se lee con un modelo de visión (ADR-0032).
		m.Catalog.SetPriceReader(vision.New(ai.APIBase, ai.APIKey, ai.VisionModel, 60*time.Second))
	}
	return s
}

// Interpreter elige el intérprete del agente según AI_PROVIDER (ADR-0026). config ya validó la combinación.
func Interpreter(ai config.AI) assistantapp.Interpreter {
	if ai.Provider == "openrouter" {
		return openrouter.New(ai.APIBase, ai.APIKey, ai.Model, assistantapp.AITimeout)
	}
	return rules.Interpreter{}
}

// Channel elige el proveedor de WhatsApp según la configuración (WHATSAPP_PROVIDER). Los números del
// simulador ("local-…") siempre van al canal local, para la demo sin cuenta; los de WAHA ("waha:…") a WAHA si
// está configurado (demo con número de prueba, ADR-0027).
func Channel(wa config.WhatsApp) messagingapp.Channel {
	r := whatsapp.Router{Local: local.Channel{}}
	if wa.Provider == "meta" {
		r.Provider = meta.New(wa.APIBase, wa.AccessToken, 10*time.Second)
	}
	if wa.WAHA.Enabled() {
		r.WAHA = WAHAClient(wa.WAHA)
	}
	return r
}

// WAHAClient es el cliente de WAHA (envíos y sesión) con su timeout.
func WAHAClient(w config.WAHA) *waha.Client { return waha.New(w.URL, w.APIKey, 10*time.Second) }

// Parser interpreta los webhooks guardados: todos van a la misma cola y cada cuerpo se lee con el parser de su
// formato (WAHA trae "event" y "session"; Meta y el simulador, el formato de Cloud API).
func Parser() messagingapp.InboundParser { return parser{} }

type parser struct{}

func (parser) Parse(body []byte) ([]messagingapp.Inbound, []messagingapp.Status, error) {
	if waha.Is(body) {
		return waha.Parser{}.Parse(body)
	}
	return meta.Parser{}.Parse(body)
}

// BuildWith es Build con el canal y el intérprete ya elegidos: las pruebas inyectan un canal que falla o que
// registra lo enviado, y una IA falsa.
func BuildWith(m *server.Modules, ch messagingapp.Channel, it assistantapp.Interpreter, ai config.AI) *Services {
	s := &Services{M: m}
	s.Messaging = messagingapp.NewService(m.Tx, messagingpg.Repository{}, ch, Parser(), m.Events, m.Clock)
	s.Messaging.SetMediaSource(media{s})
	s.Automations = automationsapp.NewService(m.Tx, automationspg.Repository{}, autoOrders{m}, autoCustomers{m}, autoWorkshops{m},
		autoQuotes{m}, autoMessenger{s}, m.Audit, m.Clock)
	cap := ai.MonthlyCapMicroUSD
	if cap <= 0 {
		cap = 5_000_000
	}
	s.Assistant = assistantapp.NewService(m.Tx, assistantpg.Repository{}, &taller{s: s}, m.Audit, m.Clock, it, rules.Interpreter{}, cap)
	m.Customers.SetMover(customerMover{s})   // unir clientes repetidos (HU-15)
	m.Customers.SetEraser(customerEraser{s}) // borrar los datos del titular (HU-87)
	return s
}

// customerMover pasa al cliente que se conserva los vehículos, órdenes y conversaciones del repetido (HU-15), cada
// cosa por la fachada de su módulo y en la transacción de la unión.
type customerMover struct{ s *Services }

func (c customerMover) MoveCustomer(ctx context.Context, tallerID, from, to shared.ID) (map[string]int, error) {
	out := map[string]int{}
	var err error
	if out["vehiculos"], err = c.s.M.VehDir.MoveCustomer(ctx, tallerID, from, to); err != nil {
		return nil, err
	}
	if out["ordenes"], err = c.s.M.OrdersF.MoveCustomer(ctx, tallerID, from, to); err != nil {
		return nil, err
	}
	if out["conversaciones"], err = c.s.Messaging.MoveCustomer(ctx, tallerID, from, to); err != nil {
		return nil, err
	}
	return out, nil
}
