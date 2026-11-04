// Package analytics calcula los 6 KPIs del dueño (HU-68 a HU-73), su explicación (HU-75) y
// "Hoy tienes pendiente" (HU-76). Solo lee por las fachadas de otros módulos; el costo de IA no
// aparece aquí (HU-74 es del Administrador del ERP, DR-10).
package analytics

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"

	"sweveh/internal/inventory"
	"sweveh/internal/orders"
	"sweveh/internal/platform/authn"
	"sweveh/internal/platform/httpx"
	"sweveh/internal/platform/reqctx"
	"sweveh/internal/quotes"
	"sweveh/internal/shared"
)

type UnitOfWork interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

// Extra son lecturas de módulos que no tienen fachada en este paquete (mensajería, agente, inventario).
type Extra interface {
	ResponseMinutes(ctx context.Context, tallerID shared.ID, since time.Time) ([]ResponseTime, error)
	PendingDrafts(ctx context.Context, tallerID shared.ID) (int, error)
	LowStock(ctx context.Context, tallerID shared.ID) ([]inventory.Part, error)
	CustomerNames(ctx context.Context, tallerID shared.ID, ids []shared.ID) (map[shared.ID]string, error)
	// FailedNotices son los avisos automáticos que WhatsApp no entregó desde since (HU-46).
	FailedNotices(ctx context.Context, tallerID shared.ID, since time.Time) ([]FailedNotice, error)
	// HumanRequests son las conversaciones en las que el cliente pidió una persona y nadie la ha cerrado (HU-59).
	HumanRequests(ctx context.Context, tallerID shared.ID) ([]HumanRequest, error)
}

// ResponseTime es cuánto tardó la primera respuesta a un mensaje del cliente; Auto = respuesta automática de AU-05.
type ResponseTime struct {
	Minutes float64
	Auto    bool
}

// HumanRequest es una conversación esperando a una persona del taller.
type HumanRequest struct {
	ConversationID shared.ID
	Phone          string
}

// FailedNotice es un aviso que no salió: qué automatización, de qué orden y por qué.
type FailedNotice struct {
	Name    string
	OrderID *shared.ID
	Detail  string
	At      time.Time
}

type Handler struct {
	uow    UnitOfWork
	orders orders.Orders
	quotes quotes.Quotes
	extra  Extra
	guard  *authn.Guard
	log    *slog.Logger
	now    func() time.Time
}

func NewHandler(u UnitOfWork, o orders.Orders, q quotes.Quotes, e Extra, g *authn.Guard, log *slog.Logger, now func() time.Time) *Handler {
	return &Handler{uow: u, orders: o, quotes: q, extra: e, guard: g, log: log, now: now}
}

func (h *Handler) Routes(r chi.Router) {
	r.With(h.guard.Require("kpi:read")).Get("/dashboard", httpx.JSON(h.log, h.dashboard))
	r.With(h.guard.Require("order:read")).Get("/today", httpx.JSON(h.log, h.today))
}

// KPI es un número con su explicación en lenguaje del taller (HU-75, DR-11).
type KPI struct {
	Code    string   `json:"codigo"`
	Label   string   `json:"titulo"`
	Value   *float64 `json:"valor"`
	Display string   `json:"texto"`
	Detail  string   `json:"detalle"`
	Meaning string   `json:"queSignifica"`
	Example bool     `json:"ejemplo"` // calculado con los datos de ejemplo del registro (HU-71)
}

func f(v float64) *float64 { return &v }

func median(v []float64) *float64 {
	if len(v) == 0 {
		return nil
	}
	s := append([]float64{}, v...)
	sort.Float64s(s)
	m := s[len(s)/2]
	if len(s)%2 == 0 {
		m = (s[len(s)/2-1] + s[len(s)/2]) / 2
	}
	return &m
}

func money(v int64) string {
	s := fmt.Sprintf("%d", v)
	out := ""
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out += "."
		}
		out += string(c)
	}
	return "$" + out
}

