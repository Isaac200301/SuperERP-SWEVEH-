// Package app contiene los casos de uso de mensajería: entrada (webhook), salida (con ventana de 24 h)
// y lectura de conversaciones.
package app

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"sweveh/internal/messaging/domain"
	"sweveh/internal/shared"
)

// Channel es el puerto CanalMensajeria (RNF-07): cambiar del número de prueba al del taller no toca el resto.
type Channel interface {
	SendText(ctx context.Context, phoneNumberID, to, text string) (string, error)
	SendButtons(ctx context.Context, phoneNumberID, to, text string, buttons []domain.Button) (string, error)
}

// Conversation es una conversación con un celular.
type Conversation struct {
	ID          shared.ID
	TallerID    shared.ID
	CustomerID  *shared.ID
	Phone       string
	LastInbound *time.Time
	Mode        string
	WantsHuman  bool
}

// Message es un mensaje guardado.
type Message struct {
	ID             shared.ID
	TallerID       shared.ID
	ConversationID shared.ID
	Direction      string
	WAMID          *string
	Kind           string
	Body           string
	Payload        json.RawMessage
	State          string
	Origin         string
	Category       *string
	OrderID        *shared.ID
	Automation     *string
	Error          *string
	CreatedAt      time.Time
	SentAt         *time.Time
}

// ChannelInfo es el número del taller.
type ChannelInfo struct {
	TallerID      shared.ID
	Provider      string
	PhoneNumberID string
	Visible       *string
}

type Repository interface {
	SaveWebhook(ctx context.Context, body []byte) error
	PendingWebhooks(ctx context.Context, limit int) ([]WebhookEvent, error)
	MarkWebhook(ctx context.Context, id shared.ID, errMsg *string) error
	ChannelByNumber(ctx context.Context, phoneNumberID string) (ChannelInfo, bool, error)
	ChannelOf(ctx context.Context, tallerID shared.ID) (ChannelInfo, bool, error)
	UpsertChannel(ctx context.Context, c ChannelInfo) error
	Conversation(ctx context.Context, tallerID shared.ID, phone string) (Conversation, error) // crea si no existe
	ConversationByID(ctx context.Context, tallerID, id shared.ID) (Conversation, error)
	TouchInbound(ctx context.Context, tallerID, convID shared.ID, at time.Time, customerID *shared.ID) error
	SetMode(ctx context.Context, tallerID, convID shared.ID, mode string, wantsHuman bool) error
	InsertMessage(ctx context.Context, m Message) (bool, error) // false si el wamid ya existía
	GetMessage(ctx context.Context, tallerID, id shared.ID) (Message, error)
	UpdateDelivery(ctx context.Context, tallerID, id shared.ID, state string, wamid *string, errMsg *string, at time.Time) error
	StatusByWAMID(ctx context.Context, wamid, state string) error
	Messages(ctx context.Context, tallerID, convID shared.ID, limit int) ([]Message, error)
	MessagesByOrder(ctx context.Context, tallerID, orderID shared.ID) ([]Message, error)
	Conversations(ctx context.Context, tallerID shared.ID, limit int) ([]ConversationSummary, error)
	ResponseMinutes(ctx context.Context, tallerID shared.ID, since time.Time) ([]float64, error)
}

// WebhookEvent es un cuerpo crudo recibido de Meta.
type WebhookEvent struct {
	ID   shared.ID
	Body json.RawMessage
}

// ConversationSummary es una fila de la lista de conversaciones.
type ConversationSummary struct {
	Conversation
	LastBody string
	LastAt   time.Time
}

type TxRunner interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
	DoWithSettings(ctx context.Context, settings map[string]string, fn func(ctx context.Context) error) error
}

type EventPublisher interface {
	Publish(ctx context.Context, tallerID shared.ID, e shared.Event) error
}

type Clock interface{ Now() time.Time }

type Service struct {
	tx      TxRunner
	repo    Repository
	channel Channel
	events  EventPublisher
	clock   Clock
}

func NewService(tx TxRunner, r Repository, ch Channel, e EventPublisher, c Clock) *Service {
	return &Service{tx: tx, repo: r, channel: ch, events: e, clock: c}
}

