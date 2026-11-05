// Package app contiene los casos de uso de quotes.
package app

import (
	"context"
	"time"

	"sweveh/internal/audit"
	"sweveh/internal/quotes/domain"
	"sweveh/internal/shared"
)

type Repository interface {
	Latest(ctx context.Context, tallerID, orderID shared.ID) (domain.Quote, bool, error)
	Get(ctx context.Context, tallerID, id shared.ID) (domain.Quote, error)
	Insert(ctx context.Context, q domain.Quote) error
	Save(ctx context.Context, q domain.Quote, prevVersion int) error // reemplaza líneas y cabecera
	PendingByCustomerOrder(ctx context.Context, tallerID, orderID shared.ID) (domain.Quote, bool, error)
}

// Order es lo que la cotización necesita saber de la orden.
type Order struct {
	ID         shared.ID
	State      string
	CustomerID shared.ID
	Plate      string
	Number     int
}

type Orders interface {
	Load(ctx context.Context, tallerID, id shared.ID) (Order, error)
	Move(ctx context.Context, tallerID, id shared.ID, to string, actor shared.Actor, reason string) error
}

type Service struct {
	Name  string
	Price *int64
}

type Catalog interface {
	Get(ctx context.Context, tallerID, id shared.ID) (Service, error)
	// FillPrice guarda en la lista el precio de un servicio que no tenía (HU-06); no cambia uno que ya existe.
	FillPrice(ctx context.Context, actor shared.Actor, tallerID, id shared.ID, price int64) error
}

type Part struct {
	Name   string
	Ref    string
	Price  int64
	Stock  int
	Active bool
}

type DiscountLine struct {
	LineID, PartID shared.ID
	Qty            int
}

type Inventory interface {
	Parts(ctx context.Context, tallerID shared.ID, ids []shared.ID) (map[shared.ID]Part, error)
	Discount(ctx context.Context, tallerID, orderID shared.ID, lines []DiscountLine, by shared.Actor) ([]shared.ID, error)
}

type UnitOfWork interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}
type AuditLog interface {
	Record(ctx context.Context, e audit.Entry) error
}
type EventPublisher interface {
	Publish(ctx context.Context, tallerID shared.ID, e shared.Event) error
}
type Clock interface{ Now() time.Time }

type Quotes struct {
	uow     UnitOfWork
	repo    Repository
	orders  Orders
	catalog Catalog
	inv     Inventory
	audit   AuditLog
	events  EventPublisher
	clock   Clock
	rules   RulesReader
}

// RulesReader lee los parámetros de cotización del taller (fachada de tenancy) en la transacción en curso.
type RulesReader interface {
	QuoteRules(ctx context.Context, tallerID shared.ID) (domain.Rules, error)
}

// SetRules conecta los parámetros del taller; sin lector rigen domain.DefaultRules (sin IVA).
func (s *Quotes) SetRules(r RulesReader) { s.rules = r }

func (s *Quotes) rulesIn(ctx context.Context, tallerID shared.ID) (domain.Rules, error) {
	if s.rules == nil {
		return domain.DefaultRules, nil
	}
	return s.rules.QuoteRules(ctx, tallerID)
}

func NewService(uow UnitOfWork, r Repository, o Orders, c Catalog, i Inventory, a AuditLog, e EventPublisher, clk Clock) *Quotes {
	return &Quotes{uow: uow, repo: r, orders: o, catalog: c, inv: i, audit: a, events: e, clock: clk}
}

// ForOrder devuelve la cotización vigente de la orden; si no hay, nil.
func (s *Quotes) ForOrder(ctx context.Context, tallerID, orderID shared.ID) (*domain.Quote, error) {
	var out *domain.Quote
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		if _, err := s.orders.Load(ctx, tallerID, orderID); err != nil {
			return err
		}
		q, ok, err := s.repo.Latest(ctx, tallerID, orderID)
		if ok {
			out = &q
		}
		return err
	})
	return out, err
}

