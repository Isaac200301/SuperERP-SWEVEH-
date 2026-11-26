package server_test

// Recepción por video (R9, V-12, ADR-0042, HU-108) por HTTP, con una IA falsa: llegan el audio y los cuadros (nunca
// el video), la propuesta queda "del video" en el chat de la orden con la guía de lados, nada se guarda hasta
// Confirmar, repetir con la misma clave no repite nada y un taller no recibe videos para la orden de otro. Datos
// Ficticio.

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"
	"testing"

	assistantapp "sweveh/internal/assistant/app"
)

// videoSend manda un video de recepción: n cuadros, sin audio, y la clave dada. extra agrega partes (por ejemplo
// "video", que debe rechazarse).
func (c *client) videoSend(orderID, key string, frames int, extra map[string]string) resp {
	c.e.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("segundos", "62")
	for k, v := range extra {
		fw, _ := mw.CreateFormFile(k, "archivo")
		_, _ = fw.Write([]byte(v))
	}
	for i := 0; i < frames; i++ {
		fw, _ := mw.CreateFormFile("cuadros", "cuadro.jpg")
		_, _ = fw.Write(jpg)
	}
	_ = mw.Close()
	req, _ := http.NewRequest("POST", c.e.srv.URL+"/api/v1/orders/"+orderID+"/reception/video", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-Requested-With", "sweveh")
	req.Header.Set("Origin", c.e.srv.URL)
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	res, err := c.http.Do(req)
	if err != nil {
		c.e.t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	out := resp{status: res.StatusCode, header: res.Header, body: map[string]any{}}
	raw, _ := io.ReadAll(res.Body)
	_ = json.Unmarshal(raw, &out.body)
	return out
}

// videoAdvisor: con el video, el modelo dice qué ve en cada cuadro (placa, tablero, frente e izquierdo) y lee el
// kilometraje del tablero. Cuenta las llamadas con video.
type videoAdvisor struct {
	fakeAdvisor
	mu    sync.Mutex
	calls int
}

func newVideoAdvisor() *videoAdvisor {
	v := &videoAdvisor{}
	v.step = func(r assistantapp.ChatRequest) (assistantapp.ChatMessage, error) {
		if !strings.Contains(r.System, "RECEPCIÓN POR VIDEO") {
			return say("¿En qué te ayudo?"), nil
		}
		if !strings.Contains(floorResult(r), "tiene") {
			v.mu.Lock()
			v.calls++
			v.mu.Unlock()
			return call("proponer_recepcion", `{"kilometraje":45000,"combustible":"1/2","de_foto":true,"notas":["Rayón en la puerta del conductor"],
				"lados":[{"foto":1,"lado":"placa"},{"foto":2,"lado":"tablero"},{"foto":3,"lado":"frente"},{"foto":4,"lado":"izquierdo"}]}`), nil
		}
		return say("Anoté 45000 km, medio tanque y un rayón en la puerta."), nil
	}
	return v
}

func guideOf(r resp) map[string]bool {
	out := map[string]bool{}
	g, _ := chatOf(r)["progreso"].(map[string]any)["guia"].([]any)
	for _, s := range g {
		m := s.(map[string]any)
		out[m["clave"].(string)] = m["visto"].(bool)
	}
	return out
}

func TestFloorVideo_ReceptionProposalAndConfirm(t *testing.T) {
	adv := newVideoAdvisor()
	e := floorEnv(t, adv)
	owner := e.register("3105550920", "Rosa Pérez")
	_, _, orderID := seedOrder(t, owner, "3005550920", "VID123")

	r := owner.must(http.StatusOK, owner.videoSend(orderID, "video-00000001", 8, nil))
	chat, p := chatOf(r), proposalOf(r)
	if chat["ordenId"] != orderID || chat["disponible"] != true {
		t.Fatalf("el video sigue en el chat de la orden: %v", chat)
	}
	fotos, _ := p["fotos"].([]any)
	if len(fotos) != 8 || nested(p, "kilometraje", "origen") != "video" || nested(p, "combustible", "valor") != "1/2" {
		t.Fatalf("los 8 cuadros y lo leído van a la propuesta, con origen video: %v", p)
	}
	for _, f := range fotos {
		if f.(map[string]any)["origen"] != "video" {
			t.Fatalf("cada cuadro sale del video: %v", f)
		}
	}
	g := guideOf(r)
	if !g["placa"] || !g["tablero"] || !g["frente"] || !g["izquierdo"] || g["trasera"] || g["derecho"] {
		t.Fatalf("guía de lados: %v", g)
	}
	if !strings.Contains(lastReply(r), "No vi la parte trasera ni el lado derecho") {
		t.Fatalf("el código dice qué no se vio: %q", lastReply(r))
	}
	msgs := chat["mensajes"].([]any)
	adj := msgs[len(msgs)-2].(map[string]any)["adjuntos"].([]any)
	if first := adj[0].(map[string]any); first["tipo"] != "video" || first["segundos"] != float64(62) || len(adj) != 9 {
		t.Fatalf("el mensaje lleva el video (sin guardarlo) y sus 8 cuadros: %v", adj)
	}
	// Nada se guardó todavía (R-08): la orden sigue sin fotos ni kilometraje.
	cl := owner.must(http.StatusOK, owner.do("GET", "/api/v1/orders/"+orderID+"/checklist", nil))
	if cl.body["kilometraje"] != nil {
		t.Fatalf("antes de confirmar no hay recepción guardada: %v", cl.body)
	}

	// Repetir con la misma clave no vuelve a llamar a la IA ni agrega cuadros.
	again := owner.must(http.StatusOK, owner.videoSend(orderID, "video-00000001", 8, nil))
	if adv.calls != 1 || chatOf(again)["version"] != chat["version"] || len(proposalOf(again)["fotos"].([]any)) != 8 {
		t.Fatalf("idempotente: llamadas %d, versión %v → %v", adv.calls, chat["version"], chatOf(again)["version"])
	}

	// Confirmar guarda los cuadros como fotos de la orden, con su lado.
	c := owner.must(http.StatusOK, owner.confirmFloor(chat["id"].(string), chat["version"], "conf-video-001"))
	if nested(c.body, "resultado", "fotos") != float64(8) {
		t.Fatalf("confirmar: %v", c.body)
	}
	cl = owner.must(http.StatusOK, owner.do("GET", "/api/v1/orders/"+orderID+"/checklist", nil))
	if cl.body["kilometraje"] != float64(45000) {
		t.Fatalf("la recepción quedó guardada: %v", cl.body)
	}
	if g := guideOf(owner.must(http.StatusOK, owner.do("GET", "/api/v1/orders/"+orderID+"/assistant", nil))); !g["frente"] || !g["izquierdo"] {
		t.Fatalf("lo guardado marca la guía: %v", g)
	}
}

func TestFloorVideo_RulesAndIsolation(t *testing.T) {
	adv := newVideoAdvisor()
	e := floorEnv(t, adv)
	a := e.register("3105550921", "Taller A")
	b := e.register("3105550922", "Taller B")
	_, _, orderA := seedOrder(t, a, "3005550921", "VID124")

	checks := []struct {
		name string
		r    resp
		code string
	}{
		{"sin clave", a.videoSend(orderA, "", 2, nil), "clave_requerida"},
		{"sin cuadros", a.videoSend(orderA, "video-00000002", 0, nil), "cuadros_invalidos"},
		{"más de 10 cuadros", a.videoSend(orderA, "video-00000003", 11, nil), "cuadros_invalidos"},
		{"el video completo", a.videoSend(orderA, "video-00000004", 2, map[string]string{"video": "no se manda"}), "video_completo"},
		{"un cuadro que no es imagen", a.videoSend(orderA, "video-00000005", 0, map[string]string{"cuadros": "texto"}), "cuadros_invalidos"},
	}
	for _, c := range checks {
		if c.r.status != http.StatusBadRequest || c.r.str("code") != c.code {
			t.Fatalf("%s: %d %v", c.name, c.r.status, c.r.body)
		}
	}
	// Otro taller no puede mandar un video a esta orden.
	if s := b.videoSend(orderA, "video-00000006", 2, nil).status; s != http.StatusNotFound {
		t.Fatalf("otro taller: %d", s)
	}
	if s := e.client().videoSend(orderA, "video-00000007", 2, nil).status; s != http.StatusUnauthorized {
		t.Fatalf("sin sesión: %d", s)
	}
	// Una orden ya recibida no se recibe otra vez por video.
	a.must(http.StatusOK, a.do("POST", "/api/v1/orders/"+orderA+"/cancel", map[string]any{"motivo": "El cliente no volvió", "version": r2v(a, orderA)}))
	if r := a.videoSend(orderA, "video-00000008", 2, nil); r.status != http.StatusConflict || r.str("code") != "orden_ya_recibida" {
		t.Fatalf("orden cancelada: %d %v", r.status, r.body)
	}
}

// Sin IA el video no se pierde: los cuadros quedan en la propuesta para confirmarlos como fotos (RNF-13).
func TestFloorVideo_WithoutAIKeepsFrames(t *testing.T) {
	e := floorEnv(t, nil)
	owner := e.register("3105550923", "Sara León")
	_, _, orderID := seedOrder(t, owner, "3005550923", "VID125")
	r := owner.must(http.StatusOK, owner.videoSend(orderID, "video-00000009", 3, nil))
	if chatOf(r)["disponible"] != false || lastReply(r) != assistantapp.FloorUnavailable || len(proposalOf(r)["fotos"].([]any)) != 3 {
		t.Fatalf("sin IA: %v", r.body)
	}
}

// Cuatro envíos a la vez con la misma clave (la red reintenta): un solo mensaje de video en la orden (R-07).
func TestFloorVideo_ConcurrentSameKey(t *testing.T) {
	adv := newVideoAdvisor()
	e := floorEnv(t, adv)
	owner := e.register("3105550924", "Nora Gil")
	_, _, orderID := seedOrder(t, owner, "3005550924", "VID126")
	var wg sync.WaitGroup
	status := make([]int, 4)
	for i := range status {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			status[i] = owner.videoSend(orderID, "video-concurrente", 4, nil).status
		}(i)
	}
	wg.Wait()
	for _, s := range status {
		if s != http.StatusOK {
			t.Fatalf("estados: %v", status)
		}
	}
	r := owner.must(http.StatusOK, owner.do("GET", "/api/v1/orders/"+orderID+"/assistant", nil))
	videos := 0
	for _, m := range chatOf(r)["mensajes"].([]any) {
		for _, a := range m.(map[string]any)["adjuntos"].([]any) {
			if a.(map[string]any)["tipo"] == "video" {
				videos++
			}
		}
	}
	if videos != 1 || len(proposalOf(r)["fotos"].([]any)) != 4 {
		t.Fatalf("un solo video: %d, fotos %v", videos, proposalOf(r)["fotos"])
	}
}

// La misma clave en otra orden del taller no devuelve la conversación de la primera: 409 clave_usada (R-07).
func TestFloorVideo_KeyReusedInAnotherOrder(t *testing.T) {
	adv := newVideoAdvisor()
	e := floorEnv(t, adv)
	owner := e.register("3105550925", "Iván Mora")
	_, _, o1 := seedOrder(t, owner, "3005550925", "VID127")
	_, _, o2 := seedOrder(t, owner, "3005550926", "VID128")
	owner.must(http.StatusOK, owner.videoSend(o1, "video-reusada-01", 2, nil))
	if r := owner.videoSend(o2, "video-reusada-01", 2, nil); r.status != http.StatusConflict || r.str("code") != "clave_usada" {
		t.Fatalf("clave en otra orden: %d %v", r.status, r.body)
	}
}

// Moto (HU-18): la guía pide también el tablero y el combustible puede ser "no aplica".
func TestFloorVideo_Moto(t *testing.T) {
	adv := &fakeAdvisor{}
	adv.step = func(r assistantapp.ChatRequest) (assistantapp.ChatMessage, error) {
		if strings.Contains(r.System, "RECEPCIÓN POR VIDEO") && !strings.Contains(floorResult(r), "tiene") {
			return call("proponer_recepcion", `{"combustible":"no_aplica","lados":[{"foto":1,"lado":"placa"},{"foto":2,"lado":"tablero"},{"foto":3,"lado":"frente"},{"foto":4,"lado":"trasera"},{"foto":5,"lado":"izquierdo"}]}`), nil
		}
		return say("Listo."), nil
	}
	e := floorEnv(t, adv)
	owner := e.register("3105550927", "Sebastián Arango")
	cl := owner.must(http.StatusCreated, owner.do("POST", "/api/v1/customers", map[string]any{"celular": "3005550927", "nombre": "Cliente moto"}))
	owner.must(http.StatusCreated, owner.do("POST", "/api/v1/vehicles", map[string]any{"placa": "XVN43F", "tipo": "moto",
		"clienteId": nested(cl.body, "cliente", "id"), "marca": "Honda", "linea": "CB 190R", "cilindraje": "190", "turbo": false}))
	o := owner.must(http.StatusCreated, owner.do("POST", "/api/v1/orders", map[string]any{"placa": "XVN43F", "sintoma": "Revisión"}))
	r := owner.must(http.StatusOK, owner.videoSend(o.str("id"), "video-moto-0001", 5, nil))
	p := proposalOf(r)
	if nested(p, "combustible", "valor") != "no_aplica" || p["tipo"] != "moto" {
		t.Fatalf("moto: %v", p)
	}
	lados := map[string]bool{}
	for _, f := range p["fotos"].([]any) {
		if l, ok := f.(map[string]any)["lado"].(string); ok {
			lados[l] = true
		}
	}
	if !lados["tablero"] || lados["placa"] { // en moto el tablero es lado con foto; la placa nunca
		t.Fatalf("lados de la moto: %v", lados)
	}
	if !strings.Contains(lastReply(r), "No vi el lado derecho.") {
		t.Fatalf("falta el derecho: %q", lastReply(r))
	}
}
