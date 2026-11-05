// Package app contiene los casos de uso de mensajería: entrada (webhook), salida (con ventana de 24 h)
// y lectura de conversaciones.
package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"sweveh/internal/messaging/domain"
	"sweveh/internal/shared"
)

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
	// MoveCustomer pasa las conversaciones de un cliente a otro (unir clientes repetidos, HU-15).
	MoveCustomer(ctx context.Context, tallerID, from, to shared.ID) (int, error)
	// AnonymizeCustomer borra el celular y el texto de las conversaciones del cliente (HU-87).
	AnonymizeCustomer(ctx context.Context, tallerID, customerID shared.ID) (int, error)
	// MessagesOf son los mensajes de las conversaciones del cliente, en orden (copia de sus datos, HU-87).
	MessagesOf(ctx context.Context, tallerID, customerID shared.ID) ([]Message, error)
	// MessagesByPhone son los últimos mensajes de la conversación con ese celular (registro técnico, HU-100).
	MessagesByPhone(ctx context.Context, tallerID shared.ID, phone string, limit int) ([]Message, error)
	// WebhooksMentioning son los webhooks crudos recibidos desde since que mencionan esos dígitos (HU-100).
	WebhooksMentioning(ctx context.Context, digits string, since time.Time, limit int) ([]WebhookTrace, error)
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
	ResponseMinutes(ctx context.Context, tallerID shared.ID, since time.Time) ([]ResponseTime, error)
	// SentSince cuenta los mensajes que salieron desde since (no cuenta los fallidos ni los pendientes). HU-74.
	SentSince(ctx context.Context, tallerID shared.ID, since time.Time) (int, error)
}

// WebhookEvent es un cuerpo crudo recibido del proveedor de WhatsApp.
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
	parser  InboundParser
	events  EventPublisher
	clock   Clock
	media   MediaSource
}

func NewService(tx TxRunner, r Repository, ch Channel, p InboundParser, e EventPublisher, c Clock) *Service {
	return &Service{tx: tx, repo: r, channel: ch, parser: p, events: e, clock: c}
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
		ins, statuses, perr := s.parser.Parse(ev.Body)
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
		if in, err = s.menuReply(ctx, tallerID, conv.ID, in); err != nil {
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

// menuReply convierte un texto en la respuesta de un botón cuando el último mensaje de la conversación fue
// nuestro con opciones y el cliente contesta con su número o su título ("1", "acepto"). Los canales sin botones
// (WAHA, ADR-0027) las muestran como menú numerado; con botones reales, escribir la opción también vale.
func (s *Service) menuReply(ctx context.Context, tallerID, convID shared.ID, in Inbound) (Inbound, error) {
	if in.Kind != "texto" {
		return in, nil
	}
	last, err := s.repo.Messages(ctx, tallerID, convID, 1)
	if err != nil || len(last) == 0 || last[0].Direction != "saliente" || last[0].Kind != "botones" {
		return in, err
	}
	var p struct {
		Buttons []domain.Button `json:"botones"`
	}
	_ = json.Unmarshal(last[0].Payload, &p)
	if id, ok := domain.MatchReply(in.Text, p.Buttons); ok {
		in.Kind, in.ButtonID = "respuesta_boton", id
	}
	return in, nil
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
	// Media es una foto que va con el texto como pie (p. ej. "repuesto:<id>", vitrina ADR-0029). La resuelve
	// MediaSource al enviar; el mensaje solo guarda la referencia.
	Media string
}

// MediaSource trae la foto de una referencia de Outgoing.Media (la conecta el composition root).
type MediaSource interface {
	Media(ctx context.Context, tallerID shared.ID, ref string) (Image, error)
}

// SetMediaSource conecta de dónde salen las fotos que se envían.
func (s *Service) SetMediaSource(m MediaSource) { s.media = m }

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
		kind, p := "texto", map[string]any{"botones": o.Buttons}
		switch {
		case o.Media != "":
			kind, p = "imagen", map[string]any{"imagen": o.Media}
		case len(o.Buttons) > 0:
			kind = "botones"
		}
		payload, _ := json.Marshal(p)
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

// MaxDeliveryAttempts es cuántas veces se intenta un envío con fallas pasajeras (límite de tasa, caída de Meta,
// red) antes de darlo por fallido. Con el retroceso del outbox (1 s, 2 s, 4 s, 8 s…) son unos 15 s en total.
const MaxDeliveryAttempts = 5

// Deliver envía un mensaje pendiente por el canal (lo llama el worker, fuera de la transacción del cambio).
// attempt es el número de intento (0 = primero). Una falla pasajera deja el mensaje pendiente y devuelve el
// error para que el outbox reintente. Una falla definitiva (o agotar los intentos) lo marca fallido y publica
// "messaging.envio_fallido" en la misma transacción, para que la automatización quede fallida y el dueño lo vea
// (HU-46); la orden no se entera (RNF-13).
func (s *Service) Deliver(ctx context.Context, tallerID, msgID shared.ID, attempt int) error {
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
		return nil // idempotente: ya se envió o ya se dio por fallido
	}
	if !ok {
		return s.failed(ctx, m, errors.New("el taller no tiene número de WhatsApp conectado"))
	}
	var p struct {
		Buttons []domain.Button `json:"botones"`
		Image   string          `json:"imagen"`
	}
	_ = json.Unmarshal(m.Payload, &p)
	var wamid string
	var err error
	switch {
	case p.Image != "" && s.media != nil:
		var img Image
		if img, err = s.media.Media(ctx, tallerID, p.Image); err != nil {
			return s.failed(ctx, m, errors.New("no se encontró la foto para enviar"))
		}
		img.Caption = m.Body
		wamid, err = s.channel.SendImage(ctx, ch.PhoneNumberID, conv.Phone, img)
	case len(p.Buttons) > 0:
		wamid, err = s.channel.SendButtons(ctx, ch.PhoneNumberID, conv.Phone, m.Body, p.Buttons)
	default:
		wamid, err = s.channel.SendText(ctx, ch.PhoneNumberID, conv.Phone, m.Body)
	}
	if err != nil {
		if IsRetryable(err) && attempt+1 < MaxDeliveryAttempts {
			return err // sigue pendiente; el outbox reintenta con retroceso
		}
		return s.failed(ctx, m, err)
	}
	return s.tx.Do(ctx, func(ctx context.Context) error {
		return s.repo.UpdateDelivery(ctx, tallerID, msgID, "enviado", &wamid, nil, s.clock.Now())
	})
}

// failed marca el mensaje fallido y avisa, en una sola transacción. El texto del error es el del adaptador, que
// no trae datos personales (ADR-0025).
func (s *Service) failed(ctx context.Context, m Message, cause error) error {
	e := cause.Error()
	now := s.clock.Now()
	return s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.repo.UpdateDelivery(ctx, m.TallerID, m.ID, "fallido", nil, &e, now); err != nil {
			return err
		}
		payload := map[string]any{"mensaje_id": m.ID.String(), "error": e}
		if m.Automation != nil {
			payload["automatizacion"] = *m.Automation
		}
		if m.OrderID != nil {
			payload["orden_id"] = m.OrderID.String()
		}
		return s.events.Publish(ctx, m.TallerID, shared.Event{Type: "messaging.envio_fallido", AggregateID: m.ID, OccurredAt: now, Payload: payload})
	})
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
// SentSince cuenta los mensajes que salieron desde since, para el medidor del administrador (HU-74). Corre en la
// transacción del llamador si la hay.
func (s *Service) SentSince(ctx context.Context, tallerID shared.ID, since time.Time) (int, error) {
	var n int
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		var err error
		n, err = s.repo.SentSince(ctx, tallerID, since)
		return err
	})
	return n, err
}

