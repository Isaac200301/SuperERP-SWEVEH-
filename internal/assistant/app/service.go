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
	// VoiceNote descarga del canal el audio de una nota de voz (HU-60).
	VoiceNote(ctx context.Context, tallerID, msgID shared.ID) (audio []byte, mime string, err error)
	// SetTranscript guarda la transcripción como texto del mensaje: la conversación la muestra junto al audio (HU-60).
	SetTranscript(ctx context.Context, tallerID, msgID shared.ID, text string) error
	// RespondQuoteItems aprueba solo los ítems con esos números (1 = el primero del mensaje, HU-30). ok=false si algún
	// número no está en la cotización; entonces no registra nada.
	RespondQuoteItems(ctx context.Context, tallerID, orderID shared.ID, picks []int, ref string) (res QuotePick, ok bool, err error)
	// AnswerReceipt guarda "Está bien" (ok) o "Algo no cuadra" de la constancia; false si ya había respuesta.
	AnswerReceipt(ctx context.Context, tallerID, orderID shared.ID, ok bool, ref string) (bool, error)
	// PlateOf dice de quién es la placa en el taller: "nueva", "propia" (con su orden abierta, si tiene) o "ajena".
	PlateOf(ctx context.Context, tallerID, customerID shared.ID, plate string) (PlateInfo, error)
	// CreateOrderFromPlate abre la orden "Por recibir" (HU-55) con la cita en la fecha prometida si viene when (AU-06
	// la recuerda ese día). Nunca duplica ni toca un vehículo ajeno: si la placa ya tiene orden abierta devuelve esa
	// (y si está "Por recibir" y viene when, la reagenda); si es de otro cliente no crea nada (Owner "ajena").
	CreateOrderFromPlate(ctx context.Context, tallerID, customerID shared.ID, plate, symptom string, when *time.Time) (PlateInfo, error)
	// OpenAt dice si el taller atiende a esa hora y cuál es su horario ese día ("" = no abre ese día).
	OpenAt(ctx context.Context, tallerID shared.ID, at time.Time) (bool, string, error)
	// SetCustomerName pone el nombre que dio el cliente si el que tiene es de relleno; false = no cambió.
	SetCustomerName(ctx context.Context, tallerID, customerID shared.ID, name string) (bool, error)

	Catalog(ctx context.Context, tallerID shared.ID) ([]domain.Priced, []domain.Priced, error) // servicios, repuestos
	Workshop(ctx context.Context, tallerID shared.ID) (name string, open bool, nextOpening string, err error)
	Attends(ctx context.Context, tallerID shared.ID) (string, error) // carro | moto | ambos

	// Para el asesor (ADR-0029).
	Profile(ctx context.Context, tallerID shared.ID) (WorkshopProfile, error)
	Vehicles(ctx context.Context, tallerID, customerID shared.ID) ([]Vehicle, error)
	History(ctx context.Context, tallerID, convID shared.ID, before time.Time, limit int) ([]Turn, error)
	Showcase(ctx context.Context, tallerID shared.ID, q string) ([]Product, error)
	// LastClientPhoto es la referencia de la última foto de la orden marcada para el cliente ("" = no hay). HU-56.
	LastClientPhoto(ctx context.Context, tallerID, orderID shared.ID) (string, error)
	// SendMedia envía una foto (ref "repuesto:<id>" o "evidencia:<orden>:<id>") con un pie; false = ventana cerrada.
	SendMedia(ctx context.Context, tallerID shared.ID, phone, caption, ref string) (bool, error)
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
	Voice          bool // era una nota de voz: Text es su transcripción (HU-60)
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
	Kind       string // carro | moto
	State      string
	PromisedAt *time.Time
}

type Button struct{ ID, Title string }

