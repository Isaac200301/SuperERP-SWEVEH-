// Package app es el agente de WhatsApp (AU-04, AU-05) y la bandeja "Por revisar" (HU-51 a HU-66).
package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"sweveh/internal/assistant/domain"
	"sweveh/internal/audit"
	"sweveh/internal/shared"
)

// Taller es todo lo que el agente consulta o hace en otros módulos, por sus fachadas.
type Taller interface {
	Message(ctx context.Context, tallerID, id shared.ID) (Inbound, error)
	PreviousText(ctx context.Context, tallerID, convID shared.ID, before time.Time) (*Inbound, error)
	ConsentAsked(ctx context.Context, tallerID, convID shared.ID) (bool, error)
	Send(ctx context.Context, tallerID shared.ID, phone, text string, buttons []Button, automation string, orderID *shared.ID) (bool, error) // false = ventana cerrada
	SetMode(ctx context.Context, tallerID, convID shared.ID, mode string, wantsHuman bool) error
	LinkCustomer(ctx context.Context, tallerID, convID, customerID shared.ID) error

	CustomerByPhone(ctx context.Context, tallerID shared.ID, phone string) (*Customer, error)
	RegisterCustomer(ctx context.Context, tallerID shared.ID, phone, name string) (Customer, error)
	SetConsent(ctx context.Context, tallerID, customerID shared.ID, granted bool, ref string, at time.Time) error

	OpenOrders(ctx context.Context, tallerID, customerID shared.ID) ([]Order, error)
	Order(ctx context.Context, tallerID, orderID shared.ID) (Order, error)
	StatusReply(ctx context.Context, tallerID shared.ID, o Order, c Customer) (string, bool, error) // texto, AU-05 encendida
	AgentOn(ctx context.Context, tallerID shared.ID) (bool, error)
	RespondQuote(ctx context.Context, tallerID, orderID shared.ID, decision, ref string) error
	// AnswerReceipt guarda "Está bien" (ok) o "Algo no cuadra" de la constancia; false si ya había respuesta.
	AnswerReceipt(ctx context.Context, tallerID, orderID shared.ID, ok bool, ref string) (bool, error)
	CreateOrderFromPlate(ctx context.Context, tallerID, customerID shared.ID, plate, symptom string) (string, error)

	Catalog(ctx context.Context, tallerID shared.ID) ([]domain.Priced, []domain.Priced, error) // servicios, repuestos
	Workshop(ctx context.Context, tallerID shared.ID) (name string, open bool, nextOpening string, err error)
	Attends(ctx context.Context, tallerID shared.ID) (string, error) // carro | moto | ambos
}

type Inbound struct {
	ID             shared.ID
	ConversationID shared.ID
	Phone          string
	Kind           string
	Text           string
	ButtonID       string
	ProfileName    string
	Mode           string
	At             time.Time
}

type Customer struct {
	ID      shared.ID
	Name    string
	Phone   string
	Consent string
}

type Order struct {
	ID         shared.ID
	Code       string
	Plate      string
	Vehicle    string
	State      string
	PromisedAt *time.Time
}

type Button struct{ ID, Title string }

// DraftRecord es un borrador guardado.
type DraftRecord struct {
	ID             shared.ID
	TallerID       shared.ID
	ConversationID shared.ID
	InboundID      shared.ID
	Intent         string
	Proposed       string
	Final          *string
	Sources        []domain.Source
	Action         *Action
	State          string
	Origin         string
	CreatedAt      time.Time
	ResolvedAt     *time.Time
	InboundText    string
	Phone          string
}

// Action es lo que se ejecuta si el dueño envía el borrador (HU-55: crear la orden con la placa).
type Action struct {
	CreateOrder *struct {
		Plate   string `json:"placa"`
		Symptom string `json:"sintoma"`
	} `json:"crearOrden,omitempty"`
}