// ResponseTime es cuánto tardó la primera respuesta a un mensaje del cliente; Auto = respuesta automática de AU-05.
type ResponseTime struct {
	Minutes float64
	Auto    bool
}

func (s *Service) ResponseMinutes(ctx context.Context, tallerID shared.ID, since time.Time) ([]ResponseTime, error) {
	var out []ResponseTime
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.repo.ResponseMinutes(ctx, tallerID, since)
		return err
	})
	return out, err
}

// MoveCustomer pasa las conversaciones del cliente repetido al que se conserva (HU-15). Cada conversación sigue con su
// celular; solo cambia de quién es.
func (s *Service) MoveCustomer(ctx context.Context, tallerID, from, to shared.ID) (int, error) {
	var n int
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		var err error
		n, err = s.repo.MoveCustomer(ctx, tallerID, from, to)
		return err
	})
	return n, err
}

// AnonymizeCustomer borra lo que identifica al titular en sus conversaciones (HU-87): el celular queda como
// "anonimo-<id>" y los mensajes sin texto. Lo que estaba por enviarse no sale.
func (s *Service) AnonymizeCustomer(ctx context.Context, tallerID, customerID shared.ID) (int, error) {
	var n int
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		var err error
		n, err = s.repo.AnonymizeCustomer(ctx, tallerID, customerID)
		return err
	})
	return n, err
}

// MessagesOf son los mensajes del cliente para la copia de sus datos (HU-87).
func (s *Service) MessagesOf(ctx context.Context, tallerID, customerID shared.ID) ([]Message, error) {
	var out []Message
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.repo.MessagesOf(ctx, tallerID, customerID)
		return err
	})
	return out, err
}

// WebhookTrace es un webhook crudo en el registro técnico: cuándo llegó, si se procesó y el error (HU-100).
type WebhookTrace struct {
	ID          shared.ID
	ReceivedAt  time.Time
	ProcessedAt *time.Time
	Error       *string
}

// MessagesByPhone son los últimos mensajes con ese celular en el taller, con su estado y error (HU-100).
func (s *Service) MessagesByPhone(ctx context.Context, tallerID shared.ID, phone string) ([]Message, error) {
	var out []Message
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.repo.MessagesByPhone(ctx, tallerID, phone, 50)
		return err
	})
	return out, err
}

// WebhooksMentioning busca en los webhooks crudos de los últimos días los que mencionan el celular (HU-100). No
// tienen taller: solo lo usa el registro técnico de plataforma.
func (s *Service) WebhooksMentioning(ctx context.Context, digits string, since time.Time) ([]WebhookTrace, error) {
	var out []WebhookTrace
	err := s.tx.DoWithSettings(ctx, map[string]string{"app.admin_plataforma": "on"}, func(ctx context.Context) error {
		var err error
		out, err = s.repo.WebhooksMentioning(ctx, digits, since, 50)
		return err
	})
	return out, err
}