// ReceiveWebhook guarda el cuerpo crudo y responde rápido; el worker lo procesa después (R-09).
func (s *Service) ReceiveWebhook(ctx context.Context, body []byte) error {
	return s.tx.DoWithSettings(ctx, nil, func(ctx context.Context) error { return s.repo.SaveWebhook(ctx, body) })
}

// Inbound es un mensaje entrante ya interpretado.
type Inbound struct {
	PhoneNumberID string
	From          string // E.164
	WAMID         string
	Kind          string // texto | respuesta_boton | audio | imagen
	Text          string
	ButtonID      string
	At            time.Time
	Name          string
}

// ProcessWebhooks interpreta los cuerpos pendientes (los llama el worker).
func (s *Service) ProcessWebhooks(ctx context.Context) (int, error) {
	var evs []WebhookEvent
	if err := s.tx.DoWithSettings(ctx, nil, func(ctx context.Context) error {
		var err error
		evs, err = s.repo.PendingWebhooks(ctx, 50)
		return err
	}); err != nil {
		return 0, err
	}
	for _, ev := range evs {
		ins, statuses, perr := ParseMeta(ev.Body)
		var msg *string
		if perr == nil {
			for _, in := range ins {
				if err := s.storeInbound(ctx, in); err != nil {
					e := err.Error()
					msg = &e
					break
				}
			}
			for _, st := range statuses {
				tid, err := s.tallerOf(ctx, st.PhoneNumberID)
				if err != nil || tid == shared.NilID {
					continue
				}
				_ = s.tx.DoWithSettings(ctx, map[string]string{"app.taller_id": tid.String()}, func(ctx context.Context) error {
					return s.repo.StatusByWAMID(ctx, st.WAMID, st.State)
				})
			}
		} else {
			e := perr.Error()
			msg = &e
		}
		if err := s.tx.DoWithSettings(ctx, nil, func(ctx context.Context) error { return s.repo.MarkWebhook(ctx, ev.ID, msg) }); err != nil {
			return 0, err
		}
	}
	return len(evs), nil
}

// storeInbound resuelve el taller por el número, guarda el mensaje (una vez por wamid) y publica el evento.
// tallerOf resuelve el taller dueño de un número (solo se ve esa fila del canal, por RLS).
func (s *Service) tallerOf(ctx context.Context, pnid string) (shared.ID, error) {
	var tallerID shared.ID
	err := s.tx.DoWithSettings(ctx, map[string]string{"app.phone_number_id": pnid}, func(ctx context.Context) error {
		ch, ok, err := s.repo.ChannelByNumber(ctx, pnid)
		if err != nil || !ok {
			return err
		}
		tallerID = ch.TallerID
		return nil
	})
	return tallerID, err
}

func (s *Service) storeInbound(ctx context.Context, in Inbound) error {
	tallerID, err := s.tallerOf(ctx, in.PhoneNumberID)
	if err != nil {
		return err
	}
	if tallerID == shared.NilID {
		return nil // número desconocido: se descarta (no es de ningún taller)
	}
	return s.tx.DoWithSettings(ctx, map[string]string{"app.taller_id": tallerID.String()}, func(ctx context.Context) error {
		conv, err := s.repo.Conversation(ctx, tallerID, in.From)
		if err != nil {
			return err
		}
		payload, _ := json.Marshal(map[string]any{"boton": in.ButtonID, "nombre": in.Name})
		wamid := in.WAMID
		m := Message{ID: shared.NewID(), TallerID: tallerID, ConversationID: conv.ID, Direction: "entrante", WAMID: &wamid,
			Kind: in.Kind, Body: in.Text, Payload: payload, State: "recibido", Origin: "cliente", CreatedAt: in.At}
		inserted, err := s.repo.InsertMessage(ctx, m)
		if err != nil || !inserted {
			return err // reintento de Meta: ya se guardó
		}
		if err := s.repo.TouchInbound(ctx, tallerID, conv.ID, in.At, nil); err != nil {
			return err
		}
		return s.events.Publish(ctx, tallerID, shared.Event{Type: "messaging.mensaje_recibido", AggregateID: m.ID, OccurredAt: s.clock.Now(),
			Payload: map[string]any{"conversacion_id": conv.ID.String(), "celular": in.From, "tipo": in.Kind, "boton": in.ButtonID,
				"nombre": in.Name}})
	})
}

