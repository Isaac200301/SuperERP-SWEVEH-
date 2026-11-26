package server_test

// Otro vehículo en el chat de una orden (prueba de Pablo, 26-nov): grabó en el asistente de la moto OS-0041 un video de
// un Toyota y el asistente dio vueltas ("te lo dejo en la ficha" sin ficha; "confirmar" no hacía nada). Ahora el código
// arma una tarjeta: los datos y las fotos de ese vehículo van a un borrador aparte (propuesta.otraOrden), nunca a la
// moto, y la persona decide abrirlos en una conversación nueva o descartarlos. Datos Ficticio.

import (
	"net/http"
	"strings"
	"sync"
	"testing"

	assistantapp "sweveh/internal/assistant/app"
)

func otherOf(r resp) map[string]any { m, _ := proposalOf(r)["otraOrden"].(map[string]any); return m }

func TestFloorOtherOrder_VideoCardThenOpenWithData(t *testing.T) {
	adv := &fakeAdvisor{}
	e := floorEnv(t, adv)
	owner := e.register("3105550930", "Lina Mora")
	_, _, orderID := seedOrder(t, owner, "3005550930", "XVN43F")
	before := countOrders(t, owner)
	adv.step = func(r assistantapp.ChatRequest) (assistantapp.ChatMessage, error) {
		res := floorResult(r)
		switch u := lastUser(r); {
		case strings.Contains(u, "video") && !strings.Contains(res, "otro vehículo"):
			return call("buscar_vehiculo", `{"placa":"TPR-418","de_foto":true}`), nil
		case strings.Contains(u, "Pedro") && !strings.Contains(res, "tarjeta"):
			return call("proponer_otra_orden", `{"placa":"TPR418","tipo":"carro","marca":"Toyota","linea":"Prado","sintoma":"facelift","celular":"3000000761","nombre":"Pedro Ríos"}`), nil
		}
		return say("En los cuadros se ve la placa TPR-418: es otro vehículo. Toca Abrir orden nueva con estos datos."), nil
	}

	// 1. El video del Toyota en la orden de la moto: las fotos van al borrador del Toyota, la moto queda igual.
	r := owner.must(http.StatusOK, owner.videoSend(orderID, "video-otra-0001", 3, nil))
	o := otherOf(r)
	fotos, _ := o["fotos"].([]any)
	if nested(o, "placa", "valor") != "TPR418" || len(fotos) != 3 || proposalOf(r)["fotos"] != nil || chatOf(r)["pendiente"] == true {
		t.Fatalf("borrador aparte con las 3 fotos y la moto sin nada pendiente: %v / %v", o, proposalOf(r))
	}
	if !strings.HasPrefix(lastReply(r), "En los cuadros se ve la placa TPR-418") {
		t.Fatalf("el texto del modelo no se toca: %q", lastReply(r))
	}
	chatID := chatOf(r)["id"].(string)

	// 2. "confirmar" sin nada pendiente lo contesta el código, sin IA, y apunta a la tarjeta.
	calls := len(adv.reqs)
	r = owner.must(http.StatusOK, owner.floorSend(map[string]string{"chatId": chatID, "texto": "confirmar"}, 0))
	if !strings.Contains(lastReply(r), "Abrir orden nueva con estos datos") || len(adv.reqs) != calls || countOrders(t, owner) != before {
		t.Fatalf("confirmar sin pendiente: %q (IA %d → %d)", lastReply(r), calls, len(adv.reqs))
	}

	// 3. Lo que la persona dice del Toyota va a la tarjeta, no a la moto.
	r = owner.must(http.StatusOK, owner.floorSend(map[string]string{"chatId": chatID, "texto": "es el Prado de Pedro Ríos, 3000000761, le van a hacer un facelift"}, 0))
	o = otherOf(r)
	if nested(o, "cliente", "nombre", "valor") != "Pedro Ríos" || nested(o, "sintoma", "valor") != "facelift" || nested(o, "tipo") != "carro" ||
		proposalOf(r)["sintoma"] != nil || proposalOf(r)["cliente"] != nil {
		t.Fatalf("datos del otro vehículo: %v / %v", o, proposalOf(r))
	}
	if f, _ := chatOf(r)["otraFalta"].([]any); len(f) != 0 {
		t.Fatalf("ya no falta nada para la orden nueva: %v", f)
	}

	// 4. Abrir con estos datos: conversación nueva con la ficha del Toyota y sus fotos; la de la moto queda sin tarjeta.
	ver := chatOf(r)["version"]
	open := func(c *client, v any) resp {
		return c.do("POST", "/api/v1/assistant/floor/chats/"+chatID+"/other-order", map[string]any{"version": v}, "Idempotency-Key", "abrir-otra-0001")
	}
	n := owner.must(http.StatusOK, open(owner, ver))
	np := proposalOf(n)
	nf, _ := np["fotos"].([]any)
	if chatOf(n)["id"] == chatID || chatOf(n)["ordenId"] != nil || np["nuevaOrden"] != true || nested(np, "placa", "valor") != "TPR418" ||
		nested(np, "cliente", "celular", "valor") != "3000000761" || len(nf) != 3 || chatOf(n)["pendiente"] != true || np["otraOrden"] != nil {
		t.Fatalf("conversación nueva: %v", chatOf(n))
	}
	if !strings.Contains(lastReply(n), "Traje lo que me contaste de TPR418") || !strings.Contains(lastReply(n), "Nada está guardado") {
		t.Fatalf("primer mensaje: %q", lastReply(n))
	}
	if countOrders(t, owner) != before {
		t.Fatal("abrir no crea ninguna orden: se crea al tocar Guardar allá")
	}
	// Las fotos se ven en la conversación nueva.
	msgs, _ := chatOf(n)["mensajes"].([]any)
	adj, _ := msgs[0].(map[string]any)["adjuntos"].([]any)
	if s, _, _ := owner.rawGet(adj[0].(map[string]any)["url"].(string)); len(adj) != 3 || s != http.StatusOK {
		t.Fatalf("fotos en la conversación nueva: %d %v", s, adj)
	}
	// Doble toque (misma clave): la misma conversación, aunque la versión ya cambió.
	if again := owner.must(http.StatusOK, open(owner, ver)); chatOf(again)["id"] != chatOf(n)["id"] {
		t.Fatalf("doble toque: %v", chatOf(again)["id"])
	}
	old := owner.must(http.StatusOK, owner.do("GET", "/api/v1/assistant/floor/chats/"+chatID, nil))
	if otherOf(old) != nil {
		t.Fatalf("la tarjeta sale del chat de la moto: %v", proposalOf(old))
	}
	// Otra clave, ya sin tarjeta: 409. Otro taller: 404.
	if s := owner.do("POST", "/api/v1/assistant/floor/chats/"+chatID+"/other-order", map[string]any{"version": chatOf(old)["version"]},
		"Idempotency-Key", "abrir-otra-0002").status; s != http.StatusConflict {
		t.Fatalf("sin tarjeta: %d", s)
	}
	other := e.register("3105550939", "Otro Taller")
	if s := open(other, ver).status; s != http.StatusNotFound {
		t.Fatalf("otro taller: %d", s)
	}

	// 5. Guardar en la conversación nueva crea la orden del Toyota con sus fotos.
	nid := chatOf(n)["id"].(string)
	g := owner.must(http.StatusOK, owner.confirmFloor(nid, chatOf(n)["version"], "guardar-otra-0001"))
	if g.body["resultado"].(map[string]any)["ordenNueva"] != true || countOrders(t, owner) != before+1 {
		t.Fatalf("guardar: %v", g.body["resultado"])
	}
}

