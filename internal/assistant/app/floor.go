package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"sweveh/internal/assistant/domain"
	"sweveh/internal/audit"
	"sweveh/internal/shared"
)

// Asistente del piso (R8, ADR-0041, HU-107). Es un agente interno del taller, distinto del asesor de WhatsApp: el
// mecánico le escribe, le habla o le manda fotos y el asistente arma una propuesta con tools que validan en código.
// Nada llega a la orden hasta que una persona toca Confirmar (R-08). Usa el mismo puerto de IA (Advisor), el mismo
// tope mensual y el mismo registro de llamadas que el asesor (ADR-0026, ADR-0029).

// FloorWorkshop es lo que el asistente del piso consulta y hace en los otros módulos, por sus fachadas (wire).
type FloorWorkshop interface {
	Context(ctx context.Context, tallerID shared.ID) (FloorContext, error)
	Vehicle(ctx context.Context, tallerID shared.ID, plate string) (*FloorVehicle, error) // nil = no está en el taller
	CustomerByPhone(ctx context.Context, tallerID shared.ID, phone string) (*FloorPerson, error)
	Order(ctx context.Context, tallerID, orderID shared.ID) (FloorOrder, error)
	// OrderStatus lee de la base cómo está de verdad la orden: estado, qué falta para recibir y su cotización.
	OrderStatus(ctx context.Context, tallerID, orderID shared.ID) (FloorOrderStatus, error)
	// Saved dice qué tiene ya guardado la orden de su recepción (fotos por lado, síntoma, kilometraje, combustible).
	Saved(ctx context.Context, tallerID, orderID shared.ID) (domain.Saved, error)
	// Apply aplica la propuesta dentro de la transacción del llamador, por los casos de uso de cada módulo. photo lee
	// una foto del chat para pasarla a la orden como evidencia (HU-21): antes del cambio de estado, que la puede exigir.
	Apply(ctx context.Context, tallerID shared.ID, actor shared.Actor, p domain.FloorProposal,
		photo func(ctx context.Context, ref string) ([]byte, error)) (FloorApplied, error)
}

