package server_test

// Pablo (26-nov): en una orden Por recibir, "Guardar cambios" fallaba con "Los hallazgos se registran con el vehículo en
// el taller…" y no guardaba nada (ni las fotos ni el kilometraje), porque el asistente había propuesto un hallazgo y la
// orden todavía no los admite. Ahora se guarda lo demás y el hallazgo queda en la ficha hasta que la orden pase a
// Recibido. Datos Ficticio.

import (
	"net/http"
	"strings"
	"testing"

	assistantapp "sweveh/internal/assistant/app"
)

func TestFloor_FindingsWaitForReceived(t *testing.T) {
	adv := &fakeAdvisor{}
	e := floorEnv(t, adv)
	owner := e.register("3105550931", "Lucía Mora")
	cr := owner.must(http.StatusCreated, owner.do("POST", "/api/v1/customers", map[string]any{"celular": "3005550931", "nombre": "Andrés Vélez"}))
	owner.must(http.StatusCreated, owner.do("POST", "/api/v1/vehicles", map[string]any{"placa": "HLZ123", "clienteId": nested(cr.body, "cliente", "id"),
		"cilindraje": "1600", "energia": "gasolina", "aspiracion": "atmosferico", "transmision": "manual"}))
	orderID := owner.must(http.StatusCreated, owner.do("POST", "/api/v1/orders", map[string]any{"placa": "HLZ123", "sintoma": "Frena mal"})).str("id")
	if s := owner.must(http.StatusOK, owner.do("GET", "/api/v1/orders/"+orderID, nil)).str("estado"); s != "por_recibir" {
		t.Fatalf("la orden empieza Por recibir: %s", s)
	}

	var findingNote string
	adv.step = func(r assistantapp.ChatRequest) (assistantapp.ChatMessage, error) {
		res := floorResult(r)
		switch u := lastUser(r); {
		case strings.Contains(u, "pastillas"):
			switch {
			case !strings.Contains(res, "hallazgos"):
				return call("proponer_hallazgo", `{"sistema":"frenos","descripcion":"Pastillas delanteras gastadas","gravedad":"urgente"}`), nil
			case !strings.Contains(res, "tiene"):
				findingNote = res
				return call("proponer_recepcion", `{"kilometraje":30000,"lados":[{"foto":1,"lado":"frente"}]}`), nil
			}
			return say("Anoté las pastillas, 30000 km y la foto del frente."), nil
		case strings.Contains(u, "tanque"):
			switch {
			case !strings.Contains(res, "tiene"):
				return call("proponer_recepcion", `{"combustible":"1/2","lados":[{"foto":1,"lado":"trasera"},{"foto":2,"lado":"izquierdo"},{"foto":3,"lado":"derecho"}]}`), nil
			case !strings.Contains(res, "estado"):
				return call("mover_estado", `{"estado":"recibido"}`), nil
			}
			return say("Listo para pasar a Recibido."), nil
		}
		return say("¿En qué te ayudo?"), nil
	}

	// 1. Video de recepción: un hallazgo, el kilometraje y una foto. La orden sigue Por recibir.
	r := owner.must(http.StatusOK, owner.floorSend(map[string]string{"texto": "HLZ123 tiene las pastillas gastadas, 30000 km"}, 1))
	if chatOf(r)["ordenId"] != orderID {
		t.Fatalf("la placa lleva la conversación a su orden: %v", chatOf(r))
	}
	chatID, firstVersion := chatOf(r)["id"].(string), chatOf(r)["version"]
	if !strings.Contains(findingNote, "se guarda cuando la orden pase a Recibido") {
		t.Fatalf("la tool avisa que el hallazgo espera a Recibido: %q", findingNote)
	}
	if h, _ := proposalOf(r)["hallazgos"].([]any); len(h) != 1 {
		t.Fatalf("hallazgo propuesto: %v", proposalOf(r))
	}

	r = owner.must(http.StatusOK, owner.confirmFloor(chatID, firstVersion, "hallazgo-espera-01"))
	res := r.body["resultado"].(map[string]any)
	if held, _ := res["hallazgosPendientes"].([]any); len(held) != 1 || res["fotos"] != float64(1) || res["ordenEstado"] != "por_recibir" {
		t.Fatalf("se guarda lo demás y el hallazgo queda pendiente: %v", res)
	}
	if !hasReply(r, "El hallazgo queda en la ficha: se guarda cuando la orden pase a Recibido.") {
		t.Fatalf("mensaje: %v", chatOf(r)["mensajes"])
	}
	if h, _ := proposalOf(r)["hallazgos"].([]any); len(h) != 1 || chatOf(r)["pendiente"] != true {
		t.Fatalf("el hallazgo sigue en la ficha: %v", chatOf(r))
	}
	if cl := owner.must(http.StatusOK, owner.do("GET", "/api/v1/orders/"+orderID+"/checklist", nil)); cl.body["kilometraje"] != float64(30000) {
		t.Fatalf("el kilometraje quedó guardado: %v", cl.body)
	}
	if l := owner.must(http.StatusOK, owner.do("GET", "/api/v1/orders/"+orderID+"/findings", nil)).body["hallazgos"].([]any); len(l) != 0 {
		t.Fatalf("todavía no hay hallazgos en la orden: %v", l)
	}
	// La misma clave devuelve lo mismo.
	again := owner.must(http.StatusOK, owner.confirmFloor(chatID, firstVersion, "hallazgo-espera-01"))
	if held, _ := again.body["resultado"].(map[string]any)["hallazgosPendientes"].([]any); len(held) != 1 {
		t.Fatalf("idempotencia: %v", again.body)
	}

	// 2. Solo el hallazgo, con la orden Por recibir: la regla de la orden, sin guardar nada.
	if x := owner.confirmFloor(chatID, chatOf(r)["version"], "hallazgo-espera-02"); x.str("code") != "hallazgo_fuera_de_estado" {
		t.Fatalf("solo hallazgos en Por recibir: %d %v", x.status, x.body)
	}

	// 3. Se completa la recepción y se pasa a Recibido: el hallazgo se guarda, una sola vez.
	r = owner.must(http.StatusOK, owner.floorSend(map[string]string{"chatId": chatID, "texto": "tanque a la mitad, las otras fotos"}, 3))
	r = owner.must(http.StatusOK, owner.confirmFloor(chatID, chatOf(r)["version"], "hallazgo-espera-03"))
	res = r.body["resultado"].(map[string]any)
	if res["ordenEstado"] != "recibido" || res["hallazgosPendientes"] != nil {
		t.Fatalf("con Recibido se guarda el hallazgo: %v", res)
	}
	if h, _ := proposalOf(r)["hallazgos"].([]any); len(h) != 0 {
		t.Fatalf("la ficha queda limpia: %v", proposalOf(r))
	}
	l := owner.must(http.StatusOK, owner.do("GET", "/api/v1/orders/"+orderID+"/findings", nil)).body["hallazgos"].([]any)
	if len(l) != 1 || l[0].(map[string]any)["descripcion"] != "Pastillas delanteras gastadas" {
		t.Fatalf("hallazgo guardado: %v", l)
	}
}

// hasReply: algún mensaje del asistente contiene el texto (los mensajes de la misma hora pueden venir en otro orden).
func hasReply(r resp, text string) bool {
	msgs, _ := chatOf(r)["mensajes"].([]any)
	for _, m := range msgs {
		if s, _ := m.(map[string]any)["texto"].(string); strings.Contains(s, text) {
			return true
		}
	}
	return false
}