func TestFloorOtherOrder_TypedPlateDismissAndNoFalsePlates(t *testing.T) {
	adv := &fakeAdvisor{}
	e := floorEnv(t, adv)
	owner := e.register("3105550931", "Iván Ríos")
	_, _, orderID := seedOrder(t, owner, "3005550931", "KLM482")
	adv.step = func(r assistantapp.ChatRequest) (assistantapp.ChatMessage, error) {
		if strings.Contains(lastUser(r), "mañana") {
			return say("Listo. ¿El ABC123 también viene?"), nil // una placa que nadie dijo ni se ve: no cuenta
		}
		return say("Entendido."), nil // el modelo no llama ninguna tool: el código igual busca la placa escrita
	}

	// "a las 10" no es una placa: no hay tarjeta.
	r := owner.must(http.StatusOK, owner.floorSend(map[string]string{"ordenId": orderID, "texto": "llegó a las 10 con el cliente"}, 0))
	if otherOf(r) != nil {
		t.Fatalf("a las 10 no es LAS10: %v", otherOf(r))
	}
	chatID := chatOf(r)["id"].(string)
	// Placa nombrada por el modelo que nadie dijo: no cuenta.
	r = owner.must(http.StatusOK, owner.floorSend(map[string]string{"chatId": chatID, "texto": "lo entregamos mañana"}, 0))
	if otherOf(r) != nil {
		t.Fatalf("una placa que solo dijo el modelo no arma tarjeta: %v", otherOf(r))
	}
	// La persona escribe otra placa: tarjeta, aunque el modelo no llame la tool. La orden no cambia.
	r = owner.must(http.StatusOK, owner.floorSend(map[string]string{"chatId": chatID, "texto": "también llegó el TPR418 de Pedro"}, 0))
	if nested(otherOf(r), "placa", "valor") != "TPR418" || nested(proposalOf(r), "placa", "valor") != "KLM482" || chatOf(r)["pendiente"] == true {
		t.Fatalf("placa escrita: %v", proposalOf(r))
	}
	// Sigue en los mensajes siguientes (no es de un solo turno) hasta que la persona decida.
	r = owner.must(http.StatusOK, owner.floorSend(map[string]string{"chatId": chatID, "texto": "el KLM482 tiene 45 mil km"}, 0))
	if otherOf(r) == nil {
		t.Fatal("la tarjeta sigue hasta que la persona decida")
	}
	// "No, sigo aquí" (era un error de dedo): PATCH sin otraOrden la quita.
	prop := map[string]any{}
	for k, v := range proposalOf(r) {
		if k != "otraOrden" {
			prop[k] = v
		}
	}
	ed := owner.must(http.StatusOK, owner.do("PATCH", "/api/v1/assistant/floor/chats/"+chatID, map[string]any{"version": chatOf(r)["version"], "propuesta": prop}))
	if otherOf(ed) != nil {
		t.Fatalf("descartar: %v", proposalOf(ed))
	}
}