// FloorFiles guarda las fotos que llegan al chat hasta que se confirman.
type FloorFiles interface {
	Put(ctx context.Context, key string, data []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
}

// FloorRepository guarda las conversaciones, sus mensajes y las confirmaciones.
type FloorRepository interface {
	InsertChat(ctx context.Context, c FloorChat) error
	Chat(ctx context.Context, tallerID, id shared.ID) (FloorChat, error)
	ChatForOrder(ctx context.Context, tallerID, orderID shared.ID) (*FloorChat, error)
	LatestForUser(ctx context.Context, tallerID, userID shared.ID, since time.Time) (*FloorChat, error)
	UpdateChat(ctx context.Context, c FloorChat, prevVersion int) (bool, error)
	InsertMessage(ctx context.Context, tallerID shared.ID, m FloorMessage) error
	// MessageChat dice en qué conversación está el mensaje (found=false si no existe).
	MessageChat(ctx context.Context, tallerID, msgID shared.ID) (chatID shared.ID, found bool, err error)
	Messages(ctx context.Context, tallerID, chatID shared.ID, limit int) ([]FloorMessage, error)
	// Confirmation devuelve el resultado guardado con esa clave y la conversación en que se usó.
	Confirmation(ctx context.Context, tallerID shared.ID, key string) (json.RawMessage, shared.ID, bool, error)
	InsertConfirmation(ctx context.Context, tallerID, chatID shared.ID, key string, by shared.ID, result json.RawMessage, at time.Time) (bool, error)
	LogCall(ctx context.Context, tallerID shared.ID, c AICall) error
	MonthCost(ctx context.Context, tallerID shared.ID, since time.Time) (int64, error)
	AnonymizeOrders(ctx context.Context, tallerID shared.ID, orders []shared.ID) (int, error)
}

// FloorContext es lo que el asistente sabe del taller.
type FloorContext struct {
	Workshop  string
	Attends   string // carro | moto | ambos
	Services  []domain.ListService
	ItemsCar  []FloorChecklistItem
	ItemsMoto []FloorChecklistItem
}

type FloorChecklistItem struct{ Key, Name string }

type FloorVehicle struct {
	ID, Plate, Kind, Description string
	CustomerName                 string
	Open                         *FloorOrder
}

type FloorPerson struct{ ID, Name string }

// FloorOrderStatus es lo que está guardado de verdad en la base (no la propuesta).
type FloorOrderStatus struct {
	Code       string   `json:"codigo"`
	State      string   `json:"estado"`
	Missing    []string `json:"falta_para_recibido,omitempty"`
	QuoteState string   `json:"cotizacion_estado,omitempty"`
	QuoteItems int      `json:"cotizacion_items"`
	QuoteTotal int64    `json:"cotizacion_total"`
}

type FloorOrder struct {
	ID, Code, State, Plate, Kind, Vehicle string
}

// FloorApplied es lo que quedó guardado al confirmar.
type FloorApplied struct {
	OrderID    shared.ID `json:"ordenId"`
	OrderCode  string    `json:"ordenCodigo"`
	OrderState string    `json:"ordenEstado"`
	Created    bool      `json:"ordenNueva"`
	Done       []string  `json:"hecho"`
	Photos     int       `json:"fotos"`
	// Message es el "Listo, quedó guardado…" que escribió esta confirmación (no se guarda con la clave).
	Message *FloorMessage `json:"-"`
}

// FloorChat es una conversación del piso.
type FloorChat struct {
	ID, TallerID, UserID shared.ID
	OrderID              *shared.ID
	Proposal             domain.FloorProposal
	Version              int
	CreatedAt, UpdatedAt time.Time
}

// FloorAttachment es una foto, una nota de voz o un video del mensaje. La voz y el video no se guardan: de la voz
// queda la transcripción y del video, la transcripción y sus cuadros (que van como fotos).
type FloorAttachment struct {
	Kind    string `json:"tipo"` // foto | voz | video
	Ref     string `json:"ref,omitempty"`
	Seconds int    `json:"segundos,omitempty"` // duración del video
}

// FloorMessage es un mensaje de la conversación.
type FloorMessage struct {
	ID, ChatID  shared.ID
	Role        string // usuario | asistente
	Text        string
	Attachments []FloorAttachment
	Choices     []string // botones (dos placas: "¿Seguimos con ABC123 o con XYZ98D?")
	Proposal    *domain.FloorProposal
	CreatedAt   time.Time
}

// Blob es un archivo que llegó con el mensaje.
type Blob struct {
	Data []byte
	Mime string
}

// FloorInput es un mensaje de la persona. ChatID u OrderID dicen dónde sigue; sin ninguno empieza un chat general.
type FloorInput struct {
	TallerID shared.ID
	Actor    shared.Actor
	Roles    []string // para confirmar desde el chat con los permisos de quien escribe
	ChatID   *shared.ID
	OrderID  *shared.ID
	Text     string
	Audio    *Blob
	Photos   []Blob
	// Video: el mensaje es una recepción por video (lo pone solo el caso de uso Video; no llega por HTTP).
	Video bool
}

// FloorReply es lo que ve la pantalla después de un mensaje.
type FloorReply struct {
	Chat      FloorChat
	Messages  []FloorMessage
	Available bool // false = la IA no respondió; sigue con el formulario (RNF-13)
}

// Límites del mensaje.
const (
	FloorMaxText     = 2000
	FloorMaxPhotos   = 4
	FloorMaxPhoto    = 5 << 20 // 4 fotos y una nota de voz caben en los 32 MB del envío (Cloud Run)
	FloorMaxAudio    = 10 << 20
	floorHistory     = 12
	floorTimeout     = 60 * time.Second // los modelos gratis tardan 10 a 20 s por ronda (medido con make agent-eval-floor)
	floorConfirmText = "Listo, quedó guardado:"
)

// Textos fijos del asistente (no son redacción de la IA).
const (
	FloorUnavailable  = "El asistente no está disponible, sigue con el formulario."
	floorNoVoice      = "No pude escuchar la nota de voz. Escríbeme lo que necesitas."
	floorNoVideoAudio = "No pude escuchar el audio del video: solo revisé los cuadros."
	floorChecked      = "Te dejé la propuesta en la ficha. Revisa los valores antes de confirmar."
	floorEmpty        = "Escríbeme, mándame una nota de voz o una foto."
)

var (
	ErrFloorText     = shared.Validation("mensaje_largo", "El mensaje es muy largo: máximo 2.000 letras.").WithField("texto")
	ErrFloorEmptyMsg = shared.Validation("mensaje_vacio", floorEmpty).WithField("texto")
	ErrFloorPhotos   = shared.Validation("demasiadas_fotos", "Manda hasta 4 fotos por mensaje.").WithField("fotos")
	ErrFloorPhoto    = shared.Validation("foto_invalida", "La foto debe ser JPG, PNG o WEBP de máximo 5 MB.").WithField("fotos")
	ErrFloorAudio    = shared.Validation("audio_invalido", "La nota de voz es muy larga.").WithField("audio")
	ErrFloorChat     = shared.NotFound("chat_no_encontrado", "No encontramos esa conversación.")
	ErrFloorVersion  = shared.Conflict("version_conflicto", "Alguien más cambió esta propuesta. Recarga para ver lo último.")
	ErrFloorPerm     = shared.Forbidden("sin_permiso", "No tienes permiso para confirmar esta parte de la propuesta.")
	ErrFloorKeyUsed  = shared.Conflict("clave_usada", "Esa Idempotency-Key ya se usó para confirmar otra conversación.")
	ErrFloorKey      = shared.Validation("clave_requerida", "Falta el encabezado Idempotency-Key.").WithField("Idempotency-Key")
)

// Floor es el caso de uso del asistente del piso.
type Floor struct {
	uow    UnitOfWork
	repo   FloorRepository
	shop   FloorWorkshop
	files  FloorFiles
	audit  AuditLog
	events EventPublisher
	clock  Clock
	model  Advisor     // nil = sin IA: el chat dice que siga con el formulario
	stt    Transcriber // nil = sin voz
	capUSD int64
	can    func(roles []string, perm string) bool
}

// EventPublisher publica en el outbox, en la transacción del llamador.
type EventPublisher interface {
	Publish(ctx context.Context, tallerID shared.ID, e shared.Event) error
}

// FloorDeps son las piezas del asistente del piso (las conecta el composition root).
type FloorDeps struct {
	UoW    UnitOfWork
	Repo   FloorRepository
	Shop   FloorWorkshop
	Files  FloorFiles
	Audit  AuditLog
	Events EventPublisher
	Clock  Clock
	// MonthlyCapMicroUSD es el tope de gasto en IA por taller y mes (el mismo del asesor).
	MonthlyCapMicroUSD int64
	// Can dice si los roles tienen el permiso (identity.HasPermission).
	Can func(roles []string, perm string) bool
}

func NewFloor(d FloorDeps) *Floor {
	return &Floor{uow: d.UoW, repo: d.Repo, shop: d.Shop, files: d.Files, audit: d.Audit, events: d.Events, clock: d.Clock,
		capUSD: d.MonthlyCapMicroUSD, can: d.Can}
}

// SetModel conecta el modelo con tools (composition root).
func (f *Floor) SetModel(m Advisor) { f.model = m }

// SetTranscriber conecta la voz (Groq, ADR-0033).
func (f *Floor) SetTranscriber(t Transcriber) { f.stt = t }

// HasModel dice si hay IA conectada.
func (f *Floor) HasModel() bool { return f.model != nil }

// HasVoice dice si se puede hablarle (la pantalla oculta el micrófono si no).
func (f *Floor) HasVoice() bool { return f.stt != nil }

// Send procesa un mensaje: transcribe la voz, guarda las fotos, corre el asistente con sus tools y guarda la
// conversación con la propuesta nueva. La IA y los archivos van fuera de la transacción.
func (f *Floor) Send(ctx context.Context, in FloorInput) (FloorReply, error) {
	text, err := validateInput(&in)
	if err != nil {
		return FloorReply{}, err
	}
	chat, isNew, err := f.chatFor(ctx, in)
	if err != nil {
		return FloorReply{}, err
	}
	now := f.clock.Now()
	user, voiceFailed, err := f.userMessage(ctx, in, chat.ID, text, now)
	if err != nil {
		return FloorReply{}, err
	}
	if !isNew && chat.Proposal.Pending() && domain.IsConfirmWord(user.Text) {
		return f.confirmFromChat(ctx, in, chat, user)
	}
	history, err := f.history(ctx, in.TallerID, chat.ID, isNew)
	if err != nil {
		return FloorReply{}, err
	}
	reply := FloorMessage{ID: shared.NewID(), ChatID: chat.ID, Role: "asistente", CreatedAt: now.Add(time.Millisecond)}
	next, available, err := f.answer(ctx, chat, history, user, in, voiceFailed, &reply)
	if err != nil {
		return FloorReply{}, err
	}
	snap := next.Proposal
	reply.Proposal = &snap
	prev := chat.Version
	next.Version, next.UpdatedAt = chat.Version+1, now
	if err := f.save(ctx, in.TallerID, chat, isNew, next, prev, user, reply); err != nil {
		return FloorReply{}, err
	}
	return FloorReply{Chat: next, Messages: []FloorMessage{user, reply}, Available: available}, nil
}

// confirmFromChat: la persona escribió o dijo "confírmalo" o "guárdalo". Lo detecta el código (no es una tool: la IA
// no puede confirmar por su cuenta) y confirma igual que el botón, con la clave del mensaje como Idempotency-Key. Si
// algo falta, responde qué falta sin gastar IA (decisión de Pablo, 26-nov).
func (f *Floor) confirmFromChat(ctx context.Context, in FloorInput, chat FloorChat, user FloorMessage) (FloorReply, error) {
	applied, done, err := f.Confirm(ctx, ConfirmCmd{TallerID: in.TallerID, ChatID: chat.ID, Actor: in.Actor, Roles: in.Roles,
		Key: "chat-" + user.ID.String(), Version: chat.Version})
	if err != nil {
		e, ok := shared.AsError(err)
		if !ok {
			return FloorReply{}, err
		}
		reply := FloorMessage{ID: shared.NewID(), ChatID: chat.ID, Role: "asistente", CreatedAt: user.CreatedAt.Add(time.Millisecond),
			Text: "No lo pude guardar: " + e.Message}
		next := chat
		next.Version, next.UpdatedAt = chat.Version+1, f.clock.Now()
		if err := f.save(ctx, in.TallerID, chat, false, next, chat.Version, user, reply); err != nil {
			return FloorReply{}, err
		}
		return FloorReply{Chat: next, Messages: []FloorMessage{user, reply}, Available: true}, nil
	}
	// El mensaje de la persona va antes del "Listo, quedó guardado" que escribió Confirm.
	user.CreatedAt = user.CreatedAt.Add(-time.Millisecond)
	if err := f.uow.Do(ctx, func(ctx context.Context) error { return f.repo.InsertMessage(ctx, in.TallerID, user) }); err != nil {
		return FloorReply{}, err
	}
	msgs := []FloorMessage{user}
	if applied.Message != nil {
		msgs = append(msgs, *applied.Message)
	}
	return FloorReply{Chat: done, Messages: msgs, Available: f.model != nil}, nil
}

// validateInput revisa el tamaño del mensaje y el tipo de cada foto por su contenido. Devuelve el texto limpio.
func validateInput(in *FloorInput) (string, error) {
	text := strings.TrimSpace(in.Text)
	switch {
	case utf8.RuneCountInString(text) > FloorMaxText:
		return "", ErrFloorText
	case len(in.Photos) > FloorMaxPhotos:
		return "", ErrFloorPhotos
	case in.Audio != nil && len(in.Audio.Data) > FloorMaxAudio:
		return "", ErrFloorAudio
	case text == "" && in.Audio == nil && len(in.Photos) == 0:
		return "", ErrFloorEmptyMsg
	}
	for i := range in.Photos {
		mime := domain.SniffImage(in.Photos[i].Data)
		if mime == "" || len(in.Photos[i].Data) > FloorMaxPhoto {
			return "", ErrFloorPhoto
		}
		in.Photos[i].Mime = mime
	}
	return text, nil
}

// userMessage arma el mensaje de la persona: el texto con la transcripción de la voz (la voz no se guarda) y las fotos
// ya guardadas. voiceFailed: había nota de voz y no se pudo transcribir.
func (f *Floor) userMessage(ctx context.Context, in FloorInput, chatID shared.ID, text string, now time.Time) (FloorMessage, bool, error) {
	user := FloorMessage{ID: shared.NewID(), ChatID: chatID, Role: "usuario", CreatedAt: now}
	voiceFailed := false
	if in.Audio != nil {
		user.Attachments = append(user.Attachments, FloorAttachment{Kind: "voz"})
		if said, err := f.transcribe(ctx, *in.Audio); err != nil {
			voiceFailed = true
		} else {
			text = strings.TrimSpace(text + " " + said)
		}
	}
	user.Text = text
	for _, p := range in.Photos {
		ref := in.TallerID.String() + "/" + shared.NewID().String() // la clave del almacenamiento es taller/uuid
		if err := f.files.Put(ctx, ref, p.Data); err != nil {
			return user, voiceFailed, err
		}
		user.Attachments = append(user.Attachments, FloorAttachment{Kind: "foto", Ref: ref})
	}
	return user, voiceFailed, nil
}

// answer llena la respuesta del asistente y devuelve la conversación con la propuesta nueva.
func (f *Floor) answer(ctx context.Context, chat FloorChat, history []FloorMessage, user FloorMessage, in FloorInput, voiceFailed bool,
	reply *FloorMessage) (FloorChat, bool, error) {
	plates := domain.FindPlates(user.Text)
	switch {
	case voiceFailed && user.Text == "" && len(in.Photos) == 0:
		reply.Text = floorNoVoice
		return chat, true, nil
	case chat.Proposal.Plate == nil && chat.OrderID == nil && len(plates) >= 2:
		// Dos placas: el código pregunta con cuál sigue, con dos botones (sin gastar IA).
		reply.Text = "¿Seguimos con " + plates[0] + " o con " + plates[1] + "?"
		reply.Choices = plates[:2]
		return chat, true, nil
	}
	res, ok, err := f.think(ctx, chat, history, user, in)
	if err != nil || !ok {
		reply.Text = FloorUnavailable
		return chat, false, err
	}
	reply.Text = res.text
	switch {
	case voiceFailed && in.Video:
		reply.Text = floorNoVideoAudio + " " + reply.Text
	case voiceFailed:
		reply.Text = floorNoVoice + " " + reply.Text
	}
	return res.chat, true, nil
}

// save guarda en una transacción la conversación (si es nueva), los dos mensajes y la propuesta, con versión optimista.
func (f *Floor) save(ctx context.Context, tallerID shared.ID, chat FloorChat, isNew bool, next FloorChat, prev int, msgs ...FloorMessage) error {
	return f.uow.Do(ctx, func(ctx context.Context) error {
		if isNew {
			if err := f.repo.InsertChat(ctx, chat); err != nil {
				return err
			}
		}
		for _, m := range msgs {
			if err := f.repo.InsertMessage(ctx, tallerID, m); err != nil {
				return err
			}
		}
		ok, err := f.repo.UpdateChat(ctx, next, prev)
		if err == nil && !ok {
			err = ErrFloorVersion
		}
		return err
	})
}

// chatFor encuentra la conversación: la pedida, la de la orden o una nueva.
func (f *Floor) chatFor(ctx context.Context, in FloorInput) (FloorChat, bool, error) {
	var out FloorChat
	isNew := false
	err := f.uow.Do(ctx, func(ctx context.Context) error {
		switch {
		case in.ChatID != nil:
			c, err := f.repo.Chat(ctx, in.TallerID, *in.ChatID)
			if err != nil {
				return err
			}
			out = c
			return nil
		case in.OrderID != nil:
			o, err := f.shop.Order(ctx, in.TallerID, *in.OrderID) // anti-IDOR: la orden es de este taller (RLS)
			if err != nil {
				return err
			}
			c, err := f.repo.ChatForOrder(ctx, in.TallerID, *in.OrderID)
			if err != nil {
				return err
			}
			if c != nil {
				out = *c
				return nil
			}
			out, isNew = f.newChat(in, &o), true
			return nil
		}
		out, isNew = f.newChat(in, nil), true
		return nil
	})
	return out, isNew, err
}

func (f *Floor) newChat(in FloorInput, o *FloorOrder) FloorChat {
	now := f.clock.Now()
	c := FloorChat{ID: shared.NewID(), TallerID: in.TallerID, UserID: in.Actor.ID, Version: 1, CreatedAt: now, UpdatedAt: now}
	if o != nil {
		id, _ := shared.ParseID(o.ID)
		c.OrderID = &id
		c.Proposal = withOrder(c.Proposal, *o)
	}
	return c
}

func withOrder(p domain.FloorProposal, o FloorOrder) domain.FloorProposal {
	p.Plate = &domain.Field{Value: o.Plate, Origin: domain.FromWorkshop}
	p.Kind, p.Vehicle, p.OrderID, p.OrderCode, p.OrderState = o.Kind, o.Vehicle, o.ID, o.Code, o.State
	p.NewOrder, p.VehicleNew, p.Customer = false, false, nil
	return p
}

func (f *Floor) history(ctx context.Context, tallerID, chatID shared.ID, isNew bool) ([]FloorMessage, error) {
	if isNew {
		return nil, nil
	}
	var out []FloorMessage
	err := f.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = f.repo.Messages(ctx, tallerID, chatID, floorHistory)
		return err
	})
	return out, err
}