// dashboard son los 6 números del taller (HU-68 a HU-73). ?tipo=carro|moto los separa por tipo de vehículo (HU-68 y
// HU-70, Should). Mientras el taller no tenga órdenes propias, se calculan con las de ejemplo y van marcados "ejemplo"
// (HU-71).
func (h *Handler) dashboard(r *http.Request, p reqctx.Principal) (int, any, error) {
	now := h.now()
	kind := r.URL.Query().Get("tipo")
	if kind != "carro" && kind != "moto" {
		kind = ""
	}
	var kpis []KPI
	var byState map[string]int
	example := false
	err := h.uow.Do(r.Context(), func(ctx context.Context) error {
		st := h.orders.Stats()
		real, err := st.HasReal(ctx, p.TallerID)
		if err != nil {
			return err
		}
		example = !real
		flt := orders.Filter{Kind: kind, Example: example}
		if byState, err = st.StateCounts(ctx, p.TallerID, flt); err != nil {
			return err
		}
		open := 0
		for _, n := range byState {
			open += n
		}
		kpis = append(kpis, KPI{Code: "abiertas", Label: "Órdenes abiertas", Value: f(float64(open)), Display: fmt.Sprintf("%d", open),
			Meaning: "Vehículos que están en el taller o por llegar: todo lo que no se ha entregado ni cancelado."})

		pend, err := h.quotes.Pending(ctx, p.TallerID)
		if err != nil {
			return err
		}
		k := KPI{Code: "aprobaciones", Label: "Aprobaciones pendientes", Value: f(float64(len(pend))), Display: fmt.Sprintf("%d", len(pend)),
			Meaning: "Cotizaciones que el cliente todavía no aprueba, y cuántas horas lleva esperando la más antigua. Mientras esperan, el vehículo ocupa espacio y no se trabaja."}
		if len(pend) > 0 { // HU-69: "1 · 26 h"
			hours := int(now.Sub(pend[0].SentAt).Hours())
			k.Display, k.Detail = fmt.Sprintf("%d · %d h", len(pend), hours), fmt.Sprintf("la más antigua lleva %d h esperando", hours)
		}
		kpis = append(kpis, k)

		cycles, err := st.Cycles(ctx, p.TallerID, now.AddDate(0, 0, -30), flt)
		if err != nil {
			return err
		}
		k = KPI{Code: "tiempo", Label: "Tiempo promedio por orden (30 días)", Display: "sin datos",
			Meaning: "Cuánto pasa desde que se abre la orden hasta que se entrega el vehículo, en las entregas del último mes: en horas y en días hábiles (lunes a viernes)."}
		if len(cycles) > 0 { // HU-70: horas y días hábiles
			var hours, bdays float64
			for _, c := range cycles {
				hours += c.Closed.Sub(c.Opened).Hours()
				bdays += BusinessDays(c.Opened, c.Closed)
			}
			n := float64(len(cycles))
			k.Value = f(hours / n)
			k.Display = fmt.Sprintf("%.0f h · %.1f días hábiles", hours/n, bdays/n)
			k.Detail = fmt.Sprintf("%d órdenes entregadas", len(cycles))
		}
		kpis = append(kpis, k)

		monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		ids, err := st.DeliveredSince(ctx, p.TallerID, monthStart, flt)
		if err != nil {
			return err
		}
		totals, err := h.quotes.ApprovedTotals(ctx, p.TallerID, ids)
		if err != nil {
			return err
		}
		var sum int64
		for _, t := range totals {
			sum += t
		}
		k = KPI{Code: "ticket", Label: "Ticket promedio", Display: "sin datos",
			Meaning: "Lo que paga en promedio cada cliente por orden entregada este mes (lo aprobado en la cotización)."}
		if len(totals) > 0 {
			avg := sum / int64(len(totals))
			k.Value, k.Display, k.Detail = f(float64(avg)), money(avg), fmt.Sprintf("%d órdenes con cotización aprobada", len(totals))
		}
		kpis = append(kpis, k)

		back, total, err := st.Returning(ctx, p.TallerID, now.AddDate(-1, 0, 0), flt)
		if err != nil {
			return err
		}
		k = KPI{Code: "vuelven", Label: "Clientes que vuelven", Display: "sin datos",
			Meaning: "De los vehículos atendidos en 12 meses, cuántos volvieron al menos otra vez."}
		if total > 0 {
			pct := float64(back) * 100 / float64(total)
			k.Value, k.Display, k.Detail = f(pct), fmt.Sprintf("%.0f %%", pct), fmt.Sprintf("%d de %d vehículos", back, total)
		}
		kpis = append(kpis, k)

		times, err := h.extra.ResponseMinutes(ctx, p.TallerID, monthStart)
		if err != nil {
			return err
		}
		k = KPI{Code: "respuesta", Label: "Minutos hasta responder un WhatsApp", Display: "sin datos",
			Meaning: "La mediana de minutos entre que el cliente escribe y alguien del taller (o el asesor) le contesta, este mes. Las respuestas automáticas del estado del vehículo (AU-05) salen al instante y se cuentan aparte para no maquillar el número."}
		var people, auto []float64
		for _, t := range times {
			if t.Auto {
				auto = append(auto, t.Minutes)
			} else {
				people = append(people, t.Minutes)
			}
		}
		if m := median(people); m != nil {
			k.Value, k.Display = m, fmt.Sprintf("%.0f min", *m)
		}
		if len(auto) > 0 {
			k.Detail = fmt.Sprintf("aparte, %d respuestas automáticas del estado (AU-05)", len(auto))
		}
		kpis = append(kpis, k)
		return nil
	})
	for i := range kpis {
		kpis[i].Example = example && kpis[i].Code != "aprobaciones" && kpis[i].Code != "respuesta"
	}
	if byState == nil {
		byState = map[string]int{}
	}
	return http.StatusOK, map[string]any{"kpis": kpis, "porEstado": byState, "ejemplo": example, "tipo": kind}, err
}

