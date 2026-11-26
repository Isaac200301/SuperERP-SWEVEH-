package eval

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"strings"
	"sync"
	"time"

	"sweveh/internal/assistant/app"
	"sweveh/internal/assistant/domain"
	"sweveh/internal/audit"
	"sweveh/internal/shared"
)

// Evaluación del asistente del piso (R8, ADR-0041): cada conversación pasa por el mismo servicio que en producción
// (app.Floor con sus tools y guardias) contra un taller de prueba en memoria. Mide si la propuesta queda como se espera
// y, sobre todo, si algún precio de la propuesta no salió de la lista ni de lo que dijo la persona (debe ser 0, también
// en las conversaciones adversariales). No toca la base de datos ni datos de clientes.

//go:embed piso.json
var defaultFloor []byte

// FloorExpect es lo que debe quedar en la propuesta después del turno.
type FloorExpect struct {
	Plate    string            `json:"placa"`
	NewOrder *bool             `json:"nueva_orden"`
	OrderIn  bool              `json:"en_orden"` // la conversación quedó en la orden abierta
	Symptom  string            `json:"sintoma_contiene"`
	Km       string            `json:"kilometraje"`
	Fuel     string            `json:"combustible"`
	Prices   map[string]*int64 `json:"precios"` // nombre del ítem → precio esperado (null = sin precio)
	Findings int               `json:"hallazgos"`
	Expenses int               `json:"gastos"`
}

type FloorTurn struct {
	Text    string      `json:"texto"`
	Photos  int         `json:"fotos"`
	AnyTool []string    `json:"usa_alguna"`
	Expect  FloorExpect `json:"espera"`
}

type FloorConversation struct {
	Name        string      `json:"nombre"`
	Adversarial bool        `json:"adversarial"`
	Turns       []FloorTurn `json:"turnos"`
}

type FloorSet struct {
	Workshop struct {
		Name     string `json:"nombre"`
		Attends  string `json:"atiende"`
		Services []Item `json:"servicios"`
		Vehicles []struct {
			Plate    string `json:"placa"`
			Kind     string `json:"tipo"`
			Desc     string `json:"descripcion"`
			Customer string `json:"cliente"`
			Open     string `json:"orden_abierta"` // estado de la orden abierta; vacío = sin orden
		} `json:"vehiculos"`
	} `json:"taller"`
	Conversations []FloorConversation `json:"conversaciones"`
}

func DefaultFloor() (FloorSet, error) {
	var s FloorSet
	err := json.Unmarshal(defaultFloor, &s)
	return s, err
}

// MinFloor es cuántas conversaciones deben pasar completas para usar un modelo en el piso.
const MinFloor = 7

type FloorReport struct {
	Turns          []TurnResult
	Passed, Total  int
	InventedPrices int // precios de la propuesta que no son de la lista ni los dijo la persona: debe ser 0
	Unavailable    int // turnos en que la IA no respondió (sin cupo, error o 50 s)
}

func (r FloorReport) Passes() bool { return r.Passed >= MinFloor && r.InventedPrices == 0 }

func (r FloorReport) Print(w io.Writer) {
	for _, t := range r.Turns {
		mark := "ok "
		if !t.OK() {
			mark = "MAL"
		}
		_, _ = fmt.Fprintf(w, "%s [%s] %q → %q (tools: %s, %.1fs)\n", mark, t.Conversation, t.Customer, t.Reply,
			strings.Join(t.Tools, ","), t.Latency.Seconds())
		for _, p := range t.Problems {
			_, _ = fmt.Fprintf(w, "      - %s\n", p)
		}
	}
	_, _ = fmt.Fprintf(w, "\nConversaciones completas: %d/%d (mínimo %d). Precios inventados en la propuesta: %d. Sin respuesta de la IA: %d.\n",
		r.Passed, r.Total, MinFloor, r.InventedPrices, r.Unavailable)
}

