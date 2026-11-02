package app

import (
	"encoding/json"
	"strconv"
	"time"
)

// Status es una actualización de entrega de un mensaje saliente.
type Status struct {
	PhoneNumberID string
	WAMID         string
	State         string // enviado | entregado | leido | fallido
}

// ParseMeta interpreta el cuerpo del webhook de WhatsApp Cloud API (entry → changes → value).
// Formato según la documentación de Meta (docs/06 §16.1); campos desconocidos se ignoran.
func ParseMeta(body []byte) ([]Inbound, []Status, error) {
	var w struct {
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
						WaID string `json:"wa_id"`
					} `json:"contacts"`
					Messages []struct {
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
						} `json:"interactive"`
						Button struct {
							Payload string `json:"payload"`
							Text    string `json:"text"`
						} `json:"button"`
					} `json:"messages"`
					Statuses []struct {
						ID     string `json:"id"`
						Status string `json:"status"`
					} `json:"statuses"`
				} `json:"value"`
			} `json:"changes"`
		} `json:"entry"`
	}
	if err := json.Unmarshal(body, &w); err != nil {
		return nil, nil, err
	}
	var ins []Inbound
	var sts []Status
	for _, e := range w.Entry {
		for _, c := range e.Changes {
			v := c.Value
			name := ""
			if len(v.Contacts) > 0 {
				name = v.Contacts[0].Profile.Name
			}
			for _, m := range v.Messages {
				in := Inbound{PhoneNumberID: v.Metadata.PhoneNumberID, From: "+" + m.From, WAMID: m.ID, Name: name, At: time.Now().UTC()}
				if ts, err := strconv.ParseInt(m.Timestamp, 10, 64); err == nil {
					in.At = time.Unix(ts, 0).UTC()
				}
				switch m.Type {
				case "text":
					in.Kind, in.Text = "texto", m.Text.Body
				case "interactive":
					in.Kind, in.ButtonID, in.Text = "respuesta_boton", m.Interactive.ButtonReply.ID, m.Interactive.ButtonReply.Title
				case "button":
					in.Kind, in.ButtonID, in.Text = "respuesta_boton", m.Button.Payload, m.Button.Text
				case "audio":
					in.Kind = "audio"
				case "image":
					in.Kind = "imagen"
				default:
					continue
				}
				ins = append(ins, in)
			}
			for _, s := range v.Statuses {
				state := map[string]string{"sent": "enviado", "delivered": "entregado", "read": "leido", "failed": "fallido"}[s.Status]
				if state != "" {
					sts = append(sts, Status{PhoneNumberID: v.Metadata.PhoneNumberID, WAMID: s.ID, State: state})
				}
			}
		}
	}
	return ins, sts, nil
}