// PlateInfo es lo que se sabe de una placa para este cliente. Owner: nueva | propia | ajena. Open es la orden abierta
// del cliente con esa placa; Code, el número de la orden creada o encontrada.
type PlateInfo struct {
	Owner       string
	Open        *Order
	Code        string
	Rescheduled bool // la orden "Por recibir" quedó con la nueva fecha
}

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
	Voice          bool // el cliente mandó una nota de voz (HU-60)
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
	Catalog                 []domain.CatalogItem
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
	InsertDraft(ctx context.Context, d DraftRecord) (bool, error) // false si el mensaje ya tenía borrador
	// AnonymizePhone borra el celular y los textos de los borradores de ese celular (HU-87).
	AnonymizePhone(ctx context.Context, tallerID shared.ID, phone string) (int, error)
	AutoReply(ctx context.Context, tallerID shared.ID) (bool, error)
	SetAutoReply(ctx context.Context, tallerID shared.ID, on bool, at time.Time) error
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
	uow     UnitOfWork
	repo    Repository
	stt     Transcriber // notas de voz (HU-60); nil = sin transcripción
	taller  Taller
	audit   AuditLog
	clock   Clock
	ai      Interpreter
	free    Interpreter // reglas: respaldo sin costo cuando se agota el tope mensual
	capUSD  int64
	advisor Advisor // asesor con tools (ADR-0029); nil = solo el flujo de reglas
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
	plateAsk    = "¡Gracias! Una persona del taller revisa esa placa y te escribe en un momento."
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
	if done, err := s.quotePick(ctx, tallerID, in, *c); done || err != nil {
		return err
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
	case strings.HasPrefix(in.ButtonID, "quote:elegir:"): // HU-30: el cliente escribe los números de lo que aprueba
		oid, err := shared.ParseID(strings.TrimPrefix(in.ButtonID, "quote:elegir:"))
		if err != nil || c == nil {
			return nil
		}
		_, err = s.taller.Send(ctx, tallerID, in.Phone, quotePickAsk, nil, "AU-02", &oid)
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
	if _, err = s.taller.Send(ctx, tallerID, in.Phone, text, nil, "AU-05", &oid); err != nil {
		return err
	}
	return s.sendLastPhoto(ctx, tallerID, in.Phone, o)
}

// sendLastPhoto manda, después del estado, la última foto de la orden marcada para el cliente (HU-56, criterio 1).
func (s *Service) sendLastPhoto(ctx context.Context, tallerID shared.ID, phone string, o Order) error {
	ref, err := s.taller.LastClientPhoto(ctx, tallerID, o.ID)
	if err != nil || ref == "" {
		return err
	}
	_, err = s.taller.SendMedia(ctx, tallerID, phone, photoCaption(o), ref)
	return err
}

func photoCaption(o Order) string {
	return strings.TrimSpace("Así va tu " + o.Vehicle + " " + o.Plate)
}

// closedWindow es cuánto dura un aviso de fuera de horario: si ya se le dijo al cliente en ese lapso, no se repite
// aunque escriba varias veces en la noche (HU-61).
const closedWindow = 12 * time.Hour

// closedNotice manda el aviso de fuera de horario con la próxima apertura, salvo que ya haya salido en esta
// conversación en las últimas 12 horas.
func (s *Service) closedNotice(ctx context.Context, tallerID shared.ID, in Inbound, nextOpening string) error {
	history, err := s.taller.History(ctx, tallerID, in.ConversationID, in.At, historyTurns)
	if err != nil {
		return err
	}
	prefix := strings.SplitN(closedTxt, "%", 2)[0]
	for _, h := range history {
		if !h.FromCustomer && strings.HasPrefix(h.Text, prefix) && in.At.Sub(h.At) < closedWindow {
			return nil
		}
	}
	_, err = s.taller.Send(ctx, tallerID, in.Phone, sprintf(closedTxt, nextOpening), nil, "AU-04", nil)
	return err
}