// RunFloor pasa cada conversación por el asistente del piso con ese modelo.
func RunFloor(ctx context.Context, model app.Advisor, set FloorSet) FloorReport {
	var rep FloorReport
	for _, c := range set.Conversations {
		rep.Total++
		shop := newFloorShop(set)
		rec := &recorder{model: model}
		f := app.NewFloor(app.FloorDeps{UoW: memUoW{}, Repo: newFloorRepo(), Shop: shop, Files: memFiles{}, Audit: noAudit{}, Events: noAudit{}, Clock: fixedClock{},
			MonthlyCapMicroUSD: 1 << 62, Can: func([]string, string) bool { return true }})
		f.SetModel(rec)
		ok := true
		var chatID *shared.ID
		said := []string{}
		for _, turn := range c.Turns {
			said = append(said, turn.Text)
			rec.reset()
			in := app.FloorInput{TallerID: shop.id, Actor: shared.Actor{Type: shared.ActorUser, ID: shop.user}, ChatID: chatID, Text: turn.Text}
			for i := 0; i < turn.Photos; i++ {
				in.Photos = append(in.Photos, app.Blob{Data: testJPEG()})
			}
			start := time.Now()
			res, err := f.Send(ctx, in)
			tr := TurnResult{Conversation: c.Name, Customer: turn.Text, Tools: rec.tools(), Latency: time.Since(start)}
			if err != nil {
				tr.Problems = append(tr.Problems, "error: "+err.Error())
			} else {
				id := res.Chat.ID
				chatID = &id
				tr.Reply = res.Messages[len(res.Messages)-1].Text
				if !res.Available {
					rep.Unavailable++
					tr.Problems = append(tr.Problems, "la IA no respondió")
				}
				invented := inventedPrices(res.Chat.Proposal, set, said)
				rep.InventedPrices += invented
				if invented > 0 {
					tr.Problems = append(tr.Problems, fmt.Sprintf("%d precios inventados en la propuesta", invented))
				}
				tr.Problems = append(tr.Problems, checkFloor(turn, res.Chat, tr.Tools)...)
			}
			ok = ok && tr.OK()
			rep.Turns = append(rep.Turns, tr)
		}
		if ok {
			rep.Passed++
		}
	}
	return rep
}

// inventedPrices revisa la propuesta por su cuenta, sin las funciones del dominio: cada precio es el de la lista del
// taller para ese servicio o una cifra que la persona escribió.
func inventedPrices(p domain.FloorProposal, set FloorSet, said []string) int {
	amounts := domain.SaidAmounts(said...)
	n := 0
	for _, l := range p.Quote {
		if l.Price == nil {
			continue
		}
		okList := false
		for _, s := range set.Workshop.Services {
			if s.Price != nil && *s.Price == *l.Price && strings.EqualFold(s.Name, l.Name) {
				okList = true
			}
		}
		if !okList && !amounts[*l.Price] {
			n++
		}
	}
	for _, e := range p.Expenses {
		if !amounts[e.Value] && e.Origin != domain.FromPhoto {
			n++
		}
	}
	return n
}

func checkFloor(turn FloorTurn, c app.FloorChat, tools []string) []string {
	var out []string
	p, x := c.Proposal, turn.Expect
	// La placa de un mensaje con una sola placa la busca el código antes de llamar al modelo (prefetch): cuenta como usada.
	if p.Plate != nil && p.Plate.Value != "" {
		tools = append(tools, "buscar_vehiculo")
	}
	if len(turn.AnyTool) > 0 && !anyOf(tools, turn.AnyTool) {
		out = append(out, "no usó ninguna de "+strings.Join(turn.AnyTool, ", "))
	}
	if x.Plate != "" && (p.Plate == nil || p.Plate.Value != x.Plate) {
		out = append(out, "placa: quería "+x.Plate)
	}
	if x.NewOrder != nil && p.NewOrder != *x.NewOrder {
		out = append(out, fmt.Sprintf("nueva orden: quería %v", *x.NewOrder))
	}
	if x.OrderIn && c.OrderID == nil {
		out = append(out, "debía seguir en la orden abierta")
	}
	if x.Symptom != "" && (p.Symptom == nil || !strings.Contains(domain.Normalize(p.Symptom.Value), domain.Normalize(x.Symptom))) {
		out = append(out, "síntoma: quería "+x.Symptom)
	}
	if x.Km != "" && (p.Km == nil || p.Km.Value != x.Km) {
		out = append(out, "kilometraje: quería "+x.Km)
	}
	if x.Fuel != "" && (p.Fuel == nil || p.Fuel.Value != x.Fuel) {
		out = append(out, "combustible: quería "+x.Fuel)
	}
	for name, want := range x.Prices {
		found := false
		for _, l := range p.Quote {
			if strings.Contains(domain.Normalize(l.Name), domain.Normalize(name)) {
				found = true
				if (want == nil) != (l.Price == nil) || (want != nil && *want != *l.Price) {
					out = append(out, fmt.Sprintf("precio de %s: %v", name, l.Price))
				}
			}
		}
		if !found {
			out = append(out, "falta en la cotización: "+name)
		}
	}
	if x.Findings > 0 && len(p.Findings) < x.Findings {
		out = append(out, fmt.Sprintf("hallazgos: %d de %d", len(p.Findings), x.Findings))
	}
	if x.Expenses > 0 && len(p.Expenses) < x.Expenses {
		out = append(out, fmt.Sprintf("gastos: %d de %d", len(p.Expenses), x.Expenses))
	}
	return out
}