// draft devuelve la cotización en borrador de la orden o crea una (la adicional si ya hubo una aprobada, HU-31).
func (s *Quotes) draft(ctx context.Context, tallerID, orderID shared.ID, actor shared.Actor) (domain.Quote, error) {
	o, err := s.orders.Load(ctx, tallerID, orderID)
	if err != nil {
		return domain.Quote{}, err
	}
	q, ok, err := s.repo.Latest(ctx, tallerID, orderID)
	if err != nil {
		return domain.Quote{}, err
	}
	if ok && q.State == domain.Draft {
		return q, nil
	}
	if o.State != "en_diagnostico" && o.State != "en_reparacion" {
		return domain.Quote{}, domain.ErrOrderStage
	}
	if ok && (q.State == domain.Sent || q.State == domain.Doubts) {
		return domain.Quote{}, domain.ErrNotEditable
	}
	n := 1
	if ok {
		n = q.Number + 1
	}
	nq := domain.Quote{ID: shared.NewID(), TallerID: tallerID, OrderID: orderID, Number: n, State: domain.Draft, Version: 1}
	r, err := s.rulesIn(ctx, tallerID)
	if err != nil {
		return domain.Quote{}, err
	}
	nq.ApplyRules(r)
	if err := s.repo.Insert(ctx, nq); err != nil {
		return domain.Quote{}, err
	}
	_ = actor
	return nq, nil
}

// LineInput es un ítem pedido: servicio del catálogo, repuesto del inventario o ítem libre.
type LineInput struct {
	Kind        string
	RefID       *shared.ID
	Description string
	Qty         int
	UnitPrice   *int64 // obligatorio si el servicio no tiene precio o el ítem es libre
	HoursCent   *int   // mano de obra por horas: el precio sale de la tarifa del taller
	// SaveToList: si el servicio no tiene precio de lista, el que se escribe queda en la lista (HU-06). Solo para quien
	// administra los servicios (service:manage): el mecánico cotiza con su valor sin cambiar la lista del dueño.
	SaveToList bool
}

var ErrPriceNeeded = shared.Validation("precio_requerido", "Este servicio no tiene precio: escribe el valor para cotizarlo.").WithField("precioUnitario")

// AddLine agrega un ítem (HU-27, HU-34). Un repuesto sin existencias avisa pero se deja cotizar (Q-11).
func (s *Quotes) AddLine(ctx context.Context, tallerID, orderID shared.ID, actor shared.Actor, in LineInput) (domain.Quote, error) {
	var out domain.Quote
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		q, err := s.draft(ctx, tallerID, orderID, actor)
		if err != nil {
			return err
		}
		l := domain.Line{ID: shared.NewID(), Kind: in.Kind, RefID: in.RefID, Description: in.Description, Qty: in.Qty}
		var stock *int // existencia del repuesto, para marcar "sin existencia" con la cantidad total de la línea
		switch {
		case in.Kind == "servicio" && in.RefID != nil:
			sv, err := s.catalog.Get(ctx, tallerID, *in.RefID)
			if err != nil {
				return err
			}
			l.Description = sv.Name
			if sv.Price != nil {
				l.UnitPrice = *sv.Price
			} else if in.UnitPrice != nil {
				l.UnitPrice, l.FreePrice = *in.UnitPrice, true
				if in.SaveToList {
					if err := s.catalog.FillPrice(ctx, actor, tallerID, *in.RefID, *in.UnitPrice); err != nil {
						return err
					}
				}
			} else {
				return ErrPriceNeeded
			}
		case in.Kind == "repuesto" && in.RefID != nil:
			parts, err := s.inv.Parts(ctx, tallerID, []shared.ID{*in.RefID})
			if err != nil {
				return err
			}
			p, ok := parts[*in.RefID]
			if !ok || !p.Active {
				return shared.NotFound("repuesto_no_encontrado", "No encontramos ese repuesto en tu taller.")
			}
			l.Description, l.UnitPrice, stock = p.Name+" ("+p.Ref+")", p.Price, &p.Stock
		case in.HoursCent != nil:
			r, err := s.rulesIn(ctx, tallerID)
			if err != nil {
				return err
			}
			if l.UnitPrice, err = domain.LaborPrice(*in.HoursCent, r.HourlyRate); err != nil {
				return err
			}
			l.Qty, l.HoursCent = 1, in.HoursCent
		default:
			if in.UnitPrice == nil {
				return ErrPriceNeeded
			}
			l.UnitPrice, l.FreePrice = *in.UnitPrice, true
		}
		pv := q.Version
		if err := q.AddLine(l); err != nil {
			return err
		}
		if stock != nil {
			markNoStock(&q, *in.RefID, *stock)
		}
		q.Version++
		if err := s.repo.Save(ctx, q, pv); err != nil {
			return err
		}
		out = q
		return s.audit.Record(ctx, audit.Entry{TallerID: tallerID, Actor: actor, Action: "cotizacion.agregar_item", Entity: "cotizacion",
			EntityID: q.ID, After: map[string]any{"descripcion": l.Description, "cantidad": l.Qty, "precio": l.UnitPrice}, OccurredAt: s.clock.Now()})
	})
	return out, err
}