func (f *Floor) transcribe(ctx context.Context, a Blob) (string, error) {
	if f.stt == nil {
		return "", errors.New("sin transcripción")
	}
	cctx, cancel := context.WithTimeout(ctx, transcribeTimeout)
	defer cancel()
	t, err := f.stt.Transcribe(cctx, a.Data, a.Mime)
	if err != nil {
		return "", err
	}
	if t = strings.TrimSpace(t); t == "" {
		return "", errors.New("transcripción vacía")
	}
	if r := []rune(t); len(r) > FloorMaxText {
		t = string(r[:FloorMaxText])
	}
	return t, nil
}

type thought struct {
	text string
	chat FloorChat
}

// think corre el modelo con las tools sobre una copia de la conversación. ok=false: no hay IA, se agotó el tope o
// falló (la propuesta queda como estaba: nada a medias).
func (f *Floor) think(ctx context.Context, chat FloorChat, history []FloorMessage, user FloorMessage, in FloorInput) (thought, bool, error) {
	fctx, ok, err := f.ready(ctx, in.TallerID)
	if err != nil || !ok {
		return thought{}, false, err
	}
	msgs, saidTexts := chatMessages(history, user, in.Photos, in.Video)
	work := chat
	work.Proposal = cloneProposal(chat.Proposal)
	t := &floorTurn{f: f, tallerID: in.TallerID, chat: &work, fctx: fctx, said: strings.Join(saidTexts, "\n"),
		amounts: domain.SaidAmounts(saidTexts...), video: in.Video}
	for _, a := range user.Attachments {
		if a.Kind == "foto" {
			t.photos = append(t.photos, a.Ref)
		}
	}
	cctx, cancel := context.WithTimeout(ctx, floorTimeout)
	defer cancel()
	// Una sola placa en el mensaje y la conversación sin vehículo: el código la busca antes de llamar al modelo. Así la
	// conversación pasa a la orden aunque el modelo olvide la tool, y se ahorra una ronda.
	// Lo que viene de la persona o de una foto (la propuesta, el cliente de la placa) nunca va en el prompt del sistema
	// (R-08): entra como resultado de una tool, igual que cualquier dato que consulta el modelo.
	if plates := domain.FindPlates(user.Text); len(plates) == 1 && work.Proposal.Plate == nil && work.OrderID == nil {
		args := jsonString(map[string]string{"placa": plates[0]})
		msgs = append(msgs, toolTurn("pre-placa", "buscar_vehiculo", args, t.findVehicle(cctx, plates[0], false))...)
	}
	msgs = append(msgs, toolTurn("pre-ficha", "ver_ficha", "{}", t.sheetView())...)
	if work.OrderID != nil { // lo guardado de verdad, leído de la base, para que no diga "creada" sin estarlo
		msgs = append(msgs, toolTurn("pre-estado", "estado_orden", "{}", t.orderStatus(cctx))...)
	}
	chatFn := func(ctx context.Context, req ChatRequest) (ChatMessage, error) {
		return f.chat(ctx, in.TallerID, user.ID, req)
	}
	system := floorSystem(fctx, work.Proposal, f.clock.Now())
	if in.Video {
		system += floorVideoSystem
	}
	text, _, err := Converse(cctx, chatFn, ChatRequest{System: system, Messages: msgs, Tools: FloorTools}, t.run)
	if err != nil || strings.TrimSpace(text) == "" {
		return thought{}, false, ctx.Err() // la IA falló o se agotó el tiempo: solo se corta si el llamador se fue
	}
	return thought{text: floorText(text, t.allowed()), chat: work}, true, nil
}

