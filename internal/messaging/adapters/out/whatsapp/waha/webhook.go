package waha

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"sweveh/internal/messaging/app"
)

// SignatureHeader trae hex(HMAC_SHA512(clave, cuerpo crudo)); la clave es WHATSAPP_HOOK_HMAC_KEY en WAHA.
const SignatureHeader = "X-Webhook-Hmac"

// ValidSignature valida la firma en tiempo constante. Sin clave, rechaza (R-05, R-09).
func ValidSignature(secret string, body []byte, header string) bool {
	if secret == "" || header == "" {
		return false
	}
	got, err := hex.DecodeString(strings.TrimSpace(header))
	if err != nil {
		return false
	}
	m := hmac.New(sha512.New, []byte(secret))
	m.Write(body)
	return hmac.Equal(got, m.Sum(nil))
}

// Parser interpreta los webhooks de WAHA: "message" (lo que escribe el cliente) y "message.ack" (entregas).
// El resto de eventos se ignora.
type Parser struct{}

var _ app.InboundParser = Parser{}

type event struct {
	Event   string `json:"event"`
	Session string `json:"session"`
	Payload struct {
		ID        string  `json:"id"`
		Timestamp float64 `json:"timestamp"`
		From      string  `json:"from"`
		FromMe    bool    `json:"fromMe"`
		Body      string  `json:"body"`
		HasMedia  bool    `json:"hasMedia"`
		Media     *struct {
			Mimetype string `json:"mimetype"`
		} `json:"media"`
		Ack  int `json:"ack"`
		Data struct {
			NotifyName string `json:"notifyName"`
			PushName   string `json:"pushName"`
		} `json:"_data"`
	} `json:"payload"`
}

// Is dice si el cuerpo es de WAHA (trae event y session); el de Meta trae "object".
func Is(body []byte) bool {
	var e struct {
		Event   string `json:"event"`
		Session string `json:"session"`
	}
	return json.Unmarshal(body, &e) == nil && e.Event != "" && e.Session != ""
}

// acks: -1 error, 0 pendiente, 1 en el servidor, 2 en el celular, 3 leído, 4 reproducido.
var acks = map[int]string{-1: "fallido", 1: "enviado", 2: "entregado", 3: "leido", 4: "leido"}

func (Parser) Parse(body []byte) ([]app.Inbound, []app.Status, error) {
	var e event
	if err := json.Unmarshal(body, &e); err != nil {
		return nil, nil, err
	}
	p, pnid := e.Payload, Prefix+e.Session
	if !direct(p.From) {
		return nil, nil, nil // grupos, canales y estados de WhatsApp no son clientes
	}
	switch e.Event {
	case "message":
		if p.FromMe {
			return nil, nil, nil
		}
		in := app.Inbound{PhoneNumberID: pnid, From: phone(p.From), WAMID: p.ID, Kind: "texto", Text: p.Body,
			Name: firstOf(p.Data.NotifyName, p.Data.PushName), At: time.Now().UTC()}
		if p.Timestamp > 0 {
			in.At = time.Unix(int64(p.Timestamp), 0).UTC()
		}
		if p.HasMedia {
			in.Kind = "imagen"
			if p.Media != nil && strings.HasPrefix(p.Media.Mimetype, "audio/") {
				in.Kind = "audio"
			}
		}
		return []app.Inbound{in}, nil, nil
	case "message.ack":
		if state := acks[p.Ack]; state != "" && p.FromMe {
			return nil, []app.Status{{PhoneNumberID: pnid, WAMID: p.ID, State: state}}, nil
		}
	}
	return nil, nil, nil
}

func direct(from string) bool {
	return strings.HasSuffix(from, "@c.us") || strings.HasSuffix(from, "@lid")
}

// phone: "573001234567@c.us" → "+573001234567". Un "@lid" (WhatsApp oculta el número) queda como id del chat,
// y ChatID lo usa tal cual para responder.
func phone(from string) string {
	if n, ok := strings.CutSuffix(from, "@c.us"); ok {
		return "+" + n
	}
	return from
}

func firstOf(s ...string) string {
	for _, x := range s {
		if x != "" {
			return x
		}
	}
	return ""
}
