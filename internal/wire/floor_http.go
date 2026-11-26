package wire

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	assistantapp "sweveh/internal/assistant/app"
	assistantdomain "sweveh/internal/assistant/domain"
	"sweveh/internal/platform/httpx"
	"sweveh/internal/platform/reqctx"
	"sweveh/internal/shared"
)

// Rutas del asistente del piso (R8, ADR-0041). Siempre montadas: no dependen de WhatsApp. Todas piden assistant:use
// (los 3 roles); al confirmar, el caso de uso revisa además el permiso de cada parte (crear orden, cotizar, gastos…).
func (h *handler) floorRoutes(r chi.Router, j func(httpx.Func) http.HandlerFunc) {
	use := h.g.Require("assistant:use")
	r.With(use).Post("/assistant/floor/messages", j(h.floorSend))
	r.With(use).Get("/assistant/floor/latest", j(h.floorLatest))
	r.With(use).Get("/assistant/floor/chats/{id}", j(h.floorChat))
	r.With(use).Patch("/assistant/floor/chats/{id}", j(h.floorEdit))
	r.With(use).Post("/assistant/floor/chats/{id}/confirm", j(h.floorConfirm))
	r.With(use).Get("/assistant/floor/chats/{id}/photos/{photo}", h.floorPhoto)
	r.With(use).Get("/orders/{id}/assistant", j(h.floorOrder))
}

type floorMsgDTO struct {
	ID       string           `json:"id"`
	Rol      string           `json:"rol"`
	Texto    string           `json:"texto"`
	Adjuntos []floorAttachDTO `json:"adjuntos"`
	Opciones []string         `json:"opciones"`
	Creado   time.Time        `json:"creadoEn"`
	Prop     *bool            `json:"conPropuesta,omitempty"`
}

type floorAttachDTO struct {
	Tipo string `json:"tipo"`
	URL  string `json:"url,omitempty"`
}

type floorChatDTO struct {
	ID         string                        `json:"id"`
	OrdenID    *string                       `json:"ordenId"`
	Version    int                           `json:"version"`
	Propuesta  assistantdomain.FloorProposal `json:"propuesta"`
	Pendiente  bool                          `json:"pendiente"`
	Progreso   assistantdomain.Progress      `json:"progreso"`
	Mensajes   []floorMsgDTO                 `json:"mensajes"`
	Disponible bool                          `json:"disponible"`
	Voz        bool                          `json:"voz"`
}

func (h *handler) floorChatOut(c assistantapp.FloorChat, msgs []assistantapp.FloorMessage, pr assistantdomain.Progress, available bool) floorChatDTO {
	d := floorChatDTO{ID: c.ID.String(), Version: c.Version, Propuesta: c.Proposal, Pendiente: c.Proposal.Pending(), Progreso: pr,
		Mensajes: make([]floorMsgDTO, 0, len(msgs)), Disponible: available, Voz: h.s.Floor.HasVoice()}
	if d.Progreso.Missing == nil {
		d.Progreso.Missing = []string{}
	}
	if c.OrderID != nil {
		s := c.OrderID.String()
		d.OrdenID = &s
	}
	for _, m := range msgs {
		md := floorMsgDTO{ID: m.ID.String(), Rol: m.Role, Texto: m.Text, Adjuntos: []floorAttachDTO{}, Opciones: m.Choices, Creado: m.CreatedAt}
		if md.Opciones == nil {
			md.Opciones = []string{}
		}
		for _, a := range m.Attachments {
			ad := floorAttachDTO{Tipo: a.Kind}
			if a.Kind == "foto" {
				_, id, _ := strings.Cut(a.Ref, "/")
				ad.URL = "/api/v1/assistant/floor/chats/" + c.ID.String() + "/photos/" + id
			}
			md.Adjuntos = append(md.Adjuntos, ad)
		}
		if m.Proposal != nil && m.Proposal.Pending() && m.Role == "asistente" {
			t := true
			md.Prop = &t
		}
		d.Mensajes = append(d.Mensajes, md)
	}
	return d
}

var errFloorChat = shared.NotFound("chat_no_encontrado", "No encontramos esa conversación.")

