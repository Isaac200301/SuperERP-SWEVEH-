package meta

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"time"

	"sweveh/internal/messaging/app"
	"sweveh/internal/shared"
)

// SignatureHeader es el encabezado con la firma del cuerpo: "sha256=" + hex(HMAC_SHA256(app secret, cuerpo)).
const SignatureHeader = "X-Hub-Signature-256"

// ValidSignature valida la firma del webhook en tiempo constante. Sin secreto, rechaza (R-05, R-09).
func ValidSignature(secret string, body []byte, header string) bool {
	if secret == "" || !strings.HasPrefix(header, "sha256=") {
		return false
	}
	got, err := hex.DecodeString(strings.TrimPrefix(header, "sha256="))
	if err != nil {
		return false
	}
	m := hmac.New(sha256.New, []byte(secret))
	m.Write(body)
	return hmac.Equal(got, m.Sum(nil))
}

// VerifyChallenge responde la verificación de la suscripción (GET con hub.mode, hub.verify_token y hub.challenge).
// Devuelve el challenge solo si el token coincide, como número: quien responde lo escribe con strconv y nunca
// refleja texto ajeno.
func VerifyChallenge(verifyToken string, q url.Values) (uint64, bool) {
	if verifyToken == "" || q.Get("hub.mode") != "subscribe" ||
		subtle.ConstantTimeCompare([]byte(q.Get("hub.verify_token")), []byte(verifyToken)) != 1 {
		return 0, false
	}
	n, err := strconv.ParseUint(q.Get("hub.challenge"), 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

// Parser interpreta el webhook de WhatsApp Cloud API (entry → changes → value). Campos desconocidos se ignoran.
type Parser struct{}

var _ app.InboundParser = Parser{}

type webhook struct {
	Entry []struct {
		Changes []struct {
			Value struct {
				Metadata struct {
					PhoneNumberID string `json:"phone_number_id"`
				} `json:"metadata"`
				Contacts []struct {
					Profile struct {
						Name string `json:"name"`
					} `json:"profile"`
				} `json:"contacts"`
				Messages []message `json:"messages"`
				Statuses []struct {
					ID     string `json:"id"`
					Status string `json:"status"`
				} `json:"statuses"`
			} `json:"value"`
		} `json:"changes"`
	} `json:"entry"`
}

type message struct {
	From      string `json:"from"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Text      struct {
		Body string `json:"body"`
	} `json:"text"`
	Interactive struct {
		ButtonReply struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"button_reply"`
		ListReply struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"list_reply"`
	} `json:"interactive"`
	Button struct { // respuesta a un botón de plantilla
		Payload string `json:"payload"`
		Text    string `json:"text"`
	} `json:"button"`
	Audio struct { // nota de voz: se descarga por su id (HU-60)
		ID       string `json:"id"`
		MimeType string `json:"mime_type"`
	} `json:"audio"`
}

var states = map[string]string{"sent": "enviado", "delivered": "entregado", "read": "leido", "failed": "fallido"}

func (Parser) Parse(body []byte) ([]app.Inbound, []app.Status, error) {
	var w webhook
	if err := json.Unmarshal(body, &w); err != nil {
		return nil, nil, err
	}
	var ins []app.Inbound
	var sts []app.Status
	for _, e := range w.Entry {
		for _, c := range e.Changes {
			v := c.Value
			name := ""
			if len(v.Contacts) > 0 {
				name = v.Contacts[0].Profile.Name
			}
			for _, m := range v.Messages {
				if in, ok := inbound(v.Metadata.PhoneNumberID, name, m); ok {
					ins = append(ins, in)
				}
			}
			for _, s := range v.Statuses {
				if state := states[s.Status]; state != "" {
					sts = append(sts, app.Status{PhoneNumberID: v.Metadata.PhoneNumberID, WAMID: s.ID, State: state})
				}
			}
		}
	}
	return ins, sts, nil
}

func inbound(pnid, name string, m message) (app.Inbound, bool) {
	in := app.Inbound{PhoneNumberID: pnid, From: "+" + m.From, WAMID: m.ID, Name: name, At: time.Now().UTC()}
	if ts, err := strconv.ParseInt(m.Timestamp, 10, 64); err == nil {
		in.At = time.Unix(ts, 0).UTC()
	}
	switch m.Type {
	case "text":
		in.Kind, in.Text = "texto", m.Text.Body
	case "interactive":
		in.Kind, in.ButtonID, in.Text = "respuesta_boton", m.Interactive.ButtonReply.ID, m.Interactive.ButtonReply.Title
		if in.ButtonID == "" { // opción de una lista (HU-57)
			in.ButtonID, in.Text = m.Interactive.ListReply.ID, m.Interactive.ListReply.Title
		}
	case "button":
		in.Kind, in.ButtonID, in.Text = "respuesta_boton", m.Button.Payload, m.Button.Text
	case "audio":
		in.Kind, in.Media, in.Mime = "audio", m.Audio.ID, m.Audio.MimeType
	case "image":
		in.Kind = "imagen"
	default:
		return in, false
	}
	return in, true
}

// SimulatedBody arma un webhook con el formato de Cloud API: lo usa el simulador local para que el mensaje
// recorra el mismo camino que uno real (cola de webhooks → worker → agente).
func SimulatedBody(pnid, phone, name, text, button string, at time.Time) ([]byte, error) {
	msg := map[string]any{"from": strings.TrimPrefix(phone, "+"), "id": "wamid.local." + shared.NewID().String(),
		"timestamp": strconv.FormatInt(at.Unix(), 10), "type": "text", "text": map[string]string{"body": text}}
	if button != "" {
		msg["type"] = "interactive"
		msg["interactive"] = map[string]any{"type": "button_reply", "button_reply": map[string]string{"id": button, "title": text}}
	}
	return json.Marshal(map[string]any{"object": "whatsapp_business_account", "entry": []any{map[string]any{"changes": []any{
		map[string]any{"field": "messages", "value": map[string]any{"messaging_product": "whatsapp",
			"metadata": map[string]string{"phone_number_id": pnid}, "contacts": []any{map[string]any{"profile": map[string]string{"name": name},
				"wa_id": strings.TrimPrefix(phone, "+")}}, "messages": []any{msg}}}}}}})
}
