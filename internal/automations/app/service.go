// Package app contiene el catálogo con interruptores y los disparadores de las automatizaciones.
package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"sweveh/internal/audit"
	"sweveh/internal/automations/domain"
	"sweveh/internal/shared"
)

type Rule struct {
	Code   string
	Active bool
	Text   *string
}

type Repository interface {
	Rules(ctx context.Context, tallerID shared.ID) (map[string]Rule, error)
	SaveRule(ctx context.Context, tallerID shared.ID, r Rule) error
	// RecordRun guarda la ejecución; false si ya existía (misma automatización y misma clave).
	RecordRun(ctx context.Context, tallerID shared.ID, code, key string, orderID, customerID *shared.ID, result, detail string, msgID *shared.ID) (bool, error)
	WeekCounts(ctx context.Context, tallerID shared.ID, since time.Time) (map[string]int, error)
	Runs(ctx context.Context, tallerID shared.ID, limit int) ([]Run, error)
	HasRun(ctx context.Context, tallerID shared.ID, code, key string) (bool, error)
	// FailRunByMessage marca fallida la ejecución que envió ese mensaje (HU-46).
	FailRunByMessage(ctx context.Context, tallerID, msgID shared.ID, detail string) error
}

type Run struct {
	Code      string
	Key       string
	OrderID   *shared.ID
	Result    string
	Detail    *string
	CreatedAt time.Time
}

// OrderInfo son los datos reales de la orden que llenan los avisos.
type OrderInfo struct {
	ID          shared.ID
	Code        string
	State       string
	Plate       string
	Vehicle     string // marca y línea ("Yamaha NMAX")
	Kind        string // "carro" | "moto"
	CustomerID  shared.ID
	PromisedAt  *time.Time
	Example     bool
	Km          *int
	Exceptions  []string
	Accessories []string
	StateSince  time.Time
}

type Orders interface {
	Info(ctx context.Context, tallerID, orderID shared.ID) (OrderInfo, error)
	InState(ctx context.Context, tallerID shared.ID, state string) ([]OrderInfo, error)
}

type Customer struct {
	Name    string
	Phone   string
	Consent string
}

type Customers interface {
	Get(ctx context.Context, tallerID, id shared.ID) (Customer, error)
}

type Workshop struct {
	Name     string
	Address  string                // opcional (HU-44)
	Schedule map[string]*[2]string // día → [abre, cierra]
}

type Workshops interface {
	Get(ctx context.Context, tallerID shared.ID) (Workshop, error)
}

type QuoteInfo struct {
	Total int64
	Lines []string
}

type Quotes interface {
	ForOrder(ctx context.Context, tallerID, orderID shared.ID) (*QuoteInfo, error)
}

type Button struct{ ID, Title string }

type Outgoing struct {
	TallerID   shared.ID
	Phone      string
	Text       string
	Buttons    []Button
	OrderID    *shared.ID
	Automation string
}

// Messenger envía por WhatsApp. Devuelve (id, ventanaCerrada, error).
type Messenger interface {
	Send(ctx context.Context, o Outgoing) (*shared.ID, bool, error)
}

type UnitOfWork interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}
type AuditLog interface {
	Record(ctx context.Context, e audit.Entry) error
}
type Clock interface{ Now() time.Time }

type Service struct {
	uow       UnitOfWork
	repo      Repository
	orders    Orders
	customers Customers
	workshops Workshops
	quotes    Quotes
	msg       Messenger
	audit     AuditLog
	clock     Clock
	loc       *time.Location
}

func NewService(uow UnitOfWork, r Repository, o Orders, c Customers, w Workshops, q Quotes, m Messenger, a AuditLog, clk Clock) *Service {
	loc, err := time.LoadLocation("America/Bogota")
	if err != nil {
		loc = time.FixedZone("COT", -5*3600) // Colombia no tiene horario de verano
	}
	return &Service{uow: uow, repo: r, orders: o, customers: c, workshops: w, quotes: q, msg: m, audit: a, clock: clk, loc: loc}
}