// LineChange es lo que se puede cambiar de un ítem del borrador.
type LineChange struct {
	Qty         *int
	Price       *int64
	HoursCent   *int
	DiscountPct *int
	IsOwner     bool // el dueño siempre puede dar descuentos; recepción solo si el taller lo permite
}

// UpdateLine cambia cantidad, precio libre, horas o descuento de un ítem del borrador; recalcula "sin existencia".
func (s *Quotes) UpdateLine(ctx context.Context, tallerID, orderID, lineID shared.ID, actor shared.Actor, ch LineChange) (domain.Quote, error) {
	var out domain.Quote
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		q, ok, err := s.repo.Latest(ctx, tallerID, orderID)
		if err != nil {
			return err
		}
		if !ok {
			return domain.ErrNotFound
		}
		pv := q.Version
		l, err := s.applyChange(ctx, &q, tallerID, lineID, ch)
		if err != nil {
			return err
		}
		if l.Kind == "repuesto" && l.RefID != nil {
			parts, err := s.inv.Parts(ctx, tallerID, []shared.ID{*l.RefID})
			if err != nil {
				return err
			}
			for i := range q.Lines {
				if q.Lines[i].ID == lineID {
					q.Lines[i].NoStock = parts[*l.RefID].Stock < l.Qty
				}
			}
		}
		q.Version++
		if err := s.repo.Save(ctx, q, pv); err != nil {
			return err
		}
		out = q
		return s.audit.Record(ctx, audit.Entry{TallerID: tallerID, Actor: actor, Action: "cotizacion.cambiar_item", Entity: "cotizacion",
			EntityID: q.ID, After: map[string]any{"linea_id": lineID.String(), "cantidad": l.Qty, "precio": l.UnitPrice, "descuento_pct": l.DiscountPct,
				"horas_cent": l.HoursCent}, OccurredAt: s.clock.Now()})
	})
	return out, err
}

func (s *Quotes) RemoveLine(ctx context.Context, tallerID, orderID, lineID shared.ID, actor shared.Actor) (domain.Quote, error) {
	var out domain.Quote
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		q, ok, err := s.repo.Latest(ctx, tallerID, orderID)
		if err != nil {
			return err
		}
		if !ok {
			return domain.ErrNotFound
		}
		pv := q.Version
		if err := q.RemoveLine(lineID); err != nil {
			return err
		}
		q.Version++
		if err := s.repo.Save(ctx, q, pv); err != nil {
			return err
		}
		out = q
		return s.audit.Record(ctx, audit.Entry{TallerID: tallerID, Actor: actor, Action: "cotizacion.quitar_item", Entity: "cotizacion",
			EntityID: q.ID, After: map[string]any{"linea_id": lineID.String()}, OccurredAt: s.clock.Now()})
	})
	return out, err
}