// ready dice si se puede llamar a la IA (hay modelo y queda tope este mes) y trae lo que el asistente sabe del taller.
func (f *Floor) ready(ctx context.Context, tallerID shared.ID) (FloorContext, bool, error) {
	if f.model == nil {
		return FloorContext{}, false, nil
	}
	var (
		fctx  FloorContext
		spent int64
	)
	err := f.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		if spent, err = f.repo.MonthCost(ctx, tallerID, monthStart(f.clock.Now())); err != nil || spent >= f.capUSD {
			return err
		}
		fctx, err = f.shop.Context(ctx, tallerID)
		return err
	})
	return fctx, err == nil && spent < f.capUSD, err
}

// chatMessages arma el historial para el modelo (el mensaje actual con sus fotos) y lo que dijo la persona.
func chatMessages(history []FloorMessage, user FloorMessage, photos []Blob, video bool) ([]ChatMessage, []string) {
	said := []string{user.Text}
	msgs := make([]ChatMessage, 0, len(history)+1)
	for _, h := range history {
		role := "assistant"
		if h.Role == "usuario" {
			role = "user"
			said = append(said, h.Text)
		}
		msgs = append(msgs, ChatMessage{Role: role, Content: h.Text})
	}
	um := ChatMessage{Role: "user", Content: user.Text}
	switch {
	case um.Content == "" && video:
		um.Content = "(video de recepción sin audio)"
	case um.Content == "":
		um.Content = "(mandó fotos sin texto)"
	}
	for _, p := range photos {
		um.Images = append(um.Images, Image{Mime: p.Mime, Data: p.Data})
	}
	return append(msgs, um), said
}

