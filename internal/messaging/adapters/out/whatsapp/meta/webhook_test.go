package meta_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"testing"
	"time"

	"sweveh/internal/messaging/adapters/out/whatsapp/meta"
)

// Cuerpo con la forma de los ejemplos oficiales de webhooks de Cloud API (mensaje de texto, respuesta a botón
// interactivo, respuesta a botón de plantilla y estados), con datos Ficticio:
// https://developers.facebook.com/docs/whatsapp/cloud-api/webhooks/components
const sample = `{
  "object": "whatsapp_business_account",
  "entry": [{
    "id": "WABA_ID",
    "changes": [{
      "field": "messages",
      "value": {
        "messaging_product": "whatsapp",
        "metadata": {"display_phone_number": "15550001111", "phone_number_id": "PNID-1"},
        "contacts": [{"profile": {"name": "Laura Gómez"}, "wa_id": "573001112233"}],
        "messages": [
          {"from": "573001112233", "id": "wamid.T1", "timestamp": "1762200000", "type": "text", "text": {"body": "¿Ya está mi carro?"}},
          {"from": "573001112233", "id": "wamid.B1", "timestamp": "1762200060", "type": "interactive",
           "interactive": {"type": "button_reply", "button_reply": {"id": "quote:aprobar:42", "title": "Aprobar"}}},
          {"from": "573001112233", "id": "wamid.P1", "timestamp": "1762200120", "type": "button", "button": {"payload": "consent:si", "text": "Acepto"}},
          {"from": "573001112233", "id": "wamid.L1", "timestamp": "1762200150", "type": "interactive",
           "interactive": {"type": "list_reply", "list_reply": {"id": "estado:7", "title": "ABC12D"}}},
          {"from": "573001112233", "id": "wamid.S1", "timestamp": "1762200180", "type": "sticker", "sticker": {"id": "x"}}
        ],
        "statuses": [
          {"id": "wamid.OUT1", "status": "delivered", "timestamp": "1762200200", "recipient_id": "573001112233"},
          {"id": "wamid.OUT2", "status": "failed", "timestamp": "1762200201", "recipient_id": "573001112233"},
          {"id": "wamid.OUT3", "status": "deleted", "timestamp": "1762200202", "recipient_id": "573001112233"}
        ]
      }
    }]
  }]
}`

func TestParser_MessagesButtonsAndStatuses(t *testing.T) {
	ins, sts, err := meta.Parser{}.Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	if len(ins) != 4 { // el sticker se ignora
		t.Fatalf("mensajes: %+v", ins)
	}
	if in := ins[0]; in.PhoneNumberID != "PNID-1" || in.From != "+573001112233" || in.Kind != "texto" || in.Text != "¿Ya está mi carro?" ||
		in.Name != "Laura Gómez" || in.WAMID != "wamid.T1" || !in.At.Equal(time.Unix(1762200000, 0).UTC()) {
		t.Fatalf("texto: %+v", in)
	}
	if in := ins[1]; in.Kind != "respuesta_boton" || in.ButtonID != "quote:aprobar:42" || in.Text != "Aprobar" {
		t.Fatalf("botón interactivo: %+v", in)
	}
	if in := ins[2]; in.Kind != "respuesta_boton" || in.ButtonID != "consent:si" {
		t.Fatalf("botón de plantilla: %+v", in)
	}
	if in := ins[3]; in.Kind != "respuesta_boton" || in.ButtonID != "estado:7" || in.Text != "ABC12D" {
		t.Fatalf("opción de lista: %+v", in)
	}
	if len(sts) != 2 || sts[0].State != "entregado" || sts[1].State != "fallido" || sts[1].WAMID != "wamid.OUT2" {
		t.Fatalf("estados: %+v", sts) // "deleted" no es un estado que se guarde
	}
	if _, _, err := (meta.Parser{}).Parse([]byte("no es json")); err == nil {
		t.Fatal("un cuerpo roto debía fallar")
	}
}

func TestSimulatedBody_RoundTrips(t *testing.T) {
	at := time.Unix(1762200000, 0).UTC()
	body, err := meta.SimulatedBody("local-1", "+573001112233", "Laura", "Aprobar", "quote:aprobar:42", at)
	if err != nil {
		t.Fatal(err)
	}
	ins, _, err := meta.Parser{}.Parse(body)
	if err != nil || len(ins) != 1 || ins[0].ButtonID != "quote:aprobar:42" || ins[0].PhoneNumberID != "local-1" || !ins[0].At.Equal(at) {
		t.Fatalf("simulado: %+v %v", ins, err)
	}
}

func TestValidSignature(t *testing.T) {
	body := []byte(sample)
	m := hmac.New(sha256.New, []byte("secreto"))
	m.Write(body)
	good := "sha256=" + hex.EncodeToString(m.Sum(nil))
	if !meta.ValidSignature("secreto", body, good) {
		t.Fatal("firma buena rechazada")
	}
	for name, tc := range map[string]struct{ secret, header string }{
		"otro secreto": {"otro", good}, "sin prefijo": {"secreto", good[7:]}, "hex roto": {"secreto", "sha256=zz"},
		"sin secreto": {"", good}, "vacía": {"secreto", ""},
	} {
		if meta.ValidSignature(tc.secret, body, tc.header) {
			t.Errorf("%s: firma aceptada", name)
		}
	}
	if meta.ValidSignature("secreto", append(body, ' '), good) {
		t.Error("cuerpo alterado aceptado")
	}
}

func TestVerifyChallenge(t *testing.T) {
	q := url.Values{"hub.mode": {"subscribe"}, "hub.verify_token": {"tok"}, "hub.challenge": {"1158201444"}}
	if c, ok := meta.VerifyChallenge("tok", q); !ok || c != 1158201444 {
		t.Fatalf("challenge: %d %v", c, ok)
	}
	bad := []url.Values{
		{"hub.mode": {"subscribe"}, "hub.verify_token": {"otro"}, "hub.challenge": {"1"}},
		{"hub.mode": {"unsubscribe"}, "hub.verify_token": {"tok"}, "hub.challenge": {"1"}},
		{"hub.mode": {"subscribe"}, "hub.verify_token": {"tok"}, "hub.challenge": {"<script>"}},
	}
	for _, b := range bad {
		if _, ok := meta.VerifyChallenge("tok", b); ok {
			t.Errorf("aceptado: %v", b)
		}
	}
	if _, ok := meta.VerifyChallenge("", q); ok {
		t.Error("sin token configurado no se verifica")
	}
}