// Outgoing es un mensaje que se quiere enviar.
type Outgoing struct {
	TallerID   shared.ID
	Phone      string // E.164
	Text       string
	Buttons    []domain.Button
	Origin     string // automatizacion | agente | persona
	OrderID    *shared.ID
	Automation *string
	// Template indica que el texto está aprobado como plantilla de utilidad y puede salir fuera de la ventana.
	// Hoy no hay plantillas aprobadas (Q-07/Q-17, Sin dato), así que fuera de la ventana no sale.
	Template bool
}

var ErrOutsideWindow = shared.Rule("fuera_de_ventana", "El cliente no ha escrito en las últimas 24 horas; este aviso necesita una plantilla aprobada por Meta.")
var ErrEmptyText = shared.Validation("mensaje_vacio", "Escribe el mensaje.")

// Enqueue guarda el mensaje como pendiente y publica el evento de envío, en la transacción del llamador.
// Respeta la ventana de 24 h (HU-47): fuera de ella devuelve ErrOutsideWindow sin guardar.
func (s *Service) Enqueue(ctx context.Context, o Outgoing) (Message, error) {
	if strings.TrimSpace(o.Text) == "" {
		return Message{}, ErrEmptyText
	}
	if err := domain.ValidateButtons(o.Buttons); err != nil {
		return Message{}, err
	}
	var out Message
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		conv, err := s.repo.Conversation(ctx, o.TallerID, o.Phone)
		if err != nil {
			return err
		}
		if !domain.InWindow(conv.LastInbound, s.clock.Now()) && !o.Template {
			return ErrOutsideWindow
		}
		kind := "texto"
		if len(o.Buttons) > 0 {
			kind = "botones"
		}
		payload, _ := json.Marshal(map[string]any{"botones": o.Buttons})
		cat := "servicio"
		m := Message{ID: shared.NewID(), TallerID: o.TallerID, ConversationID: conv.ID, Direction: "saliente", Kind: kind, Body: o.Text,
			Payload: payload, State: "pendiente", Origin: o.Origin, Category: &cat, OrderID: o.OrderID, Automation: o.Automation, CreatedAt: s.clock.Now()}
		if _, err := s.repo.InsertMessage(ctx, m); err != nil {
			return err
		}
		out = m
		return s.events.Publish(ctx, o.TallerID, shared.Event{Type: "messaging.enviar", AggregateID: m.ID, OccurredAt: m.CreatedAt})
	})
	return out, err
}

// Deliver envía un mensaje pendiente por el canal (lo llama el worker, fuera de la transacción del cambio).
func (s *Service) Deliver(ctx context.Context, tallerID, msgID shared.ID) error {
	var (
		m    Message
		conv Conversation
		ch   ChannelInfo
		ok   bool
	)
	if err := s.tx.Do(ctx, func(ctx context.Context) error {
		var err error
		if m, err = s.repo.GetMessage(ctx, tallerID, msgID); err != nil {
			return err
		}
		if conv, err = s.repo.ConversationByID(ctx, tallerID, m.ConversationID); err != nil {
			return err
		}
		ch, ok, err = s.repo.ChannelOf(ctx, tallerID)
		return err
	}); err != nil {
		return err
	}
	if m.State != "pendiente" {
		return nil // idempotente: ya se envió
	}
	now := s.clock.Now()
	if !ok {
		e := "el taller no tiene número de WhatsApp conectado"
		return s.tx.Do(ctx, func(ctx context.Context) error {
			return s.repo.UpdateDelivery(ctx, tallerID, msgID, "fallido", nil, &e, now)
		})
	}
	var p struct {
		Buttons []domain.Button `json:"botones"`
	}
	_ = json.Unmarshal(m.Payload, &p)
	var wamid string
	var err error
	if len(p.Buttons) > 0 {
		wamid, err = s.channel.SendButtons(ctx, ch.PhoneNumberID, conv.Phone, m.Body, p.Buttons)
	} else {
		wamid, err = s.channel.SendText(ctx, ch.PhoneNumberID, conv.Phone, m.Body)
	}
	state, errMsg := "enviado", (*string)(nil)
	if err != nil {
		e := err.Error()
		state, errMsg = "fallido", &e
	}
	var w *string
	if wamid != "" {
		w = &wamid
	}
	if uerr := s.tx.Do(ctx, func(ctx context.Context) error {
		return s.repo.UpdateDelivery(ctx, tallerID, msgID, state, w, errMsg, now)
	}); uerr != nil {
		return uerr
	}
	return err // el worker reintenta con retroceso si falló el canal
}