// AICall es el registro de cada llamada al intérprete (HU-99), ligada al mensaje que la originó.
type AICall struct {
	MessageID                        *shared.ID
	Provider, Model, Purpose, Result string
	TokensIn, TokensOut, LatencyMs   int
	CostMicroUSD                     int64
}

// Interpreter es el puerto LLMProvider (docs/06 §5, ADR-0026): una llamada por mensaje que devuelve el JSON crudo
// de domain.Understanding. El código lo valida antes de usarlo; el adaptador no decide nada (RNF-05, RNF-06).
type Interpreter interface {
	Interpret(ctx context.Context, r InterpretRequest) (InterpretResult, error)
}

// InterpretRequest es lo que ve la IA: el texto del cliente y la lista cerrada de servicios del taller.
// Sin datos personales del cliente (nombre, celular): no hacen falta para interpretar (AQ-10).
type InterpretRequest struct {
	Text, Workshop, Attends string
	Catalog                 []string
}

// InterpretResult es la salida cruda con su costo.
type InterpretResult struct {
	Raw                 []byte
	Provider, Model     string
	TokensIn, TokensOut int
	CostMicroUSD        int64
}

// AITimeout es lo máximo que se espera a la IA (HU-54, criterio 2).
const AITimeout = 30 * time.Second

type Repository interface {
	InsertDraft(ctx context.Context, d DraftRecord) error
	Draft(ctx context.Context, tallerID, id shared.ID) (DraftRecord, error)
	Drafts(ctx context.Context, tallerID shared.ID, onlyPending bool, limit int) ([]DraftRecord, error)
	Resolve(ctx context.Context, tallerID, id shared.ID, state string, final *string, by shared.ID, at time.Time) (bool, error)
	LogCall(ctx context.Context, tallerID shared.ID, c AICall) error
	MonthCost(ctx context.Context, tallerID shared.ID, since time.Time) (int64, error)
	EditStats(ctx context.Context, tallerID shared.ID, since time.Time) (sent, edited int, err error)
}

type UnitOfWork interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}
type AuditLog interface {
	Record(ctx context.Context, e audit.Entry) error
}
type Clock interface{ Now() time.Time }

type Service struct {
	uow    UnitOfWork
	repo   Repository
	taller Taller
	audit  AuditLog
	clock  Clock
	ai     Interpreter
	free   Interpreter // reglas: respaldo sin costo cuando se agota el tope mensual
	capUSD int64
}

func NewService(u UnitOfWork, r Repository, t Taller, a AuditLog, c Clock, ai, free Interpreter, monthlyCapMicroUSD int64) *Service {
	return &Service{uow: u, repo: r, taller: t, audit: a, clock: c, ai: ai, free: free, capUSD: monthlyCapMicroUSD}
}

// Textos de plantilla (llenos con datos; no son redacción de IA, por eso pueden salir solos, RF-18).
const (
	consentText = "Hola, te escribe %s. Antes de seguir, ¿aceptas el tratamiento de tus datos (nombre, celular y vehículo) para atender tu servicio? Puedes ver la política en la web del taller."
	thanksText  = "¡Gracias! Ya quedó registrada tu autorización. En un momento te respondemos."
	noConsent   = "Entendido. Sin tu autorización no procesamos tu mensaje automáticamente; una persona del taller te atiende."
	humanText   = "Te comunico con una persona del taller. Te responde en cuanto pueda."
	offTopicTxt = "Solo puedo ayudarte con temas de %s: servicios, citas y el estado de tu vehículo."
	closedTxt   = "Hola, estamos fuera de horario. Te respondemos %s."
	quoteOK     = "¡Listo! Quedó aprobada la cotización. Empezamos con tu vehículo y te avisamos cuando esté."
	quoteAsk    = "Claro, una persona del taller te escribe para resolver tus dudas sobre la cotización."
	whichText   = "¿Sobre cuál vehículo me preguntas?"
	receiptOK   = "¡Gracias! Quedó registrado que el recibo está bien. Te avisamos cada avance."
	receiptAsk  = "Gracias por avisarnos. Una persona del taller te escribe para revisarlo contigo."
	cantAnswer  = "No pudimos armar una respuesta segura. Responde tú."
)