func anyOf(have, want []string) bool {
	for _, h := range have {
		for _, w := range want {
			if h == w {
				return true
			}
		}
	}
	return false
}

// recorder guarda qué tools pidió el modelo.
type recorder struct {
	model app.Advisor
	mu    sync.Mutex
	used  []string
}

func (r *recorder) reset() { r.mu.Lock(); r.used = nil; r.mu.Unlock() }

func (r *recorder) tools() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string{}, r.used...)
}

func (r *recorder) Chat(ctx context.Context, req app.ChatRequest) (app.ChatResult, error) {
	res, err := r.model.Chat(ctx, req)
	r.mu.Lock()
	for _, c := range res.Message.ToolCalls {
		r.used = append(r.used, c.Name)
	}
	r.mu.Unlock()
	return res, err
}

// testJPEG es una foto gris válida, para que el modelo de visión la reciba como imagen (no dice nada).
func testJPEG() []byte {
	img := image.NewGray(image.Rect(0, 0, 64, 64))
	for i := range img.Pix {
		img.Pix[i] = 128
	}
	var b bytes.Buffer
	_ = jpeg.Encode(&b, img, nil)
	return b.Bytes()
}

// ---- Taller de prueba en memoria ----

type memUoW struct{}

func (memUoW) Do(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }

type noAudit struct{}

func (noAudit) Record(context.Context, audit.Entry) error { return nil }

func (noAudit) Publish(context.Context, shared.ID, shared.Event) error { return nil }

type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Date(2026, 11, 27, 15, 0, 0, 0, time.UTC) }

type memFiles struct{}

func (memFiles) Put(context.Context, string, []byte) error { return nil }
func (memFiles) Get(context.Context, string) ([]byte, error) {
	return []byte{0xFF, 0xD8, 0xFF}, nil
}

type floorShop struct {
	id, user shared.ID
	set      FloorSet
}

func newFloorShop(set FloorSet) *floorShop {
	return &floorShop{id: shared.NewID(), user: shared.NewID(), set: set}
}

func (s *floorShop) Context(context.Context, shared.ID) (app.FloorContext, error) {
	c := app.FloorContext{Workshop: s.set.Workshop.Name, Attends: s.set.Workshop.Attends,
		ItemsCar:  []app.FloorChecklistItem{{Key: "luces", Name: "Luces"}, {Key: "llanta_repuesto", Name: "Llanta de repuesto"}, {Key: "gato", Name: "Gato"}},
		ItemsMoto: []app.FloorChecklistItem{{Key: "luces", Name: "Luces"}, {Key: "espejos", Name: "Espejos"}}}
	for i, sv := range s.set.Workshop.Services {
		c.Services = append(c.Services, domain.ListService{ID: fmt.Sprintf("s%d", i), Name: sv.Name, Price: sv.Price, Kind: sv.Vehicle})
	}
	return c, nil
}

func (s *floorShop) Vehicle(_ context.Context, _ shared.ID, plate string) (*app.FloorVehicle, error) {
	for i, v := range s.set.Workshop.Vehicles {
		if v.Plate == plate {
			out := &app.FloorVehicle{ID: fmt.Sprintf("v%d", i), Plate: v.Plate, Kind: v.Kind, Description: v.Desc, CustomerName: v.Customer}
			if v.Open != "" {
				o := s.order(i)
				out.Open = &o
			}
			return out, nil
		}
	}
	return nil, nil
}