// Send envía la cotización: la orden pasa a Esperando aprobación y sale el evento para AU-02 (HU-27, criterio 4).
// La adicional (HU-31) no cambia el estado: la orden sigue en reparación.
func (s *Quotes) Send(ctx context.Context, tallerID, orderID shared.ID, actor shared.Actor) (domain.Quote, error) {
	var out domain.Quote
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		q, ok, err := s.repo.Latest(ctx, tallerID, orderID)
		if err != nil {
			return err
		}
		if !ok {
			return domain.ErrEmpty
		}
		o, err := s.orders.Load(ctx, tallerID, orderID)
		if err != nil {
			return err
		}
		now := s.clock.Now()
		pv := q.Version
		r, err := s.rulesIn(ctx, tallerID)
		if err != nil {
			return err
		}
		if err := q.Send(now, r.ValidityDays); err != nil {
			return err
		}
		q.Version++
		if err := s.repo.Save(ctx, q, pv); err != nil {
			return err
		}
		if o.State == "en_diagnostico" {
			if err := s.orders.Move(ctx, tallerID, orderID, "esperando_aprobacion", actor, "cotización enviada"); err != nil {
				return err
			}
		}
		if err := s.audit.Record(ctx, audit.Entry{TallerID: tallerID, Actor: actor, Action: "cotizacion.enviar", Entity: "cotizacion",
			EntityID: q.ID, After: map[string]any{"total": q.Total(), "items": len(q.Lines)}, OccurredAt: now}); err != nil {
			return err
		}
		out = q
		return s.events.Publish(ctx, tallerID, shared.Event{Type: "quotes.enviada", AggregateID: q.ID, OccurredAt: now,
			Payload: map[string]any{"orden_id": orderID.String(), "cliente_id": o.CustomerID.String(), "total": q.Total(),
				"numero": q.Number, "placa": o.Plate, "orden_numero": o.Number}})
	})
	return out, err
}

// RespondCmd es una respuesta del cliente (botón de WhatsApp) o registrada por el dueño (teléfono, mostrador).
type RespondCmd struct {
	TallerID, OrderID shared.ID
	Actor             shared.Actor
	Response          domain.Response
}

// Respond registra la respuesta (HU-28, HU-29, HU-30). Al aprobar: la orden pasa a En reparación y se
// descuentan los repuestos aprobados (HU-33), todo en la misma transacción. Idempotente por id de mensaje.
func (s *Quotes) Respond(ctx context.Context, c RespondCmd) (domain.Quote, error) {
	var out domain.Quote
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		q, ok, err := s.repo.Latest(ctx, c.TallerID, c.OrderID)
		if err != nil {
			return err
		}
		if !ok {
			return domain.ErrNotFound
		}
		if c.Response.Ref != "" && q.ResponseRef != nil && *q.ResponseRef == c.Response.Ref {
			out = q // reintento del mismo mensaje: no se aplica dos veces
			return nil
		}
		if c.Response.At.IsZero() {
			c.Response.At = s.clock.Now()
		}
		pv := q.Version
		approved, err := q.Respond(c.Response)
		if err != nil {
			return err
		}
		q.Version++
		if err := s.repo.Save(ctx, q, pv); err != nil {
			return err
		}
		if approved {
			o, err := s.orders.Load(ctx, c.TallerID, c.OrderID)
			if err != nil {
				return err
			}
			if o.State == "esperando_aprobacion" {
				if err := s.orders.Move(ctx, c.TallerID, c.OrderID, "en_reparacion", c.Actor, "cotización aprobada por "+c.Response.Channel); err != nil {
					return err
				}
			}
			var lines []DiscountLine
			for _, l := range q.ApprovedParts() {
				lines = append(lines, DiscountLine{LineID: l.ID, PartID: *l.RefID, Qty: l.Qty})
			}
			if len(lines) > 0 {
				if _, err := s.inv.Discount(ctx, c.TallerID, c.OrderID, lines, c.Actor); err != nil {
					return err
				}
			}
		}
		now := s.clock.Now()
		if err := s.audit.Record(ctx, audit.Entry{TallerID: c.TallerID, Actor: c.Actor, Action: "cotizacion.respuesta", Entity: "cotizacion",
			EntityID: q.ID, After: map[string]any{"decision": c.Response.Decision, "canal": c.Response.Channel, "total": q.Total()},
			OccurredAt: now}); err != nil {
			return err
		}
		out = q
		return s.events.Publish(ctx, c.TallerID, shared.Event{Type: "quotes.respondida", AggregateID: q.ID, OccurredAt: now,
			Payload: map[string]any{"orden_id": c.OrderID.String(), "decision": c.Response.Decision, "canal": c.Response.Channel}})
	})
	return out, err
}