// HandleInbound procesa un mensaje entrante (lo llama el worker al recibir "messaging.mensaje_recibido").
func (s *Service) HandleInbound(ctx context.Context, tallerID, msgID shared.ID) error {
	in, err := s.taller.Message(ctx, tallerID, msgID)
	if err != nil {
		return err
	}
	c, err := s.taller.CustomerByPhone(ctx, tallerID, in.Phone)
	if err != nil {
		return err
	}
	if in.Kind == "respuesta_boton" {
		return s.button(ctx, tallerID, in, c)
	}
	// Autorización antes de procesar nada (HU-85, RF-22). Sin ella el mensaje no pasa por la IA (Q-09).
	if c == nil || c.Consent == "pendiente" {
		asked, err := s.taller.ConsentAsked(ctx, tallerID, in.ConversationID)
		if err != nil || asked {
			return err
		}
		name, _, _, err := s.taller.Workshop(ctx, tallerID)
		if err != nil {
			return err
		}
		_, err = s.taller.Send(ctx, tallerID, in.Phone, sprintf(consentText, name),
			[]Button{{ID: "consent:si", Title: "Acepto"}, {ID: "consent:no", Title: "No acepto"}}, "consentimiento", nil)
		return err
	}
	if c.Consent == "rechazada" || in.Mode == "humano" {
		return nil // lo atiende una persona desde la conversación
	}
	return s.process(ctx, tallerID, in, *c)
}

func (s *Service) button(ctx context.Context, tallerID shared.ID, in Inbound, c *Customer) error {
	switch {
	case in.ButtonID == "consent:si" || in.ButtonID == "consent:no":
		granted := in.ButtonID == "consent:si"
		if c == nil {
			name := in.ProfileName
			if strings.TrimSpace(name) == "" {
				name = "Cliente WhatsApp"
			}
			nc, err := s.taller.RegisterCustomer(ctx, tallerID, in.Phone, name)
			if err != nil {
				return err
			}
			c = &nc
		}
		if err := s.taller.SetConsent(ctx, tallerID, c.ID, granted, in.ID.String(), in.At); err != nil {
			return err
		}
		if err := s.taller.LinkCustomer(ctx, tallerID, in.ConversationID, c.ID); err != nil {
			return err
		}
		if !granted {
			if err := s.taller.SetMode(ctx, tallerID, in.ConversationID, "humano", true); err != nil {
				return err
			}
			_, err := s.taller.Send(ctx, tallerID, in.Phone, noConsent, nil, "consentimiento", nil)
			return err
		}
		c.Consent = "autorizada"
		if _, err := s.taller.Send(ctx, tallerID, in.Phone, thanksText, nil, "consentimiento", nil); err != nil {
			return err
		}
		// Se procesa el mensaje que el cliente escribió antes de autorizar.
		prev, err := s.taller.PreviousText(ctx, tallerID, in.ConversationID, in.At)
		if err != nil || prev == nil {
			return err
		}
		return s.process(ctx, tallerID, *prev, *c)
	case strings.HasPrefix(in.ButtonID, "quote:aprobar:") || strings.HasPrefix(in.ButtonID, "quote:dudas:"):
		approve := strings.HasPrefix(in.ButtonID, "quote:aprobar:")
		oid, err := shared.ParseID(in.ButtonID[strings.LastIndex(in.ButtonID, ":")+1:])
		if err != nil || c == nil {
			return nil
		}
		decision, reply := "dudas", quoteAsk
		if approve {
			decision, reply = "aprobar", quoteOK
		}
		if err := s.taller.RespondQuote(ctx, tallerID, oid, decision, in.ID.String()); err != nil {
			return err
		}
		if !approve {
			if err := s.taller.SetMode(ctx, tallerID, in.ConversationID, "humano", true); err != nil {
				return err
			}
		}
		_, err = s.taller.Send(ctx, tallerID, in.Phone, reply, nil, "AU-02", &oid)
		return err
	case strings.HasPrefix(in.ButtonID, "recibo:ok:") || strings.HasPrefix(in.ButtonID, "recibo:revisar:"):
		return s.receipt(ctx, tallerID, in, c)
	case strings.HasPrefix(in.ButtonID, "estado:"):
		oid, err := shared.ParseID(strings.TrimPrefix(in.ButtonID, "estado:"))
		if err != nil || c == nil {
			return nil
		}
		o, err := s.taller.Order(ctx, tallerID, oid)
		if err != nil {
			return err
		}
		return s.status(ctx, tallerID, in, *c, o)
	case in.ButtonID == "humano":
		return s.human(ctx, tallerID, in)
	}
	return nil
}

