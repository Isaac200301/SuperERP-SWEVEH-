package app

import (
	"context"
	"strings"
	"time"

	"sweveh/internal/assistant/domain"
	"sweveh/internal/shared"
)

// Recepción por video (R9, V-12, ADR-0042, HU-108). El celular graba hasta 90 s y manda solo el audio y de 8 a 10
// cuadros: el video completo nunca llega al servidor (minimización, Ley 1581). El audio pasa a texto con Groq y los
// cuadros van al modelo de visión con las tools del asistente del piso (R8); el resultado es la misma propuesta, con
// origen "video", en el chat de la orden. Nada se guarda hasta Confirmar (R-08).

// VideoMaxFrame es el tamaño máximo de un cuadro: el celular los manda de 1280 px como máximo (unos 300 KB), así que 10
// cuadros y el audio caben en los 32 MB del envío.
const VideoMaxFrame = 2 << 20

// floorVideoSystem se suma al prompt del asistente cuando el mensaje es un video. Es texto fijo del código: lo que
// dijo la persona sigue llegando aparte, como dato (R-08).
const floorVideoSystem = `Este mensaje es una RECEPCIÓN POR VIDEO: el texto es la transcripción del audio que grabó la persona mientras recorría el vehículo y las imágenes son cuadros del video, en orden (1 = el primero).
- Usa proponer_recepcion con "lados": para cada cuadro donde se vea claramente la placa, el tablero, el frente, el lado izquierdo, la parte trasera o el lado derecho, di cuál es. Solo lo que se ve de verdad; si no estás seguro, no lo pongas.
- Kilometraje y combustible: solo si se leen en el tablero (de_foto = true) o la persona los dijo.
- Notas: golpes, rayones o faltantes que se vean o que la persona mencionó. No diagnostiques.
- Si la placa que se ve no es la de la orden, dilo.
- Al final di en una o dos frases qué anotaste. El código agrega qué lados no se vieron.
`

var (
	ErrVideoFrames = shared.Validation("cuadros_invalidos", "Manda de 1 a 10 cuadros del video, en JPG, PNG o WEBP de máximo 2 MB.").WithField("cuadros")
	ErrVideoState  = shared.Conflict("orden_ya_recibida", "La recepción por video es para órdenes por recibir.")
	ErrVideoLength = shared.Validation("video_largo", "El video dura máximo 90 segundos.").WithField("segundos")
)

// VideoInput es un video de recepción: el audio (opcional) y los cuadros. Key es la Idempotency-Key.
type VideoInput struct {
	TallerID shared.ID
	Actor    shared.Actor
	Roles    []string
	OrderID  shared.ID
	Key      string
	Seconds  int
	Audio    *Blob
	Frames   []Blob
}

// Video procesa un video de recepción. Repetir con la misma Idempotency-Key devuelve la conversación como quedó la
// primera vez, sin volver a llamar a la IA ni repetir los cuadros.
func (f *Floor) Video(ctx context.Context, in VideoInput) (FloorReply, error) {
	if err := validateVideo(&in); err != nil {
		return FloorReply{}, err
	}
	// El id del mensaje sale de la clave: dos envíos con la misma clave son el mismo mensaje.
	msgID := shared.NameID(in.TallerID, "video:"+strings.TrimSpace(in.Key))
	if done, ok, err := f.videoReplay(ctx, in.TallerID, in.OrderID, msgID); err != nil || ok {
		return done, err
	}
	if err := f.videoOrder(ctx, in.TallerID, in.OrderID); err != nil {
		return FloorReply{}, err
	}
	fin := FloorInput{TallerID: in.TallerID, Actor: in.Actor, Roles: in.Roles, OrderID: &in.OrderID, Audio: in.Audio,
		Photos: in.Frames, Video: true}
	chat, isNew, err := f.chatFor(ctx, fin)
	if err != nil {
		return FloorReply{}, err
	}
	now := f.clock.Now()
	user, voiceFailed, err := f.userMessage(ctx, fin, chat.ID, "", now)
	if err != nil {
		return FloorReply{}, err
	}
	user.ID = msgID
	user.Attachments = append([]FloorAttachment{{Kind: "video", Seconds: in.Seconds}}, user.Attachments...)
	history, err := f.history(ctx, in.TallerID, chat.ID, isNew)
	if err != nil {
		return FloorReply{}, err
	}
	reply := FloorMessage{ID: shared.NewID(), ChatID: chat.ID, Role: "asistente", CreatedAt: now.Add(time.Millisecond)}
	next, available, err := f.answer(ctx, chat, history, user, fin, voiceFailed, &reply)
	if err != nil {
		return FloorReply{}, err
	}
	// Los cuadros van a la propuesta aunque la IA no responda: no se pierden y se confirman como fotos de la orden.
	for _, a := range user.Attachments {
		if a.Kind == "foto" {
			next.Proposal.Photos = upsertPhoto(next.Proposal.Photos, domain.FloorPhoto{Ref: a.Ref, Origin: domain.FromVideo})
		}
	}
	if available {
		saved, err := f.savedFor(ctx, next)
		if err != nil {
			return FloorReply{}, err
		}
		if miss := domain.GuideMissing(next.Proposal.Guide(saved)); miss != "" {
			reply.Text = strings.TrimSpace(reply.Text + " " + miss)
		}
	}
	snap := next.Proposal
	reply.Proposal = &snap
	prev := chat.Version
	next.Version, next.UpdatedAt = chat.Version+1, now
	if err := f.save(ctx, in.TallerID, chat, isNew, next, prev, user, reply); err != nil {
		// Dos envíos a la vez con la misma clave: el segundo choca con el mensaje del primero y devuelve ese.
		if done, ok, rerr := f.videoReplay(ctx, in.TallerID, in.OrderID, msgID); rerr == nil && ok {
			return done, nil
		}
		return FloorReply{}, err
	}
	return FloorReply{Chat: next, Messages: []FloorMessage{user, reply}, Available: available}, nil
}