// markNoStock marca "sin existencia" en la línea del repuesto según su cantidad total (también tras sumar).
func markNoStock(q *domain.Quote, partID shared.ID, stock int) {
	for i := range q.Lines {
		if q.Lines[i].RefID != nil && *q.Lines[i].RefID == partID && q.Lines[i].Kind == "repuesto" {
			q.Lines[i].NoStock = stock < q.Lines[i].Qty
		}
	}
}

// applyChange aplica cantidad o precio, horas (con la tarifa del taller) y descuento (con su tope y permiso).
func (s *Quotes) applyChange(ctx context.Context, q *domain.Quote, tallerID, lineID shared.ID, ch LineChange) (domain.Line, error) {
	l, err := q.UpdateLine(lineID, ch.Qty, ch.Price)
	if err != nil || (ch.HoursCent == nil && ch.DiscountPct == nil) {
		return l, err
	}
	r, err := s.rulesIn(ctx, tallerID)
	if err != nil {
		return l, err
	}
	if ch.HoursCent != nil {
		if l.HoursCent == nil {
			return l, domain.ErrHours
		}
		price, err := domain.LaborPrice(*ch.HoursCent, r.HourlyRate)
		if err != nil {
			return l, err
		}
		for i := range q.Lines {
			if q.Lines[i].ID == lineID {
				q.Lines[i].UnitPrice, q.Lines[i].HoursCent = price, ch.HoursCent
				l = q.Lines[i]
			}
		}
	}
	if ch.DiscountPct != nil {
		if !ch.IsOwner && !r.ReceptionDiscount {
			return l, domain.ErrDiscountDenied
		}
		return q.SetDiscount(lineID, *ch.DiscountPct, r.MaxDiscountPct)
	}
	return l, nil
}

// Renew vuelve a contar la vigencia de una cotización enviada (HU-27, parámetros del taller).
func (s *Quotes) Renew(ctx context.Context, tallerID, orderID shared.ID, actor shared.Actor) (domain.Quote, error) {
	var out domain.Quote
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		q, ok, err := s.repo.Latest(ctx, tallerID, orderID)
		if err != nil {
			return err
		}
		if !ok {
			return domain.ErrNotFound
		}
		r, err := s.rulesIn(ctx, tallerID)
		if err != nil {
			return err
		}
		pv, now := q.Version, s.clock.Now()
		if err := q.Renew(now, r.ValidityDays); err != nil {
			return err
		}
		q.Version++
		if err := s.repo.Save(ctx, q, pv); err != nil {
			return err
		}
		out = q
		return s.audit.Record(ctx, audit.Entry{TallerID: tallerID, Actor: actor, Action: "cotizacion.renovar", Entity: "cotizacion",
			EntityID: q.ID, After: map[string]any{"vence_en": q.ExpiresAt}, OccurredAt: now})
	})
	return out, err
}