// process clasifica y responde: plantillas exactas salen solas; las respuestas armadas con datos del taller salen
// solas con el envío automático (ADR-0028) o quedan como borrador. Las notas para el dueño siempre esperan.
func (s *Service) process(ctx context.Context, tallerID shared.ID, in Inbound, c Customer) error {
	on, err := s.taller.AgentOn(ctx, tallerID)
	if err != nil || !on {
		return err
	}
	name, open, nextOpening, err := s.taller.Workshop(ctx, tallerID)
	if err != nil {
		return err
	}
	if in.Kind == "audio" { // HU-60: se transcribe y sigue el mismo flujo del texto
		text, err := s.transcribe(ctx, tallerID, in)
		if err != nil {
			return err
		}
		if text == "" {
			return s.saveDraft(ctx, tallerID, in, "nota_de_voz", voiceNoteCard, nil, nil, "plantilla")
		}
		in.Kind, in.Text, in.Voice = "texto", text, true
	}
	if in.Kind != "texto" {
		return nil
	}
	handled, tried, err := s.advise(ctx, tallerID, in, c)
	if err != nil || handled {
		return err
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
		Catalog: domain.Items(services, parts)}, tried)
	if err != nil {
		return err
	}
	if !ok { // salida inválida, error o tiempo vencido: no sale nada de la IA (HU-54)
		return s.saveDraft(ctx, tallerID, in, "otro", cantAnswer, nil, nil, "plantilla")
	}
	intent := u.ToIntent(in.Text)
	if !open { // HU-61: aviso fijo de fuera de horario (sin IA), una vez por noche; el borrador queda para la mañana
		if err := s.closedNotice(ctx, tallerID, in, nextOpening); err != nil {
			return err
		}
	}
	switch intent.Kind {
	case "humano":
		return s.human(ctx, tallerID, in)
	case "fuera_tema": // HU-58: borrador de plantilla; el dueño decide si lo envía
		return s.answer(ctx, tallerID, in, "fuera_tema", sprintf(offTopicTxt, name), nil, nil, "plantilla")
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
	if intent.Plate != "" { // HU-55: la placa crea la orden "Por recibir" al enviar la respuesta
		p, err := s.taller.PlateOf(ctx, tallerID, c.ID, intent.Plate)
		if err != nil {
			return err
		}
		switch {
		case p.Owner == "ajena": // de otro cliente: no se crea nada ni se dice nada del vehículo (HU-12)
			return s.plateReview(ctx, tallerID, in)
		case p.Open != nil && len(intent.Services)+len(intent.Others) == 0: // ya tiene orden: se cuenta cómo va
			return s.status(ctx, tallerID, in, c, *p.Open)
		case p.Open == nil:
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
				d := domain.Draft{Text: domain.Greet("¡Gracias", c.Name) + " Te dejamos registrada la placa " + intent.Plate + ". Te esperamos " +
					strings.TrimSuffix(nextOpening, ".") + "."}
				return s.answer(ctx, tallerID, in, "placa", d.Text, d.Sources, action, "ia")
			}
		}
	}
	d := domain.BuildDraft(domain.DraftInput{CustomerName: c.Name, Workshop: name, Intent: intent, Services: services, Parts: parts,
		NextOpening: nextOpening, HasPlate: intent.Plate != "", Attends: attends})
	if err := domain.Validate(d.Text, append(services, parts...)); err != nil {
		// Lo que no valida no sale: pasa a "Lo contesto yo" (RF-17).
		return s.saveDraft(ctx, tallerID, in, intent.Kind, cantAnswer, nil, nil, "plantilla")
	}
	return s.answer(ctx, tallerID, in, intent.Kind, d.Text, d.Sources, action, "ia")
}

// plateReview: la placa es de otro cliente. No se crea nada ni se dice nada del vehículo; una persona del taller
// revisa (la conversación pasa a modo humano con alerta, HU-59).
func (s *Service) plateReview(ctx context.Context, tallerID shared.ID, in Inbound) error {
	if err := s.taller.SetMode(ctx, tallerID, in.ConversationID, "humano", true); err != nil {
		return err
	}
	_, err := s.taller.Send(ctx, tallerID, in.Phone, plateAsk, nil, "AU-04", nil)
	return err
}