// validateVideo revisa la duración, el audio y cada cuadro por su contenido.
func validateVideo(in *VideoInput) error {
	key := strings.TrimSpace(in.Key)
	switch {
	case len(key) < 8 || len(key) > 100:
		return ErrFloorKey
	case in.Seconds < 0 || in.Seconds > domain.VideoMaxSeconds:
		return ErrVideoLength
	case len(in.Frames) == 0 || len(in.Frames) > domain.VideoMaxFrames:
		return ErrVideoFrames
	case in.Audio != nil && len(in.Audio.Data) > FloorMaxAudio:
		return ErrFloorAudio
	}
	for i := range in.Frames {
		mime := domain.SniffImage(in.Frames[i].Data)
		if mime == "" || len(in.Frames[i].Data) > VideoMaxFrame {
			return ErrVideoFrames
		}
		in.Frames[i].Mime = mime
	}
	return nil
}

// videoOrder: la orden es de este taller (RLS, anti-IDOR) y está por recibir.
func (f *Floor) videoOrder(ctx context.Context, tallerID, orderID shared.ID) error {
	return f.uow.Do(ctx, func(ctx context.Context) error {
		o, err := f.shop.Order(ctx, tallerID, orderID)
		if err != nil {
			return err
		}
		if o.State != "por_recibir" {
			return ErrVideoState
		}
		return nil
	})
}

// videoReplay devuelve la conversación si el mensaje de esa clave ya existe (ok=false si no). La misma clave en otra
// orden no es una repetición: es un error del cliente (409 clave_usada, como en R8).
func (f *Floor) videoReplay(ctx context.Context, tallerID, orderID, msgID shared.ID) (FloorReply, bool, error) {
	var (
		out FloorReply
		ok  bool
	)
	err := f.uow.Do(ctx, func(ctx context.Context) error {
		chatID, found, err := f.repo.MessageChat(ctx, tallerID, msgID)
		if err != nil || !found {
			return err
		}
		c, err := f.repo.Chat(ctx, tallerID, chatID)
		if err != nil {
			return err
		}
		if c.OrderID == nil || *c.OrderID != orderID {
			return ErrFloorKeyUsed
		}
		msgs, err := f.repo.Messages(ctx, tallerID, chatID, 200)
		if err != nil {
			return err
		}
		for i, m := range msgs { // el mensaje del video y la respuesta que le siguió
			if m.ID == msgID {
				out.Messages = msgs[i:min(i+2, len(msgs))]
			}
		}
		out.Chat, out.Available, ok = c, f.model != nil, true
		return nil
	})
	return out, ok, err
}

// savedFor es lo que la orden ya tiene guardado de su recepción (para la guía de lados).
func (f *Floor) savedFor(ctx context.Context, c FloorChat) (domain.Saved, error) {
	var saved domain.Saved
	if c.OrderID == nil {
		return saved, nil
	}
	err := f.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		saved, err = f.shop.Saved(ctx, c.TallerID, *c.OrderID)
		return err
	})
	return saved, err
}