// floorSend recibe un mensaje (multipart): texto, chatId u ordenId, una nota de voz (audio) y hasta 4 fotos (fotos).
func (h *handler) floorSend(r *http.Request, p reqctx.Principal) (int, any, error) {
	r.Body = http.MaxBytesReader(nil, r.Body, 32<<20)
	mr, err := r.MultipartReader()
	if err != nil {
		return 0, nil, httpx.ErrBadJSON.WithMessage("Envía el mensaje como formulario (multipart).")
	}
	in := assistantapp.FloorInput{TallerID: p.TallerID, Actor: p.Actor()}
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return 0, nil, httpx.ErrBadJSON.WithMessage("El mensaje llegó incompleto.")
		}
		if err := floorPart(&in, part); err != nil {
			return 0, nil, err
		}
	}
	res, err := h.s.Floor.Send(r.Context(), in)
	if err != nil {
		return 0, nil, err
	}
	pr, err := h.s.Floor.Progress(r.Context(), res.Chat)
	if err != nil {
		return 0, nil, err
	}
	out := h.floorChatOut(res.Chat, res.Messages, pr, res.Available)
	return http.StatusOK, map[string]any{"chat": out}, nil
}

func floorPart(in *assistantapp.FloorInput, part interface {
	io.Reader
	FormName() string
	FileName() string
}) error {
	read := func(max int64) ([]byte, error) {
		b, err := io.ReadAll(io.LimitReader(part, max+1))
		if err != nil {
			return nil, httpx.ErrBadJSON.WithMessage("El mensaje llegó incompleto.")
		}
		return b, nil
	}
	switch part.FormName() {
	case "texto":
		b, err := read(4 * assistantapp.FloorMaxText)
		if err != nil {
			return err
		}
		in.Text = string(b)
	case "chatId", "ordenId":
		b, err := read(64)
		if err != nil {
			return err
		}
		id, err := shared.ParseID(strings.TrimSpace(string(b)))
		if err != nil {
			return errFloorChat
		}
		if part.FormName() == "chatId" {
			in.ChatID = &id
		} else {
			in.OrderID = &id
		}
	case "audio":
		b, err := read(assistantapp.FloorMaxAudio)
		if err != nil {
			return err
		}
		if len(b) > assistantapp.FloorMaxAudio {
			return assistantapp.ErrFloorAudio
		}
		in.Audio = &assistantapp.Blob{Data: b, Mime: audioMime(part)}
	case "fotos":
		b, err := read(assistantapp.FloorMaxPhoto)
		if err != nil {
			return err
		}
		if len(b) > assistantapp.FloorMaxPhoto {
			return assistantapp.ErrFloorPhoto
		}
		in.Photos = append(in.Photos, assistantapp.Blob{Data: b})
		if len(in.Photos) > assistantapp.FloorMaxPhotos {
			return assistantapp.ErrFloorPhotos
		}
	}
	return nil
}

// audioMime toma el tipo que dice el navegador (webm en Android, mp4 en iPhone); Groq lo usa para nombrar el archivo.
func audioMime(part interface{ FileName() string }) string {
	switch n := strings.ToLower(part.FileName()); {
	case strings.HasSuffix(n, ".mp4"), strings.HasSuffix(n, ".m4a"):
		return "audio/mp4"
	case strings.HasSuffix(n, ".ogg"):
		return "audio/ogg"
	case strings.HasSuffix(n, ".wav"):
		return "audio/wav"
	}
	return "audio/webm"
}

func (h *handler) floorChat(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := httpx.PathID(r, "id", errFloorChat)
	if err != nil {
		return 0, nil, err
	}
	v, err := h.s.Floor.Get(r.Context(), p.TallerID, id)
	if err != nil {
		return 0, nil, err
	}
	return http.StatusOK, map[string]any{"chat": h.floorChatOut(v.Chat, v.Messages, v.Progress, h.s.Floor.HasModel())}, nil
}

// floorLatest es la última conversación de la persona en las últimas 12 horas (la pestaña Asistente la retoma).
func (h *handler) floorLatest(r *http.Request, p reqctx.Principal) (int, any, error) {
	v, err := h.s.Floor.Latest(r.Context(), p.TallerID, p.UserID, 12*time.Hour)
	if err != nil {
		return 0, nil, err
	}
	out := map[string]any{"chat": nil, "disponible": h.s.Floor.HasModel(), "voz": h.s.Floor.HasVoice()}
	if v != nil {
		out["chat"] = h.floorChatOut(v.Chat, v.Messages, v.Progress, h.s.Floor.HasModel())
	}
	return http.StatusOK, out, nil
}