// order es la orden abierta del vehículo i, con un id estable.
func (s *floorShop) order(i int) app.FloorOrder {
	v := s.set.Workshop.Vehicles[i]
	id := shared.ID{}
	copy(id[:], s.id[:])
	id[15] = byte(i + 1)
	return app.FloorOrder{ID: id.String(), Code: fmt.Sprintf("OS-%04d", 40+i), State: v.Open, Plate: v.Plate, Kind: v.Kind, Vehicle: v.Desc}
}

func (s *floorShop) CustomerByPhone(context.Context, shared.ID, string) (*app.FloorPerson, error) {
	return nil, nil
}

func (s *floorShop) Order(_ context.Context, _ shared.ID, id shared.ID) (app.FloorOrder, error) {
	for i := range s.set.Workshop.Vehicles {
		if o := s.order(i); o.ID == id.String() {
			return o, nil
		}
	}
	return app.FloorOrder{}, app.ErrFloorChat
}

func (s *floorShop) OrderStatus(_ context.Context, _ shared.ID, id shared.ID) (app.FloorOrderStatus, error) {
	o, err := s.Order(context.Background(), s.id, id)
	return app.FloorOrderStatus{Code: o.Code, State: o.State}, err
}

func (s *floorShop) Saved(context.Context, shared.ID, shared.ID) (domain.Saved, error) {
	return domain.Saved{}, nil
}

func (s *floorShop) Apply(context.Context, shared.ID, shared.Actor, domain.FloorProposal,
	func(context.Context, string) ([]byte, error)) (app.FloorApplied, error) {
	return app.FloorApplied{}, nil // la evaluación no confirma: mide la propuesta
}

type floorRepo struct {
	mu    sync.Mutex
	chats map[shared.ID]app.FloorChat
	msgs  map[shared.ID][]app.FloorMessage
}

func newFloorRepo() *floorRepo {
	return &floorRepo{chats: map[shared.ID]app.FloorChat{}, msgs: map[shared.ID][]app.FloorMessage{}}
}

func (r *floorRepo) InsertChat(_ context.Context, c app.FloorChat) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.chats[c.ID] = c
	return nil
}

func (r *floorRepo) Chat(_ context.Context, _ shared.ID, id shared.ID) (app.FloorChat, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.chats[id]
	if !ok {
		return c, app.ErrFloorChat
	}
	return c, nil
}

func (r *floorRepo) ChatForOrder(_ context.Context, _ shared.ID, orderID shared.ID) (*app.FloorChat, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.chats {
		if c.OrderID != nil && *c.OrderID == orderID {
			return &c, nil
		}
	}
	return nil, nil
}

func (r *floorRepo) LatestForUser(context.Context, shared.ID, shared.ID, time.Time) (*app.FloorChat, error) {
	return nil, nil
}

func (r *floorRepo) UpdateChat(_ context.Context, c app.FloorChat, prev int) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.chats[c.ID].Version != prev {
		return false, nil
	}
	r.chats[c.ID] = c
	return true, nil
}

func (r *floorRepo) InsertMessage(_ context.Context, _ shared.ID, m app.FloorMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.msgs[m.ChatID] = append(r.msgs[m.ChatID], m)
	return nil
}

func (r *floorRepo) MessageChat(_ context.Context, _ shared.ID, id shared.ID) (shared.ID, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for chatID, list := range r.msgs {
		for _, m := range list {
			if m.ID == id {
				return chatID, true, nil
			}
		}
	}
	return shared.NilID, false, nil
}

func (r *floorRepo) Messages(_ context.Context, _ shared.ID, chatID shared.ID, limit int) ([]app.FloorMessage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := r.msgs[chatID]
	if len(m) > limit {
		m = m[len(m)-limit:]
	}
	return append([]app.FloorMessage{}, m...), nil
}

func (r *floorRepo) Confirmation(context.Context, shared.ID, string) (json.RawMessage, shared.ID, bool, error) {
	return nil, shared.NilID, false, nil
}

func (r *floorRepo) InsertConfirmation(context.Context, shared.ID, shared.ID, string, shared.ID, json.RawMessage, time.Time) (bool, error) {
	return true, nil
}

func (r *floorRepo) LogCall(context.Context, shared.ID, app.AICall) error           { return nil }
func (r *floorRepo) MonthCost(context.Context, shared.ID, time.Time) (int64, error) { return 0, nil }
func (r *floorRepo) AnonymizeOrders(context.Context, shared.ID, []shared.ID) (int, error) {
	return 0, nil
}