// floorText deja el texto del asistente corto y sin markdown; si trae una cifra que no es de la lista ni la dijo la
// persona, sale el texto fijo.
func floorText(text string, allowed map[int64]bool) string {
	text = domain.Tidy(text)
	if r := []rune(text); len(r) > 1200 {
		text = string(r[:1200])
	}
	if !domain.FloorTextOK(text, allowed) {
		return floorChecked
	}
	return domain.FloorUnsaved(text)
}

// chat llama al modelo y registra la llamada (HU-99) con propósito "piso".
func (f *Floor) chat(ctx context.Context, tallerID, msgID shared.ID, req ChatRequest) (ChatMessage, error) {
	start := time.Now()
	r, err := f.model.Chat(ctx, req)
	result := "ok"
	if err != nil {
		result = "error"
	}
	if r.Provider == "" {
		r.Provider = "desconocido"
	}
	if r.Model == "" {
		r.Model = "desconocido"
	}
	id := msgID
	call := AICall{MessageID: &id, Provider: r.Provider, Model: r.Model, Purpose: "piso", Result: result, TokensIn: r.TokensIn,
		TokensOut: r.TokensOut, LatencyMs: int(time.Since(start).Milliseconds()), CostMicroUSD: r.CostMicroUSD}
	if lerr := f.uow.Do(context.WithoutCancel(ctx), func(ctx context.Context) error { return f.repo.LogCall(ctx, tallerID, call) }); lerr != nil {
		return ChatMessage{}, lerr
	}
	return r.Message, err
}

