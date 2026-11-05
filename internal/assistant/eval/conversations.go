package eval

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"sweveh/internal/assistant/app"
	"sweveh/internal/assistant/domain"
	"sweveh/internal/shared"
)

// Evaluación del asesor por conversación (ADR-0029): cada turno pasa por el mismo bucle de tools que en producción
// (app.Converse y app.NewTools) contra un taller de prueba en memoria. Mide si usa la tool que corresponde, si dice
// lo que debe, si respeta los guardias (cero precios que no salieron de una tool) y cuánto tarda.

//go:embed conversaciones.json
var defaultConversations []byte

type showItem struct {
	Ref   string `json:"referencia"`
	Name  string `json:"nombre"`
	Brand string `json:"marca"`
	Kind  string `json:"tipo"`
	Price int64  `json:"precio"`
	Stock int    `json:"existencia"`
	Photo bool   `json:"foto"`
}

// ConvTurn es un mensaje del cliente con lo que se espera de la respuesta.
type ConvTurn struct {
	Customer string   `json:"cliente"`
	AnyTool  []string `json:"usa_alguna"`  // debe usar al menos una
	AnySays  []string `json:"dice_alguno"` // la respuesta trae al menos uno (sin tildes ni mayúsculas)
	NotSays  []string `json:"no_dice"`
	// Books exige que en este turno quede agendada la cita en esa fecha y hora ("2026-11-08 09:00").
	Books string `json:"agenda"`
	// NoBook exige que en este turno no se agende nada (fuera de horario, sin confirmar).
	NoBook bool `json:"no_agenda"`
	// Name exige que el nombre del cliente quede guardado así.
	Name string `json:"guarda_nombre"`
}

type Conversation struct {
	Name     string     `json:"nombre"`
	Customer string     `json:"cliente_nombre"` // otro nombre para esta conversación ("Cliente de prueba" = sin nombre real)
	Turns    []ConvTurn `json:"turnos"`
}

// ConvSet es el juego de conversaciones con su taller de prueba.
type ConvSet struct {
	Workshop struct {
		Name     string               `json:"nombre"`
		City     string               `json:"ciudad"`
		Attends  string               `json:"atiende"`
		Hours    string               `json:"horario"`
		Days     map[string][2]string `json:"horario_dias"` // lun..dom → [abre, cierra]; sin el día = no abre
		Services []Item               `json:"servicios"`
		Showcase []showItem           `json:"vitrina"`
	} `json:"taller"`
	Customer struct {
		Name     string `json:"nombre"`
		Vehicles []struct {
			Plate string `json:"placa"`
			Desc  string `json:"descripcion"`
		} `json:"vehiculos"`
	} `json:"cliente"`
	Now           string         `json:"hoy"` // fecha y hora fijas de la evaluación, "2026-11-04 14:20" en Colombia
	Conversations []Conversation `json:"conversaciones"`
}

func DefaultConversations() (ConvSet, error) {
	var s ConvSet
	err := json.Unmarshal(defaultConversations, &s)
	return s, err
}

// TurnResult es lo que pasó en un turno.
type TurnResult struct {
	Conversation, Customer, Reply string
	Tools                         []string
	Problems                      []string
	Latency                       time.Duration
}

func (t TurnResult) OK() bool { return len(t.Problems) == 0 }

// ConvReport resume la corrida.
type ConvReport struct {
	Turns                     []TurnResult
	Passed, Conversations     int
	GuardFails, Errors, Unfin int
}

// MinConversations es cuántas conversaciones deben pasar completas para usar un modelo en la demo (ADR-0029).
const MinConversations = 16

func (r ConvReport) Passes() bool { return r.Passed >= MinConversations && r.GuardFails == 0 }

