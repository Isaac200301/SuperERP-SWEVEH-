package wire

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	messagingapp "sweveh/internal/messaging/app"
	"sweveh/internal/platform/httpx"
	"sweveh/internal/platform/reqctx"
	"sweveh/internal/shared"
)

var errNoVoice = shared.NotFound("audio_no_encontrado", "No encontramos esa nota de voz.")

// voiceNote sirve la nota de voz para escucharla en la conversación y en la tarjeta (HU-60). La primera vez se
// descarga del canal y se guarda en el almacenamiento del api (clave taller/mensaje); después sale de ahí.
func (h *handler) voiceNote(w http.ResponseWriter, r *http.Request) {
	p, _ := reqctx.PrincipalFrom(r.Context())
	conv, err := httpx.PathID(r, "id", errNoVoice)
	if err != nil {
		httpx.WriteError(w, r, h.log, err)
		return
	}
	mid, err := httpx.PathID(r, "mid", errNoVoice)
	if err != nil {
		httpx.WriteError(w, r, h.log, err)
		return
	}
	m, err := h.s.Messaging.GetMessage(r.Context(), p.TallerID, mid)
	if err != nil || m.ConversationID != conv || m.Kind != "audio" {
		httpx.WriteError(w, r, h.log, errNoVoice)
		return
	}
	var meta struct {
		Mime string `json:"mime"`
	}
	_ = json.Unmarshal(m.Payload, &meta)
	key := p.TallerID.String() + "/" + mid.String()
	st := h.s.M.Storage
	var body io.Reader
	if rc, err := st.Open(r.Context(), key); err == nil {
		defer func() { _ = rc.Close() }()
		body = rc
	} else {
		media, err := h.s.Messaging.VoiceNote(r.Context(), p.TallerID, mid)
		if errors.Is(err, messagingapp.ErrNoMedia) {
			httpx.WriteError(w, r, h.log, errNoVoice)
			return
		}
		if err != nil {
			httpx.WriteError(w, r, h.log, err)
			return
		}
		if media.Mime != "" {
			meta.Mime = media.Mime
		}
		if err := st.Put(r.Context(), key, bytes.NewReader(media.Data), messagingapp.MaxMediaBytes); err != nil {
			h.log.WarnContext(r.Context(), "no se guardó la nota de voz", "error", err.Error())
		}
		body = bytes.NewReader(media.Data)
	}
	if meta.Mime == "" {
		meta.Mime = "audio/ogg"
	}
	w.Header().Set("Content-Type", meta.Mime)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Header().Set("Content-Disposition", "inline")
	_, _ = io.Copy(w, body)
}