// receipt registra la respuesta a la constancia (HU-19). "Algo no cuadra" pasa la conversación a una persona y la
// orden queda marcada para el dueño. Si el cliente ya había respondido, no se repite nada.
func (s *Service) receipt(ctx context.Context, tallerID shared.ID, in Inbound, c *Customer) error {
	ok := strings.HasPrefix(in.ButtonID, "recibo:ok:")
	oid, err := shared.ParseID(in.ButtonID[strings.LastIndex(in.ButtonID, ":")+1:])
	if err != nil || c == nil {
		return nil
	}
	changed, err := s.taller.AnswerReceipt(ctx, tallerID, oid, ok, in.ID.String())
	if err != nil || !changed {
		return err
	}
	reply := receiptOK
	if !ok {
		reply = receiptAsk
		if err := s.taller.SetMode(ctx, tallerID, in.ConversationID, "humano", true); err != nil {
			return err
		}
	}
	_, err = s.taller.Send(ctx, tallerID, in.Phone, reply, nil, "AU-01", &oid)
	return err
}

func (s *Service) human(ctx context.Context, tallerID shared.ID, in Inbound) error {
	if err := s.taller.SetMode(ctx, tallerID, in.ConversationID, "humano", true); err != nil {
		return err
	}
	_, err := s.taller.Send(ctx, tallerID, in.Phone, humanText, nil, "AU-04", nil)
	return err
}

func (s *Service) status(ctx context.Context, tallerID shared.ID, in Inbound, c Customer, o Order) error {
	text, on, err := s.taller.StatusReply(ctx, tallerID, o, c)
	if err != nil {
		return err
	}
	if !on { // AU-05 apagada: queda como borrador para el dueño
		return s.saveDraft(ctx, tallerID, in, "estado", text, nil, nil, "plantilla")
	}
	oid := o.ID
	_, err = s.taller.Send(ctx, tallerID, in.Phone, text, nil, "AU-05", &oid)
	return err
}