// Card es una tarjeta del catálogo con su estado en el taller (HU-40, HU-46).
type Card struct {
	domain.Definition
	Active    bool
	Text      string
	WeekCount int
}

func (s *Service) active(rules map[string]Rule, d domain.Definition) bool {
	if d.ComingSoon {
		return false
	}
	if r, ok := rules[d.Code]; ok {
		return r.Active
	}
	return d.DefaultOn
}

func (s *Service) text(rules map[string]Rule, d domain.Definition) string {
	if r, ok := rules[d.Code]; ok && r.Text != nil {
		return *r.Text
	}
	return d.DefaultText
}

func (s *Service) Cards(ctx context.Context, tallerID shared.ID) ([]Card, error) {
	var out []Card
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		rules, err := s.repo.Rules(ctx, tallerID)
		if err != nil {
			return err
		}
		counts, err := s.repo.WeekCounts(ctx, tallerID, s.clock.Now().Add(-7*24*time.Hour))
		if err != nil {
			return err
		}
		for _, d := range domain.Catalog {
			out = append(out, Card{Definition: d, Active: s.active(rules, d), Text: s.text(rules, d), WeekCount: counts[d.Code]})
		}
		return nil
	})
	return out, err
}

// Toggle enciende o apaga una automatización (HU-41).
func (s *Service) Toggle(ctx context.Context, tallerID shared.ID, actor shared.Actor, code string, on bool) error {
	d, err := domain.Find(code)
	if err != nil {
		return err
	}
	if d.ComingSoon {
		return domain.ErrComingSoon
	}
	return s.uow.Do(ctx, func(ctx context.Context) error {
		if code == "AU-04" && on { // HU-06: el horario se pide justo cuando hace falta, al encender el agente
			w, err := s.workshops.Get(ctx, tallerID)
			if err != nil {
				return err
			}
			if len(w.Schedule) == 0 {
				return domain.ErrScheduleNeeded
			}
		}
		rules, err := s.repo.Rules(ctx, tallerID)
		if err != nil {
			return err
		}
		r := rules[code]
		before := s.active(rules, d)
		r.Code, r.Active = code, on
		if err := s.repo.SaveRule(ctx, tallerID, r); err != nil {
			return err
		}
		return s.audit.Record(ctx, audit.Entry{TallerID: tallerID, Actor: actor, Action: "automatizacion.interruptor", Entity: "taller",
			EntityID: tallerID, Before: map[string]any{code: before}, After: map[string]any{code: on}, OccurredAt: s.clock.Now()})
	})
}

// EditText cambia el texto de un aviso (HU-42). nil vuelve al texto por defecto.
func (s *Service) EditText(ctx context.Context, tallerID shared.ID, actor shared.Actor, code string, text *string) error {
	d, err := domain.Find(code)
	if err != nil {
		return err
	}
	if !d.Editable {
		return domain.ErrNotEditable
	}
	if text != nil {
		t := strings.TrimSpace(*text)
		if err := d.ValidateText(t); err != nil {
			return err
		}
		text = &t
	}
	return s.uow.Do(ctx, func(ctx context.Context) error {
		rules, err := s.repo.Rules(ctx, tallerID)
		if err != nil {
			return err
		}
		r, ok := rules[code]
		if !ok {
			r = Rule{Code: code, Active: d.DefaultOn}
		}
		r.Text = text
		if err := s.repo.SaveRule(ctx, tallerID, r); err != nil {
			return err
		}
		return s.audit.Record(ctx, audit.Entry{TallerID: tallerID, Actor: actor, Action: "automatizacion.texto", Entity: "taller",
			EntityID: tallerID, After: map[string]any{"codigo": code, "texto": text}, OccurredAt: s.clock.Now()})
	})
}