// RunConversations pasa cada conversación por el asesor.
func RunConversations(ctx context.Context, adv app.Advisor, set ConvSet) ConvReport {
	var rep ConvReport
	for _, c := range set.Conversations {
		rep.Conversations++
		ok := true
		tl := newFakeTaller(set)
		name := set.Customer.Name
		if c.Customer != "" {
			name = c.Customer
		}
		tl.name = name
		actx := app.AdvisorContext{Workshop: app.WorkshopProfile{Name: set.Workshop.Name, City: set.Workshop.City, Attends: set.Workshop.Attends,
			Hours: set.Workshop.Hours}, Open: true, NextOpening: "hoy", Customer: app.Customer{ID: tl.customer, Name: name}, Now: tl.now}
		for _, v := range set.Customer.Vehicles {
			actx.Vehicles = append(actx.Vehicles, app.Vehicle{Plate: v.Plate, Description: v.Desc})
		}
		var history []app.ChatMessage
		said := ""
		for _, turn := range c.Turns {
			said += " " + turn.Customer
			tl.booked = ""
			tools := app.NewTools(tl, tl.id, actx.Customer, app.Inbound{ConversationID: shared.NewID(), Text: turn.Customer}, said, tl.now)
			var used []string
			run := func(ctx context.Context, name, args string) string {
				used = append(used, name)
				return tools.Run(ctx, name, args)
			}
			msgs := append(append([]app.ChatMessage{}, history...), app.ChatMessage{Role: "user", Content: turn.Customer})
			start := time.Now()
			cctx, cancel := context.WithTimeout(ctx, 60*time.Second)
			reply, err := app.Respond(cctx, func(ctx context.Context, r app.ChatRequest) (app.ChatMessage, error) {
				res, err := adv.Chat(ctx, r)
				return res.Message, err
			}, app.ChatRequest{System: actx.System() + "\n\n" + tools.Catalog(cctx), Messages: msgs, Tools: app.AdvisorTools}, run, tools.Quoted())
			cancel()
			tr := TurnResult{Conversation: c.Name, Customer: turn.Customer, Reply: reply, Tools: used, Latency: time.Since(start)}
			switch {
			case errors.Is(err, domain.ErrReplyPrice) || errors.Is(err, domain.ErrReplyLink) || errors.Is(err, domain.ErrReplyLong):
				rep.GuardFails++
				tr.Problems = append(tr.Problems, "guardia: "+err.Error())
			case err != nil:
				rep.Errors++
				tr.Problems = append(tr.Problems, "error: "+err.Error())
			case reply == "":
				rep.Unfin++
				tr.Problems = append(tr.Problems, "no respondió en las rondas")
			default:
				tr.Problems = append(tr.Problems, check(turn, reply, used)...)
				tr.Problems = append(tr.Problems, checkBooking(turn, tl)...)
			}
			ok = ok && tr.OK()
			rep.Turns = append(rep.Turns, tr)
			history = append(msgs, app.ChatMessage{Role: "assistant", Content: reply})
		}
		if ok {
			rep.Passed++
		}
	}
	return rep
}

// checkBooking revisa lo que quedó agendado y el nombre guardado en el taller de prueba.
func checkBooking(t ConvTurn, tl *fakeTaller) []string {
	var out []string
	switch {
	case t.Books != "" && tl.booked != t.Books:
		out = append(out, fmt.Sprintf("agendó %q; se esperaba %q", tl.booked, t.Books))
	case t.NoBook && tl.booked != "":
		out = append(out, fmt.Sprintf("agendó %q sin deber", tl.booked))
	}
	if t.Name != "" && domain.Normalize(tl.name) != domain.Normalize(t.Name) {
		out = append(out, fmt.Sprintf("nombre guardado %q; se esperaba %q", tl.name, t.Name))
	}
	return out
}

func check(t ConvTurn, reply string, used []string) []string {
	var out []string
	if len(t.AnyTool) > 0 && !anyIn(used, t.AnyTool) {
		out = append(out, fmt.Sprintf("no usó %s", strings.Join(t.AnyTool, " ni ")))
	}
	r := domain.Normalize(reply)
	if len(t.AnySays) > 0 {
		found := false
		for _, s := range t.AnySays {
			found = found || strings.Contains(r, domain.Normalize(s)) || strings.Contains(reply, s)
		}
		if !found {
			out = append(out, fmt.Sprintf("no dijo %q", strings.Join(t.AnySays, " | ")))
		}
	}
	for _, s := range t.NotSays {
		if strings.Contains(r, domain.Normalize(s)) || strings.Contains(reply, s) {
			out = append(out, fmt.Sprintf("dijo %q", s))
		}
	}
	return out
}

func anyIn(used, want []string) bool {
	for _, u := range used {
		for _, w := range want {
			if u == w {
				return true
			}
		}
	}
	return false
}

func (r ConvReport) Print(w io.Writer) {
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
	_, _ = fmt.Fprintf(w, "\nConversaciones completas: %d/%d (mínimo %d). Precios no consultados: %d. Errores: %d. Sin cerrar: %d.\n",
		r.Passed, r.Conversations, MinConversations, r.GuardFails, r.Errors, r.Unfin)
}

// fakeTaller es el taller de prueba en memoria: solo lo que usan las tools del asesor.
type fakeTaller struct {
	app.Taller // lo demás no se usa en la evaluación
	id         shared.ID
	customer   shared.ID
	set        ConvSet
	next       int
	now        time.Time
	name       string // nombre del cliente (cambia si el asesor guarda uno real)
	booked     string // cita agendada en el turno, "2026-11-08 09:00"
}

func newFakeTaller(s ConvSet) *fakeTaller {
	now, err := time.ParseInLocation("2006-01-02 15:04", s.Now, domain.Bogota)
	if err != nil {
		now = time.Now()
	}
	return &fakeTaller{id: shared.NewID(), customer: shared.NewID(), set: s, next: 41, now: now}
}