// process clasifica y responde: plantillas exactas salen solas; todo lo demás queda como borrador.
func (s *Service) process(ctx context.Context, tallerID shared.ID, in Inbound, c Customer) error {
	on, err := s.taller.AgentOn(ctx, tallerID)
	if err != nil || !on {
		return err
	}
	name, open, nextOpening, err := s.taller.Workshop(ctx, tallerID)
	if err != nil {
		return err
	}
	if in.Kind == "audio" { // HU-60: sin transcripción todavía (proveedor Sin dato)
		return s.saveDraft(ctx, tallerID, in, "nota_de_voz", "Recibimos una nota de voz. Escúchala en la conversación y responde tú.", nil, nil, "plantilla")
	}
	if in.Kind != "texto" {
		return nil
	}
	services, parts, err := s.taller.Catalog(ctx, tallerID)
	if err != nil {
		return err
	}
	attends, err := s.taller.Attends(ctx, tallerID)
	if err != nil {
		return err
	}
	u, ok, err := s.interpret(ctx, tallerID, in, InterpretRequest{Text: in.Text, Workshop: name, Attends: attends,
		Catalog: domain.Names(services, parts)})
	if err != nil {
		return err
	}
	if !ok { // salida inválida, error o tiempo vencido: no sale nada de la IA (HU-54)
		return s.saveDraft(ctx, tallerID, in, "otro", cantAnswer, nil, nil, "plantilla")
	}
	intent := u.ToIntent(in.Text)
	if !open { // HU-61: aviso de fuera de horario (una vez por mensaje) y se sigue con el borrador
		if _, err := s.taller.Send(ctx, tallerID, in.Phone, sprintf(closedTxt, nextOpening), nil, "AU-04", nil); err != nil {
			return err
		}
	}
	switch intent.Kind {
	case "humano":
		return s.human(ctx, tallerID, in)
	case "fuera_tema": // HU-58: borrador de plantilla; el dueño decide si lo envía
		return s.saveDraft(ctx, tallerID, in, "fuera_tema", sprintf(offTopicTxt, name), nil, nil, "plantilla")
	case "estado":
		orders, err := s.taller.OpenOrders(ctx, tallerID, c.ID)
		if err != nil {
			return err
		}
		if intent.Plate != "" {
			for _, o := range orders {
				if o.Plate == intent.Plate {
					return s.status(ctx, tallerID, in, c, o)
				}
			}
		}
		switch {
		case len(orders) == 1:
			return s.status(ctx, tallerID, in, c, orders[0])
		case len(orders) > 1: // HU-57: botones con las placas; de 4 en adelante el canal manda una lista
			var btns []Button
			for i, o := range orders {
				if i == 10 {
					break
				}
				btns = append(btns, Button{ID: "estado:" + o.ID.String(), Title: o.Plate})
			}
			_, err := s.taller.Send(ctx, tallerID, in.Phone, whichText, btns, "AU-05", nil)
			return err
		}
	}
	var action *Action
	if intent.Plate != "" { // HU-55: la placa crea la orden "Por recibir" cuando el dueño envía
		orders, err := s.taller.OpenOrders(ctx, tallerID, c.ID)
		if err != nil {
			return err
		}
		exists := false
		for _, o := range orders {
			exists = exists || o.Plate == intent.Plate
		}
		if !exists {
			prev, _ := s.taller.PreviousText(ctx, tallerID, in.ConversationID, in.At)
			sym := in.Text
			if prev != nil {
				sym = prev.Text
			}
			action = &Action{CreateOrder: &struct {
				Plate   string `json:"placa"`
				Symptom string `json:"sintoma"`
			}{Plate: intent.Plate, Symptom: sym}}
			// La placa sola no repite la cotización: se confirma la recepción.
			if len(intent.Services)+len(intent.Others) == 0 {
				d := domain.Draft{Text: "¡Gracias " + firstName(c.Name) + "! Te dejamos registrada la placa " + intent.Plate + ". Te esperamos " +
					strings.TrimSuffix(nextOpening, ".") + "."}
				return s.saveDraft(ctx, tallerID, in, "placa", d.Text, d.Sources, action, "ia")
			}
		}
	}
	d := domain.BuildDraft(domain.DraftInput{CustomerName: c.Name, Workshop: name, Intent: intent, Services: services, Parts: parts,
		NextOpening: nextOpening, HasPlate: intent.Plate != "", Attends: attends})
	if err := domain.Validate(d.Text, append(services, parts...)); err != nil {
		// Lo que no valida no sale: pasa a "Lo contesto yo" (RF-17).
		return s.saveDraft(ctx, tallerID, in, intent.Kind, cantAnswer, nil, nil, "plantilla")
	}
	return s.saveDraft(ctx, tallerID, in, intent.Kind, d.Text, d.Sources, action, "ia")
}