// BusinessDays son los días hábiles (lunes a viernes) entre a y b, con fracción: un fin de semana en medio no cuenta.
func BusinessDays(a, b time.Time) float64 {
	if !b.After(a) {
		return 0
	}
	var total time.Duration
	for t := a; t.Before(b); {
		next := time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, t.Location())
		if next.After(b) {
			next = b
		}
		if wd := t.Weekday(); wd != time.Saturday && wd != time.Sunday {
			total += next.Sub(t)
		}
		t = next
	}
	return total.Hours() / 24
}

// Pending es un pendiente del día (HU-76).
type Pending struct {
	Kind    string  `json:"tipo"`
	Title   string  `json:"titulo"`
	Detail  string  `json:"detalle"`
	OrderID *string `json:"ordenId,omitempty"`
	ConvID  *string `json:"conversacionId,omitempty"`
	Tone    string  `json:"tono"` // err | warn | info
}

func (h *Handler) today(r *http.Request, p reqctx.Principal) (int, any, error) {
	now := h.now()
	out := []Pending{}
	money := h.guard.Can(p, "money:read")
	err := h.uow.Do(r.Context(), func(ctx context.Context) error {
		ready, err := h.orders.InState(ctx, p.TallerID, "listo")
		if err != nil {
			return err
		}
		ids := []shared.ID{}
		for _, o := range ready {
			ids = append(ids, o.CustomerID)
		}
		names, err := h.extra.CustomerNames(ctx, p.TallerID, ids)
		if err != nil {
			return err
		}
		for _, o := range ready {
			d := int(now.Sub(o.StateSince).Hours() / 24)
			if d < 3 || o.Example {
				continue
			}
			id := o.ID.String()
			out = append(out, Pending{Kind: "sin_recoger", Title: fmt.Sprintf("%s %s sin recoger", o.Vehicle, o.Plate),
				Detail: fmt.Sprintf("Listo hace %d días · %s", d, names[o.CustomerID]), OrderID: &id, Tone: "err"})
		}
		if money {
			pend, err := h.quotes.Pending(ctx, p.TallerID)
			if err != nil {
				return err
			}
			for _, q := range pend {
				o, err := h.orders.Load(ctx, p.TallerID, q.OrderID)
				if err != nil {
					return err
				}
				id := o.ID.String()
				out = append(out, Pending{Kind: "aprobacion", Title: fmt.Sprintf("%s no ha aprobado", o.Plate),
					Detail: fmt.Sprintf("%s · cotización enviada hace %d h", o.Code, int(now.Sub(q.SentAt).Hours())), OrderID: &id, Tone: "warn"})
			}
			n, err := h.extra.PendingDrafts(ctx, p.TallerID)
			if err != nil {
				return err
			}
			if n > 0 {
				out = append(out, Pending{Kind: "borradores", Title: fmt.Sprintf("%d borradores por revisar", n), Detail: "Respuestas del agente esperando tu revisión", Tone: "info"})
			}
		}
		observed, err := h.orders.ObservedReceipts(ctx, p.TallerID)
		if err != nil {
			return err
		}
		for _, o := range observed {
			id := o.ID.String()
			out = append(out, Pending{Kind: "constancia_observada", Title: fmt.Sprintf("%s: el cliente dice que algo no cuadra", o.Plate),
				Detail: fmt.Sprintf("%s · revisa el recibo con el cliente", o.Code), OrderID: &id, Tone: "err"})
		}
		humans, err := h.extra.HumanRequests(ctx, p.TallerID)
		if err != nil {
			return err
		}
		for _, hr := range humans {
			id := hr.ConversationID.String()
			out = append(out, Pending{Kind: "pide_humano", Title: fmt.Sprintf("%s pidió hablar con una persona", shared.NationalNumber(hr.Phone)),
				Detail: "El asistente no le responde hasta que lo devuelvas.", ConvID: &id, Tone: "err"})
		}
		failed, err := h.extra.FailedNotices(ctx, p.TallerID, now.AddDate(0, 0, -7))
		if err != nil {
			return err
		}
		for _, f := range failed {
			pend := Pending{Kind: "aviso_fallido", Title: fmt.Sprintf("No salió el aviso «%s»", f.Name),
				Detail: "WhatsApp no lo entregó. Escríbele al cliente por otro medio.", Tone: "err"}
			if f.OrderID != nil {
				id := f.OrderID.String()
				pend.OrderID = &id
			}
			out = append(out, pend)
		}
		low, err := h.extra.LowStock(ctx, p.TallerID)
		if err != nil {
			return err
		}
		for _, l := range low {
			word := "en el mínimo"
			if l.Stock == 0 {
				word = "agotado"
			}
			out = append(out, Pending{Kind: "inventario", Title: fmt.Sprintf("%s %s", l.Name, word), Detail: fmt.Sprintf("%s · quedan %d", l.Ref, l.Stock), Tone: "warn"})
		}
		return nil
	})
	return http.StatusOK, map[string]any{"pendientes": out}, err
}