func (h *handler) floorOrder(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := httpx.PathID(r, "id", shared.NotFound("orden_no_encontrada", "No encontramos esa orden en tu taller."))
	if err != nil {
		return 0, nil, err
	}
	v, err := h.s.Floor.ForOrder(r.Context(), p.TallerID, id)
	if err != nil {
		return 0, nil, err
	}
	out := map[string]any{"chat": nil, "disponible": h.s.Floor.HasModel(), "voz": h.s.Floor.HasVoice()}
	if v != nil {
		out["chat"] = h.floorChatOut(v.Chat, v.Messages, v.Progress, h.s.Floor.HasModel())
	}
	return http.StatusOK, out, nil
}

// floorEdit guarda lo que la persona cambió o quitó en la ficha en vivo.
func (h *handler) floorEdit(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := httpx.PathID(r, "id", errFloorChat)
	if err != nil {
		return 0, nil, err
	}
	var q struct {
		Version   int                           `json:"version"`
		Propuesta assistantdomain.FloorProposal `json:"propuesta"`
	}
	if err := httpx.Decode(r, &q); err != nil {
		return 0, nil, err
	}
	c, err := h.s.Floor.Edit(r.Context(), p.TallerID, id, q.Version, q.Propuesta)
	if err != nil {
		return 0, nil, err
	}
	v, err := h.s.Floor.Get(r.Context(), p.TallerID, c.ID)
	if err != nil {
		return 0, nil, err
	}
	return http.StatusOK, map[string]any{"chat": h.floorChatOut(v.Chat, v.Messages, v.Progress, h.s.Floor.HasModel())}, nil
}

// floorConfirm aplica la propuesta (Idempotency-Key obligatoria).
func (h *handler) floorConfirm(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := httpx.PathID(r, "id", errFloorChat)
	if err != nil {
		return 0, nil, err
	}
	var q struct {
		Version int `json:"version"`
	}
	if err := httpx.Decode(r, &q); err != nil {
		return 0, nil, err
	}
	applied, _, err := h.s.Floor.Confirm(r.Context(), assistantapp.ConfirmCmd{TallerID: p.TallerID, ChatID: id, Actor: p.Actor(), Roles: p.Roles,
		Key: r.Header.Get("Idempotency-Key"), Version: q.Version})
	if err != nil {
		return 0, nil, err
	}
	v, err := h.s.Floor.Get(r.Context(), p.TallerID, id)
	if err != nil {
		return 0, nil, err
	}
	return http.StatusOK, map[string]any{
		"resultado": map[string]any{"ordenId": applied.OrderID.String(), "ordenCodigo": applied.OrderCode, "ordenEstado": applied.OrderState,
			"ordenNueva": applied.Created, "hecho": nonEmpty(applied.Done), "fotos": applied.Photos},
		"chat": h.floorChatOut(v.Chat, v.Messages, v.Progress, h.s.Floor.HasModel()),
	}, nil
}

func nonEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// floorPhoto sirve una foto del chat. Solo si está en un mensaje de esa conversación del taller.
func (h *handler) floorPhoto(w http.ResponseWriter, r *http.Request) {
	p, _ := reqctx.PrincipalFrom(r.Context())
	id, err := httpx.PathID(r, "id", errFloorChat)
	if err != nil {
		httpx.WriteError(w, r, h.log, err)
		return
	}
	photo, err := httpx.PathID(r, "photo", errFloorChat)
	if err != nil {
		httpx.WriteError(w, r, h.log, err)
		return
	}
	data, err := h.s.Floor.Photo(r.Context(), p.TallerID, id, p.TallerID.String()+"/"+photo.String())
	if err != nil {
		httpx.WriteError(w, r, h.log, err)
		return
	}
	ct := assistantdomain.SniffImage(data) // solo se guardan fotos validadas por su contenido
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = io.Copy(w, bytes.NewReader(data))
}