func firstName(n string) string {
	if f := strings.Fields(n); len(f) > 0 {
		return f[0]
	}
	return n
}

// interpret hace la única llamada al intérprete (RNF-05) fuera de la transacción, con límite de 30 s, y la
// registra (HU-99). Con el tope mensual del taller agotado usa las reglas (sin costo). ok=false si la salida no
// cumple el esquema o la IA falló: el llamador deja la tarjeta para el dueño (HU-54).
func (s *Service) interpret(ctx context.Context, tallerID shared.ID, in Inbound, req InterpretRequest) (domain.Understanding, bool, error) {
	var spent int64
	if err := s.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		spent, err = s.repo.MonthCost(ctx, tallerID, monthStart(s.clock.Now()))
		return err
	}); err != nil {
		return domain.Understanding{}, false, err
	}
	ai, result := s.ai, "ok"
	if spent >= s.capUSD {
		ai, result = s.free, "tope"
	}
	start := time.Now()
	cctx, cancel := context.WithTimeout(ctx, AITimeout)
	r, err := ai.Interpret(cctx, req)
	cancel()
	latency := int(time.Since(start).Milliseconds())
	var u domain.Understanding
	switch {
	case err != nil && errors.Is(err, domain.ErrInvalidOutput):
		result = "invalido"
	case err != nil:
		result = "error"
	default:
		if u, err = domain.ParseUnderstanding(r.Raw, req.Text, req.Catalog); err != nil {
			result = "invalido"
		}
	}
	if r.Provider == "" {
		r.Provider = "desconocido"
	}
	if r.Model == "" {
		r.Model = "desconocido"
	}
	id := in.ID
	call := AICall{MessageID: &id, Provider: r.Provider, Model: r.Model, Purpose: "interpretar", Result: result, TokensIn: r.TokensIn,
		TokensOut: r.TokensOut, LatencyMs: latency, CostMicroUSD: r.CostMicroUSD}
	if lerr := s.uow.Do(ctx, func(ctx context.Context) error { return s.repo.LogCall(ctx, tallerID, call) }); lerr != nil {
		return u, false, lerr
	}
	if ctx.Err() != nil { // el worker se está cerrando: se reintenta después
		return u, false, ctx.Err()
	}
	return u, result == "ok" || result == "tope", nil
}

func monthStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func (s *Service) saveDraft(ctx context.Context, tallerID shared.ID, in Inbound, intent, text string, src []domain.Source, a *Action, origin string) error {
	if src == nil {
		src = []domain.Source{}
	}
	return s.uow.Do(ctx, func(ctx context.Context) error {
		return s.repo.InsertDraft(ctx, DraftRecord{ID: shared.NewID(), TallerID: tallerID, ConversationID: in.ConversationID, InboundID: in.ID,
			Intent: intent, Proposed: text, Sources: src, Action: a, State: "pendiente", Origin: origin, CreatedAt: s.clock.Now(),
			Phone: in.Phone, InboundText: in.Text})
	})
}

// Drafts lista la bandeja (HU-62).
func (s *Service) Drafts(ctx context.Context, tallerID shared.ID, onlyPending bool) ([]DraftRecord, error) {
	var out []DraftRecord
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.repo.Drafts(ctx, tallerID, onlyPending, 100)
		return err
	})
	return out, err
}

var (
	ErrDraftNF   = shared.NotFound("borrador_no_encontrado", "No encontramos ese borrador.")
	ErrResolved  = shared.Conflict("borrador_resuelto", "Otra persona ya resolvió este borrador.")
	ErrClosedWin = shared.Rule("fuera_de_ventana", "El cliente no ha escrito en 24 horas; WhatsApp no deja enviar texto libre sin plantilla.")
)