// EnsureLocalChannel conecta el canal de prueba local al taller si no tiene número (demo sin cuenta de Meta).
func (s *Service) EnsureLocalChannel(ctx context.Context, tallerID shared.ID) (ChannelInfo, error) {
	var out ChannelInfo
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		ch, ok, err := s.repo.ChannelOf(ctx, tallerID)
		if err != nil {
			return err
		}
		if ok {
			out = ch
			return nil
		}
		out = ChannelInfo{TallerID: tallerID, Provider: "prueba_local", PhoneNumberID: "local-" + tallerID.String()}
		return s.repo.UpsertChannel(ctx, out)
	})
	return out, err
}

// SetChannel cambia el número del taller (HU-101): del de prueba al del taller sin tocar nada más.
func (s *Service) SetChannel(ctx context.Context, c ChannelInfo) error {
	return s.tx.Do(ctx, func(ctx context.Context) error { return s.repo.UpsertChannel(ctx, c) })
}

func (s *Service) Channel(ctx context.Context, tallerID shared.ID) (ChannelInfo, bool, error) {
	var (
		out ChannelInfo
		ok  bool
	)
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		var err error
		out, ok, err = s.repo.ChannelOf(ctx, tallerID)
		return err
	})
	return out, ok, err
}

// SetMode pasa la conversación a "humano" (HU-59) o la devuelve al agente.
func (s *Service) SetMode(ctx context.Context, tallerID, convID shared.ID, mode string, wantsHuman bool) error {
	return s.tx.Do(ctx, func(ctx context.Context) error { return s.repo.SetMode(ctx, tallerID, convID, mode, wantsHuman) })
}

func (s *Service) Conversations(ctx context.Context, tallerID shared.ID) ([]ConversationSummary, error) {
	var out []ConversationSummary
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.repo.Conversations(ctx, tallerID, 100)
		return err
	})
	return out, err
}

func (s *Service) Messages(ctx context.Context, tallerID, convID shared.ID) (Conversation, []Message, error) {
	var (
		c   Conversation
		out []Message
	)
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		var err error
		if c, err = s.repo.ConversationByID(ctx, tallerID, convID); err != nil {
			return err
		}
		out, err = s.repo.Messages(ctx, tallerID, convID, 200)
		return err
	})
	return c, out, err
}

// ByOrder es la conversación de WhatsApp dentro de la orden (HU-24).
func (s *Service) ByOrder(ctx context.Context, tallerID, orderID shared.ID) ([]Message, error) {
	var out []Message
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.repo.MessagesByOrder(ctx, tallerID, orderID)
		return err
	})
	return out, err
}

func (s *Service) GetMessage(ctx context.Context, tallerID, id shared.ID) (Message, error) {
	var out Message
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.repo.GetMessage(ctx, tallerID, id)
		return err
	})
	return out, err
}

func (s *Service) ConversationByID(ctx context.Context, tallerID, id shared.ID) (Conversation, error) {
	var out Conversation
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.repo.ConversationByID(ctx, tallerID, id)
		return err
	})
	return out, err
}

// LinkCustomer liga la conversación al cliente (al registrarlo o al autorizar).
func (s *Service) LinkCustomer(ctx context.Context, tallerID, convID, customerID shared.ID) error {
	return s.tx.Do(ctx, func(ctx context.Context) error {
		c, err := s.repo.ConversationByID(ctx, tallerID, convID)
		if err != nil {
			return err
		}
		at := s.clock.Now()
		if c.LastInbound != nil {
			at = *c.LastInbound
		}
		return s.repo.TouchInbound(ctx, tallerID, convID, at, &customerID)
	})
}

// ResponseMinutes devuelve los minutos que tardó el taller en contestar cada mensaje (HU-73).
func (s *Service) ResponseMinutes(ctx context.Context, tallerID shared.ID, since time.Time) ([]float64, error) {
	var out []float64
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.repo.ResponseMinutes(ctx, tallerID, since)
		return err
	})
	return out, err
}