// Una placa leída de un cuadro a un carácter de la de la orden es la misma mal leída: no hay tarjeta y las fotos
// siguen en la orden.
func TestFloorOtherOrder_MisreadPlateStaysInOrder(t *testing.T) {
	adv := &fakeAdvisor{}
	e := floorEnv(t, adv)
	owner := e.register("3105550932", "Rosa Peña")
	_, _, orderID := seedOrder(t, owner, "3005550932", "XVN43F")
	adv.step = func(r assistantapp.ChatRequest) (assistantapp.ChatMessage, error) {
		if !strings.Contains(floorResult(r), "mal leída") {
			return call("buscar_vehiculo", `{"placa":"XVN43E","de_foto":true}`), nil
		}
		return say("Veo la XVN43E en el cuadro."), nil
	}
	r := owner.must(http.StatusOK, owner.videoSend(orderID, "video-mala-0001", 2, nil))
	if fotos, _ := proposalOf(r)["fotos"].([]any); otherOf(r) != nil || len(fotos) != 2 {
		t.Fatalf("mala lectura: tarjeta %v, fotos %d", otherOf(r), len(fotos))
	}
}

// Abrir con estos datos a la vez (R-07, 8.8): con la misma clave, una sola conversación; la misma clave desde otro chat
// no devuelve la conversación ajena.
func TestFloorOtherOrder_OpenConcurrentAndKeyPerChat(t *testing.T) {
	adv := &fakeAdvisor{}
	e := floorEnv(t, adv)
	owner := e.register("3105550933", "Hugo Lara")
	_, _, orderA := seedOrder(t, owner, "3005550933", "KLM482")
	_, _, orderB := seedOrder(t, owner, "3005550934", "KLM483")
	adv.step = func(assistantapp.ChatRequest) (assistantapp.ChatMessage, error) { return say("Entendido."), nil }
	card := func(order string) (string, any) {
		r := owner.must(http.StatusOK, owner.floorSend(map[string]string{"ordenId": order, "texto": "también llegó el TPR418"}, 0))
		return chatOf(r)["id"].(string), chatOf(r)["version"]
	}
	chatA, verA := card(orderA)
	const n = 4
	ids := make(chan any, n)
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := owner.do("POST", "/api/v1/assistant/floor/chats/"+chatA+"/other-order", map[string]any{"version": verA}, "Idempotency-Key", "abrir-misma-0001")
			if r.status != http.StatusOK {
				ids <- r.status
				return
			}
			ids <- chatOf(r)["id"]
		}()
	}
	wg.Wait()
	close(ids)
	var first any
	for id := range ids {
		if first == nil {
			first = id
		}
		if _, ok := id.(string); !ok || id != first {
			t.Fatalf("a la vez con la misma clave: %v y %v", first, id)
		}
	}
	chatB, verB := card(orderB)
	r := owner.must(http.StatusOK, owner.do("POST", "/api/v1/assistant/floor/chats/"+chatB+"/other-order", map[string]any{"version": verB}, "Idempotency-Key", "abrir-misma-0001"))
	if chatOf(r)["id"] == first {
		t.Fatal("la misma clave en otro chat no devuelve la conversación ajena")
	}
}