// ChatView es una conversación con sus mensajes y lo que falta para recibir.
type ChatView struct {
	Chat     FloorChat
	Messages []FloorMessage
	Progress domain.Progress
}

// Get devuelve la conversación.
func (f *Floor) Get(ctx context.Context, tallerID, chatID shared.ID) (ChatView, error) {
	var out ChatView
	err := f.uow.Do(ctx, func(ctx context.Context) error {
		c, err := f.repo.Chat(ctx, tallerID, chatID)
		if err != nil {
			return err
		}
		out, err = f.view(ctx, c)
		return err
	})
	return out, err
}

// Latest es la última conversación de la persona desde hace within (nil si no hay).
func (f *Floor) Latest(ctx context.Context, tallerID, userID shared.ID, within time.Duration) (*ChatView, error) {
	var out *ChatView
	err := f.uow.Do(ctx, func(ctx context.Context) error {
		c, err := f.repo.LatestForUser(ctx, tallerID, userID, f.clock.Now().Add(-within))
		if err != nil || c == nil {
			return err
		}
		v, err := f.view(ctx, *c)
		out = &v
		return err
	})
	return out, err
}

// ForOrder devuelve la conversación de la orden (nil si todavía no hay).
func (f *Floor) ForOrder(ctx context.Context, tallerID, orderID shared.ID) (*ChatView, error) {
	var out *ChatView
	err := f.uow.Do(ctx, func(ctx context.Context) error {
		if _, err := f.shop.Order(ctx, tallerID, orderID); err != nil {
			return err
		}
		c, err := f.repo.ChatForOrder(ctx, tallerID, orderID)
		if err != nil || c == nil {
			return err
		}
		v, err := f.view(ctx, *c)
		out = &v
		return err
	})
	return out, err
}