// interpret hace la única llamada al intérprete (RNF-05) fuera de la transacción, con límite de 30 s, y la
// registra (HU-99). Con el tope mensual del taller agotado usa las reglas (sin costo). ok=false si la salida no
// cumple el esquema o la IA falló: el llamador deja la tarjeta para el dueño (HU-54).
func (s *Service) interpret(ctx context.Context, tallerID shared.ID, in Inbound, req InterpretRequest, rulesOnly bool) (domain.Understanding, bool, error) {
	var spent int64
	if err := s.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		spent, err = s.repo.MonthCost(ctx, tallerID, monthStart(s.clock.Now()))
		return err
	}); err != nil {
		return domain.Understanding{}, false, err
	}
	ai, result := s.ai, "ok"
	switch {
	case spent >= s.capUSD:
		ai, result = s.free, "tope"
	case rulesOnly: // el asesor ya llamó a la IA y falló: el respaldo es sin costo
		ai = s.free
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
		if u, err = domain.ParseUnderstanding(r.Raw, req.Text, domain.Names(req.Catalog)); err != nil {
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

// saveDraft deja una tarjeta para el dueño en "Por revisar". La usan las notas que no van al cliente
// ("Responde tú", nota de voz) y AU-05 apagada.
func (s *Service) saveDraft(ctx context.Context, tallerID shared.ID, in Inbound, intent, text string, src []domain.Source, a *Action, origin string) error {
	return s.uow.Do(ctx, func(ctx context.Context) error {
		_, err := s.repo.InsertDraft(ctx, s.draft(tallerID, in, intent, text, src, a, origin))
		return err
	})
}

// answer guarda la respuesta al cliente y, con el envío automático encendido (ADR-0028), la envía de una vez: queda
// "auto_enviado". La IA no redacta: el texto sale de plantillas con datos del taller y se valida antes de salir. Si no
// valida o la ventana de 24 h está cerrada, se queda pendiente para el dueño.
func (s *Service) answer(ctx context.Context, tallerID shared.ID, in Inbound, intent, text string, src []domain.Source, a *Action, origin string) error {
	d := s.draft(tallerID, in, intent, text, src, a, origin)
	return s.uow.Do(ctx, func(ctx context.Context) error {
		inserted, err := s.repo.InsertDraft(ctx, d)
		if err != nil || !inserted { // si el evento llegó otra vez, no se envía de nuevo
			return err
		}
		on, err := s.repo.AutoReply(ctx, tallerID)
		if err != nil || !on {
			return err
		}
		final, sent, err := s.deliver(ctx, tallerID, d, d.Proposed, true)
		if errors.Is(err, errInvalidDraft) || (err == nil && !sent) {
			return nil
		}
		if err != nil {
			return err
		}
		now := s.clock.Now()
		if _, err := s.repo.Resolve(ctx, tallerID, d.ID, "auto_enviado", &final, shared.ID{}, now); err != nil {
			return err
		}
		return s.audit.Record(ctx, audit.Entry{TallerID: tallerID, Actor: shared.Actor{Type: shared.ActorAgent}, Action: "borrador.auto_enviado",
			Entity: "borrador", EntityID: d.ID, After: map[string]any{"final": final}, OccurredAt: now})
	})
}

func (s *Service) draft(tallerID shared.ID, in Inbound, intent, text string, src []domain.Source, a *Action, origin string) DraftRecord {
	if src == nil {
		src = []domain.Source{}
	}
	return DraftRecord{ID: shared.NewID(), TallerID: tallerID, ConversationID: in.ConversationID, InboundID: in.ID,
		Intent: intent, Proposed: text, Sources: src, Action: a, State: "pendiente", Origin: origin, CreatedAt: s.clock.Now(),
		Phone: in.Phone, InboundText: in.Text, Voice: in.Voice || in.Kind == "audio"}
}

var errInvalidDraft = shared.Rule("borrador_invalido", "El borrador no pasó la validación de precios. Edítalo antes de enviar.")

// deliver envía un borrador: revalida el texto del agente (check), crea la orden de la placa (HU-55) y lo manda.
// sent=false si la ventana de 24 h está cerrada.
func (s *Service) deliver(ctx context.Context, tallerID shared.ID, d DraftRecord, text string, check bool) (string, bool, error) {
	if check {
		services, parts, err := s.taller.Catalog(ctx, tallerID)
		if err != nil {
			return "", false, err
		}
		if err := domain.Validate(text, append(services, parts...)); err != nil {
			return "", false, errInvalidDraft
		}
	}
	if d.Action != nil && d.Action.CreateOrder != nil {
		p, err := s.createOrder(ctx, tallerID, d)
		if err != nil {
			return "", false, err
		}
		if p.Code != "" {
			text += " Tu orden es la " + p.Code + "."
		}
	}
	ok, err := s.taller.Send(ctx, tallerID, d.Phone, text, nil, "AU-04", nil)
	return text, ok, err
}

// AutoReply dice si el agente responde solo en este taller (ADR-0028).
func (s *Service) AutoReply(ctx context.Context, tallerID shared.ID) (bool, error) {
	var on bool
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		on, err = s.repo.AutoReply(ctx, tallerID)
		return err
	})
	return on, err
}