func (s *Service) Runs(ctx context.Context, tallerID shared.ID) ([]Run, error) {
	var out []Run
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.repo.Runs(ctx, tallerID, 100)
		return err
	})
	return out, err
}

// OnDeliveryFailed: WhatsApp no entregó el mensaje de una automatización; la ejecución queda fallida para que
// el dueño lo vea (HU-46). Idempotente: marcarla dos veces deja lo mismo.
func (s *Service) OnDeliveryFailed(ctx context.Context, tallerID, msgID shared.ID, detail string) error {
	return s.uow.Do(ctx, func(ctx context.Context) error {
		return s.repo.FailRunByMessage(ctx, tallerID, msgID, "WhatsApp no lo entregó: "+detail)
	})
}

var stateWords = map[string]string{"por_recibir": "por recibir", "recibido": "recibido", "en_diagnostico": "en diagnóstico",
	"esperando_aprobacion": "esperando tu aprobación", "en_reparacion": "en reparación", "esperando_repuesto": "esperando un repuesto", "control_calidad": "en control de calidad", "listo": "listo", "entregado": "entregado", "cancelada": "cancelada"}

// Money da el formato COP sin decimales: $1.250.000 (docs/08 §4).
func Money(v int64) string {
	s := fmt.Sprintf("%d", v)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	return "$" + b.String()
}

var (
	days     = []string{"dom", "lun", "mar", "mie", "jue", "vie", "sab"}
	dayNames = []string{"domingo", "lunes", "martes", "miércoles", "jueves", "viernes", "sábado"}
)

// ScheduleText dice cuándo puede pasar el cliente, en hora de Colombia: "hoy de 7:30 a. m. a 5:30 p. m." si hoy
// abre y no ha cerrado; si no, el siguiente día con horario: "mañana de…" o "el lunes de…" (HU-43, criterio 2).
func (s *Service) ScheduleText(w Workshop, at time.Time) string {
	now := at.In(s.loc)
	for i := range 7 {
		day := now.AddDate(0, 0, i)
		sl := w.Schedule[days[day.Weekday()]]
		if sl == nil || (i == 0 && closed(sl[1], now)) {
			continue
		}
		when := "el " + dayNames[day.Weekday()]
		switch i {
		case 0:
			when = "hoy"
		case 1:
			when = "mañana"
		}
		return when + " de " + hour(sl[0]) + " a " + hour(sl[1])
	}
	return "en nuestro horario de atención"
}

// closed dice si a la hora de now ya pasó la hora de cierre ("17:30").
func closed(hhmm string, now time.Time) bool {
	var h, m int
	if _, err := fmt.Sscanf(hhmm, "%d:%d", &h, &m); err != nil {
		return false
	}
	return now.Hour()*60+now.Minute() >= h*60+m
}

func hour(hhmm string) string {
	var h, m int
	if _, err := fmt.Sscanf(hhmm, "%d:%d", &h, &m); err != nil {
		return hhmm
	}
	suf := "a. m."
	if h >= 12 {
		suf = "p. m."
	}
	if h > 12 {
		h -= 12
	}
	if h == 0 {
		h = 12
	}
	return fmt.Sprintf("%d:%02d %s", h, m, suf)
}

// FormatDate da una fecha en palabras para el cliente, en hora de Colombia.
func (s *Service) FormatDate(t time.Time, now time.Time) string {
	lt, ln := t.In(s.loc), now.In(s.loc)
	h := hour(fmt.Sprintf("%02d:%02d", lt.Hour(), lt.Minute()))
	if lt.Year() == ln.Year() && lt.YearDay() == ln.YearDay() {
		return "hoy a las " + h
	}
	if lt.Year() == ln.Year() && lt.YearDay() == ln.YearDay()+1 {
		return "mañana a las " + h
	}
	months := []string{"enero", "febrero", "marzo", "abril", "mayo", "junio", "julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"}
	return fmt.Sprintf("el %d de %s a las %s", lt.Day(), months[lt.Month()-1], h)
}