// Resolve aplica la decisión del dueño (HU-62, HU-63): enviar (tal cual o editado), lo contesto yo, descartar.
// Solo aquí sale texto propuesto por el agente, y siempre porque una persona pulsó Enviar (RF-18).
func (s *Service) Resolve(ctx context.Context, tallerID, draftID shared.ID, actor shared.Actor, decision string, edited *string) (DraftRecord, error) {
	var out DraftRecord
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		d, err := s.repo.Draft(ctx, tallerID, draftID)
		if err != nil {
			return err
		}
		if d.State != "pendiente" {
			return ErrResolved
		}
		now := s.clock.Now()
		state, final := "", (*string)(nil)
		switch decision {
		case "enviar":
			text := d.Proposed
			state = "enviado"
			if edited != nil && strings.TrimSpace(*edited) != "" && strings.TrimSpace(*edited) != d.Proposed {
				t := strings.TrimSpace(*edited)
				text, state = t, "editado_enviado"
			}
			services, parts, err := s.taller.Catalog(ctx, tallerID)
			if err != nil {
				return err
			}
			if state == "enviado" { // el texto del agente se vuelve a validar; el editado es del dueño
				if err := domain.Validate(text, append(services, parts...)); err != nil {
					return shared.Rule("borrador_invalido", "El borrador no pasó la validación de precios. Edítalo antes de enviar.")
				}
			}
			var orderID *shared.ID
			if d.Action != nil && d.Action.CreateOrder != nil {
				code, err := s.createOrder(ctx, tallerID, d)
				if err != nil {
					return err
				}
				if code != "" {
					text += " Tu orden es la " + code + "."
				}
			}
			ok, err := s.taller.Send(ctx, tallerID, d.Phone, text, nil, "AU-04", orderID)
			if err != nil {
				return err
			}
			if !ok {
				return ErrClosedWin
			}
			final = &text
		case "lo_contesto_yo":
			state = "lo_contesto_yo"
			if err := s.taller.SetMode(ctx, tallerID, d.ConversationID, "humano", false); err != nil {
				return err
			}
		case "descartar":
			state = "descartado"
		default:
			return shared.Validation("decision_invalida", "La decisión debe ser enviar, lo_contesto_yo o descartar.")
		}
		ok, err := s.repo.Resolve(ctx, tallerID, draftID, state, final, actor.ID, now)
		if err != nil {
			return err
		}
		if !ok {
			return ErrResolved
		}
		if err := s.audit.Record(ctx, audit.Entry{TallerID: tallerID, Actor: actor, Action: "borrador." + state, Entity: "borrador",
			EntityID: draftID, Before: map[string]any{"propuesto": d.Proposed}, After: map[string]any{"final": final}, OccurredAt: now}); err != nil {
			return err
		}
		d.State, d.Final, d.ResolvedAt = state, final, &now
		out = d
		return nil
	})
	return out, err
}

func (s *Service) createOrder(ctx context.Context, tallerID shared.ID, d DraftRecord) (string, error) {
	in, err := s.taller.Message(ctx, tallerID, d.InboundID)
	if err != nil {
		return "", err
	}
	c, err := s.taller.CustomerByPhone(ctx, tallerID, in.Phone)
	if err != nil || c == nil {
		return "", err
	}
	return s.taller.CreateOrderFromPlate(ctx, tallerID, c.ID, d.Action.CreateOrder.Plate, d.Action.CreateOrder.Symptom)
}

// Stats es qué tan seguido el dueño edita los borradores (HU-65).
func (s *Service) Stats(ctx context.Context, tallerID shared.ID) (sent, edited int, err error) {
	err = s.uow.Do(ctx, func(ctx context.Context) error {
		var e error
		sent, edited, e = s.repo.EditStats(ctx, tallerID, s.clock.Now().AddDate(0, 0, -30))
		return e
	})
	return sent, edited, err
}

var sprintf = fmt.Sprintf