// SetAutoReply enciende o apaga el envío automático. Apagado, todo vuelve a "Por revisar" (HU-67).
func (s *Service) SetAutoReply(ctx context.Context, tallerID shared.ID, actor shared.Actor, on bool) error {
	return s.uow.Do(ctx, func(ctx context.Context) error {
		before, err := s.repo.AutoReply(ctx, tallerID)
		if err != nil {
			return err
		}
		now := s.clock.Now()
		if err := s.repo.SetAutoReply(ctx, tallerID, on, now); err != nil {
			return err
		}
		return s.audit.Record(ctx, audit.Entry{TallerID: tallerID, Actor: actor, Action: "asistente.auto_envio", Entity: "taller",
			EntityID: tallerID, Before: map[string]any{"autoEnvio": before}, After: map[string]any{"autoEnvio": on}, OccurredAt: now})
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
			// El texto del agente se vuelve a validar; el editado es del dueño.
			text, ok, err := s.deliver(ctx, tallerID, d, text, state == "enviado")
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

func (s *Service) createOrder(ctx context.Context, tallerID shared.ID, d DraftRecord) (PlateInfo, error) {
	in, err := s.taller.Message(ctx, tallerID, d.InboundID)
	if err != nil {
		return PlateInfo{}, err
	}
	c, err := s.taller.CustomerByPhone(ctx, tallerID, in.Phone)
	if err != nil || c == nil {
		return PlateInfo{}, err
	}
	return s.taller.CreateOrderFromPlate(ctx, tallerID, c.ID, d.Action.CreateOrder.Plate, d.Action.CreateOrder.Symptom, nil)
}

// MonthCost es el costo de IA del mes en micro USD (HU-99), para el medidor del administrador (HU-74).
func (s *Service) MonthCost(ctx context.Context, tallerID shared.ID) (int64, error) {
	var c int64
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		c, err = s.repo.MonthCost(ctx, tallerID, monthStart(s.clock.Now()))
		return err
	})
	return c, err
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

// AnonymizePhone borra el celular y los textos de los borradores del titular (HU-87). La intención y el estado se
// conservan para las estadísticas de la bandeja.
func (s *Service) AnonymizePhone(ctx context.Context, tallerID shared.ID, phone string) (int, error) {
	var n int
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		n, err = s.repo.AnonymizePhone(ctx, tallerID, phone)
		return err
	})
	return n, err
}

// QuotePick es el resultado de aprobar solo algunos ítems (HU-30).
type QuotePick struct {
	Approved, Lines int
	Total           int64 // lo aprobado, con descuentos e IVA
}

// Textos de la aprobación por ítems (HU-30). Son plantillas, no IA.
const (
	quotePickAsk   = "Escribe los números de los ítems que apruebas, por ejemplo: 1 y 3."
	quotePickAgain = "No encontré esos números en la cotización. Escribe los de los ítems que apruebas, por ejemplo: 1 y 3."
	quotePickOK    = "Listo, aprobaste %d de %d ítems. El total queda en %s. Seguimos con lo que aprobaste."
)