var dayKeys = []string{"dom", "lun", "mar", "mie", "jue", "vie", "sab"}

func (f *fakeTaller) OpenAt(_ context.Context, _ shared.ID, at time.Time) (bool, string, error) {
	at = at.In(domain.Bogota)
	d, ok := f.set.Workshop.Days[dayKeys[at.Weekday()]]
	if !ok {
		return false, "", nil
	}
	hm := at.Format("15:04")
	return hm >= d[0] && hm < d[1], d[0] + " a " + d[1], nil
}

func (f *fakeTaller) SetCustomerName(_ context.Context, _, _ shared.ID, name string) (bool, error) {
	if !domain.IsPlaceholder(f.name) {
		return false, nil
	}
	f.name = name
	return true, nil
}

func (f *fakeTaller) Catalog(context.Context, shared.ID) ([]domain.Priced, []domain.Priced, error) {
	var sv []domain.Priced
	for _, i := range f.set.Workshop.Services {
		sv = append(sv, domain.Priced{Name: i.Name, Price: i.Price, Kind: "servicio", Vehicle: i.Vehicle})
	}
	return sv, nil, nil
}

func (f *fakeTaller) Showcase(_ context.Context, _ shared.ID, q string) ([]app.Product, error) {
	var out []app.Product
	q = domain.Normalize(q)
	for _, p := range f.set.Workshop.Showcase {
		if q != "" && !strings.Contains(domain.Normalize(p.Name+" "+p.Brand), q) {
			continue
		}
		out = append(out, app.Product{ID: shared.NewID(), Ref: p.Ref, Name: p.Name, Brand: p.Brand, Kind: p.Kind, Price: p.Price,
			Stock: p.Stock, HasPhoto: p.Photo})
	}
	return out, nil
}

func (f *fakeTaller) Profile(context.Context, shared.ID) (app.WorkshopProfile, error) {
	return app.WorkshopProfile{Name: f.set.Workshop.Name, City: f.set.Workshop.City, Attends: f.set.Workshop.Attends, Hours: f.set.Workshop.Hours}, nil
}

func (f *fakeTaller) Workshop(context.Context, shared.ID) (string, bool, string, error) {
	return f.set.Workshop.Name, true, "hoy", nil
}

func (f *fakeTaller) OpenOrders(context.Context, shared.ID, shared.ID) ([]app.Order, error) {
	return []app.Order{{ID: shared.NewID(), Code: "OS-0040", Plate: "GNO667", Vehicle: "Yamaha NMAX", State: "en_diagnostico"}}, nil
}

func (f *fakeTaller) PlateOf(_ context.Context, _, _ shared.ID, plate string) (app.PlateInfo, error) {
	switch plate {
	case "KMX123":
		return app.PlateInfo{Owner: "ajena"}, nil
	case "GNO667":
		o := app.Order{Code: "OS-0040", Plate: plate, State: "en_diagnostico"}
		return app.PlateInfo{Owner: "propia", Open: &o, Code: o.Code}, nil
	case "PQR45E":
		o := app.Order{Code: "OS-0039", Plate: plate, State: "por_recibir"}
		return app.PlateInfo{Owner: "propia", Open: &o, Code: o.Code}, nil
	}
	return app.PlateInfo{Owner: "nueva"}, nil
}

func (f *fakeTaller) CreateOrderFromPlate(_ context.Context, _, _ shared.ID, plate, _ string, when *time.Time) (app.PlateInfo, error) {
	switch plate {
	case "KMX123":
		return app.PlateInfo{Owner: "ajena"}, nil
	case "GNO667":
		o := app.Order{Code: "OS-0040", Plate: plate, State: "en_diagnostico"}
		return app.PlateInfo{Owner: "propia", Open: &o, Code: o.Code}, nil
	case "PQR45E": // moto de la clienta ya agendada y sin llegar: se reagenda
		o := app.Order{Code: "OS-0039", Plate: plate, State: "por_recibir"}
		f.mark(when)
		return app.PlateInfo{Owner: "propia", Open: &o, Code: o.Code, Rescheduled: when != nil}, nil
	}
	f.next++
	f.mark(when)
	return app.PlateInfo{Owner: "nueva", Code: fmt.Sprintf("OS-%04d", f.next)}, nil
}

func (f *fakeTaller) mark(when *time.Time) {
	if when != nil {
		f.booked = when.In(domain.Bogota).Format("2006-01-02 15:04")
	}
}

func (f *fakeTaller) LastClientPhoto(context.Context, shared.ID, shared.ID) (string, error) {
	return "", nil
}

func (f *fakeTaller) SetMode(context.Context, shared.ID, shared.ID, string, bool) error { return nil }