func (f *Floor) view(ctx context.Context, c FloorChat) (ChatView, error) {
	msgs, err := f.repo.Messages(ctx, c.TallerID, c.ID, 200)
	if err != nil {
		return ChatView{}, err
	}
	pr, err := f.progress(ctx, c)
	return ChatView{Chat: c, Messages: msgs, Progress: pr}, err
}

// Progress dice qué falta para recibir.
func (f *Floor) Progress(ctx context.Context, c FloorChat) (domain.Progress, error) {
	var out domain.Progress
	err := f.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = f.progress(ctx, c)
		return err
	})
	return out, err
}

func (f *Floor) progress(ctx context.Context, c FloorChat) (domain.Progress, error) {
	var saved domain.Saved
	if c.OrderID != nil && c.Proposal.Receiving() {
		var err error
		if saved, err = f.shop.Saved(ctx, c.TallerID, *c.OrderID); err != nil {
			return domain.Progress{}, err
		}
	}
	return c.Proposal.Progress(saved), nil
}

// Edit guarda lo que la persona cambió o quitó en la ficha en vivo (con versión optimista, R-07).
func (f *Floor) Edit(ctx context.Context, tallerID, chatID shared.ID, version int, next domain.FloorProposal) (FloorChat, error) {
	var out FloorChat
	err := f.uow.Do(ctx, func(ctx context.Context) error {
		c, err := f.repo.Chat(ctx, tallerID, chatID)
		if err != nil {
			return err
		}
		if c.Version != version {
			return ErrFloorVersion
		}
		c.Proposal = domain.HumanEdit(c.Proposal, next)
		c.Version, c.UpdatedAt = version+1, f.clock.Now()
		ok, err := f.repo.UpdateChat(ctx, c, version)
		if err != nil {
			return err
		}
		if !ok {
			return ErrFloorVersion
		}
		out = c
		return nil
	})
	return out, err
}

// ConfirmCmd es tocar Confirmar.
type ConfirmCmd struct {
	TallerID, ChatID shared.ID
	Actor            shared.Actor
	Roles            []string
	Key              string
	Version          int
}

// Confirm aplica la propuesta en una sola transacción, por los casos de uso que ya existen (orden, ficha,
// recepción, fotos, hallazgos, cotización, gastos y estado), con bitácora "origen: asistente" y quien confirmó.
// Repetir con la misma Idempotency-Key devuelve el mismo resultado sin aplicar dos veces.
func (f *Floor) Confirm(ctx context.Context, c ConfirmCmd) (FloorApplied, FloorChat, error) {
	key := strings.TrimSpace(c.Key)
	if len(key) < 8 || len(key) > 100 {
		return FloorApplied{}, FloorChat{}, ErrFloorKey
	}
	var (
		applied FloorApplied
		chat    FloorChat
	)
	err := f.uow.Do(ctx, func(ctx context.Context) error {
		var found bool
		var err error
		if applied, chat, found, err = f.replayed(ctx, c.TallerID, c.ChatID, key); err != nil || found {
			return err
		}
		applied, chat, err = f.apply(ctx, c, key)
		return err
	})
	if errors.Is(err, ErrFloorVersion) {
		// Dos toques a la vez con la misma clave: el segundo espera al primero y ve la versión cambiada. Si la clave
		// quedó guardada, devuelve ese resultado en vez del conflicto.
		var (
			a     FloorApplied
			ch    FloorChat
			found bool
		)
		if rerr := f.uow.Do(ctx, func(ctx context.Context) error {
			var err error
			a, ch, found, err = f.replayed(ctx, c.TallerID, c.ChatID, key)
			return err
		}); rerr == nil && found {
			return a, ch, nil
		}
	}
	return applied, chat, err
}