// quotePick atiende la respuesta a "Elegir ítems" (HU-30): si lo último que le dijimos fue que escribiera los números
// y el texto son números, aprueba esos ítems de la cotización que espera respuesta y confirma el total. done=false si
// el mensaje no es eso y debe seguir el flujo normal.
func (s *Service) quotePick(ctx context.Context, tallerID shared.ID, in Inbound, c Customer) (bool, error) {
	if in.Kind != "texto" {
		return false, nil
	}
	picks, ok := domain.ParsePicks(in.Text)
	if !ok {
		return false, nil
	}
	history, err := s.taller.History(ctx, tallerID, in.ConversationID, in.At, 4)
	if err != nil {
		return false, err
	}
	var last *Turn
	for i := len(history) - 1; i >= 0; i-- {
		if !history[i].FromCustomer {
			last = &history[i]
			break
		}
	}
	if last == nil || (last.Text != quotePickAsk && last.Text != quotePickAgain) {
		return false, nil
	}
	orders, err := s.taller.OpenOrders(ctx, tallerID, c.ID)
	if err != nil {
		return false, err
	}
	var oid *shared.ID
	for _, o := range orders {
		if o.State == "esperando_aprobacion" {
			id := o.ID
			oid = &id
		}
	}
	if oid == nil {
		return false, nil
	}
	res, ok, err := s.taller.RespondQuoteItems(ctx, tallerID, *oid, picks, in.ID.String())
	if err != nil {
		return true, err
	}
	reply := quotePickAgain
	if ok {
		reply = sprintf(quotePickOK, res.Approved, res.Lines, domain.Money(res.Total))
	}
	_, err = s.taller.Send(ctx, tallerID, in.Phone, reply, nil, "AU-02", oid)
	return true, err
}

// Transcriber pasa una nota de voz a texto en español (HU-60, ADR-0033).
type Transcriber interface {
	Transcribe(ctx context.Context, audio []byte, mime string) (string, error)
}

// SetTranscriber conecta la transcripción de notas de voz (composition root). Sin ella, la nota de voz queda como
// tarjeta para que el dueño la escuche.
func (s *Service) SetTranscriber(t Transcriber) { s.stt = t }

const (
	// voiceNoteCard es la tarjeta cuando no hay transcripción (sin proveedor o si falló).
	voiceNoteCard = "Recibimos una nota de voz y no la pudimos transcribir. Escúchala en la conversación y responde tú."
	// transcribeTimeout acota la descarga y la transcripción, fuera de la transacción.
	transcribeTimeout = 60 * time.Second
	// maxTranscript: una nota de voz larga se recorta para la IA (el texto completo queda en la conversación).
	maxTranscript = 2000
)

// transcribe descarga la nota de voz y la transcribe (HU-60). Devuelve "" si no hay transcriptor, si el canal no
// tiene el audio o si la transcripción falla: entonces queda la tarjeta para el dueño. Solo un error de la base corta.
func (s *Service) transcribe(ctx context.Context, tallerID shared.ID, in Inbound) (string, error) {
	if s.stt == nil {
		return "", nil
	}
	tctx, cancel := context.WithTimeout(ctx, transcribeTimeout)
	defer cancel()
	audio, mime, err := s.taller.VoiceNote(tctx, tallerID, in.ID)
	if err != nil || len(audio) == 0 {
		return "", nil
	}
	text, err := s.stt.Transcribe(tctx, audio, mime)
	if text = strings.TrimSpace(text); err != nil || text == "" {
		return "", nil
	}
	if err := s.taller.SetTranscript(ctx, tallerID, in.ID, text); err != nil {
		return "", err
	}
	if r := []rune(text); len(r) > maxTranscript {
		text = string(r[:maxTranscript])
	}
	return text, nil
}