// apply es el cuerpo del Confirmar, dentro de su transacción: valida, revisa permisos, aplica por los casos de uso,
// deja la conversación lista para seguir, guarda la clave, el mensaje y la bitácora.
func (f *Floor) apply(ctx context.Context, c ConfirmCmd, key string) (FloorApplied, FloorChat, error) {
	cur, err := f.repo.Chat(ctx, c.TallerID, c.ChatID)
	if err != nil {
		return FloorApplied{}, cur, err
	}
	if cur.Version != c.Version {
		return FloorApplied{}, cur, ErrFloorVersion
	}
	p := cur.Proposal
	if err := p.Validate(); err != nil {
		return FloorApplied{}, cur, err
	}
	if err := f.allowed(p, c.Roles); err != nil {
		return FloorApplied{}, cur, err
	}
	applied, err := f.shop.Apply(ctx, c.TallerID, c.Actor, p, f.files.Get)
	if err != nil {
		return applied, cur, err
	}
	now := f.clock.Now()
	oid := applied.OrderID
	cur.OrderID = &oid
	cur.Proposal = p.Reset(oid.String(), applied.OrderCode, applied.OrderState)
	prev := cur.Version
	cur.Version, cur.UpdatedAt = prev+1, now
	raw, err := json.Marshal(applied)
	if err != nil {
		return applied, cur, err
	}
	inserted, err := f.repo.InsertConfirmation(ctx, c.TallerID, cur.ID, key, c.Actor.ID, raw, now)
	if err == nil && !inserted {
		err = ErrFloorVersion
	}
	if err != nil {
		return applied, cur, err
	}
	text := floorConfirmText + " " + joinY(applied.Done) + "."
	if st, err := f.shop.OrderStatus(ctx, c.TallerID, oid); err == nil && st.State == "por_recibir" && len(st.Missing) > 0 {
		text += " Para pasar a Recibido falta: " + strings.Join(st.Missing, ", ") + "."
	}
	done := FloorMessage{ID: shared.NewID(), ChatID: cur.ID, Role: "asistente", Text: text,
		Proposal: &cur.Proposal, CreatedAt: now}
	applied.Message = &done
	if err := f.save(ctx, c.TallerID, cur, false, cur, prev, done); err != nil {
		return applied, cur, err
	}
	if err := f.audit.Record(ctx, audit.Entry{TallerID: c.TallerID, Actor: c.Actor, Action: "asistente.confirmar", Entity: "orden",
		EntityID: oid, After: map[string]any{"origen": "asistente", "chat_id": cur.ID.String(), "hecho": applied.Done,
			"orden_nueva": applied.Created, "fotos": applied.Photos}, OccurredAt: now}); err != nil {
		return applied, cur, err
	}
	return applied, cur, f.events.Publish(ctx, c.TallerID, shared.Event{Type: "assistant.propuesta_confirmada", AggregateID: oid, OccurredAt: now,
		Payload: map[string]any{"chat_id": cur.ID.String(), "orden_nueva": applied.Created}})
}

// replayed devuelve el resultado guardado de una confirmación con esa clave (found=false si no hay).
func (f *Floor) replayed(ctx context.Context, tallerID, chatID shared.ID, key string) (FloorApplied, FloorChat, bool, error) {
	raw, usedIn, ok, err := f.repo.Confirmation(ctx, tallerID, key)
	if err != nil || !ok {
		return FloorApplied{}, FloorChat{}, false, err
	}
	if usedIn != chatID { // la misma clave en otra conversación no es una repetición: es un error del cliente (R-07)
		return FloorApplied{}, FloorChat{}, false, ErrFloorKeyUsed
	}
	var a FloorApplied
	if err := json.Unmarshal(raw, &a); err != nil {
		return a, FloorChat{}, false, err
	}
	ch, err := f.repo.Chat(ctx, tallerID, chatID)
	return a, ch, true, err
}

// allowed revisa en el servidor que quien confirma pueda hacer cada parte (RBAC, R-05).
func (f *Floor) allowed(p domain.FloorProposal, roles []string) error {
	need := []string{"assistant:use"}
	if p.NewOrder {
		need = append(need, "order:create", "customer:create", "vehicle:create")
	}
	if len(p.Sheet) > 0 {
		need = append(need, "vehicle:update")
	}
	if p.Km != nil || p.Fuel != nil || len(p.Items) > 0 || len(p.Notes) > 0 || len(p.Photos) > 0 {
		need = append(need, "evidence:upload")
	}
	if len(p.Findings) > 0 {
		need = append(need, "order:diagnose")
	}
	if len(p.Quote) > 0 {
		need = append(need, "quote:edit")
	}
	if len(p.Expenses) > 0 {
		need = append(need, "expense:manage")
	}
	if p.Move != nil {
		need = append(need, "order:update_state")
	}
	for _, perm := range need {
		if !f.can(roles, perm) {
			return ErrFloorPerm
		}
	}
	return nil
}

// Photo devuelve una foto del chat (para mostrarla en la conversación).
func (f *Floor) Photo(ctx context.Context, tallerID, chatID shared.ID, ref string) ([]byte, error) {
	if !strings.HasPrefix(ref, tallerID.String()+"/") {
		return nil, ErrFloorChat
	}
	found := false
	if err := f.uow.Do(ctx, func(ctx context.Context) error {
		msgs, err := f.repo.Messages(ctx, tallerID, chatID, 200)
		for _, m := range msgs {
			for _, a := range m.Attachments {
				found = found || a.Ref == ref
			}
		}
		return err
	}); err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrFloorChat
	}
	return f.files.Get(ctx, ref)
}

// AnonymizeOrders borra los textos de las conversaciones del piso de esas órdenes (HU-87), en la transacción del
// borrado del titular.
func (f *Floor) AnonymizeOrders(ctx context.Context, tallerID shared.ID, orders []shared.ID) (int, error) {
	var n int
	err := f.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		n, err = f.repo.AnonymizeOrders(ctx, tallerID, orders)
		return err
	})
	return n, err
}

// joinY junta en español: "a, b y c".
func joinY(items []string) string {
	switch len(items) {
	case 0:
		return "nada nuevo"
	case 1:
		return items[0]
	}
	return strings.Join(items[:len(items)-1], ", ") + " y " + items[len(items)-1]
}

func cloneProposal(p domain.FloorProposal) domain.FloorProposal {
	raw, _ := json.Marshal(p)
	var out domain.FloorProposal
	_ = json.Unmarshal(raw, &out)
	return out
}
