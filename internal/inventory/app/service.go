// Package app contiene los casos de uso de inventory.
package app

import (
	"context"
	"time"

	"sweveh/internal/audit"
	"sweveh/internal/inventory/domain"
	"sweveh/internal/shared"
)

type Repository interface {
	Search(ctx context.Context, tallerID shared.ID, q string, onlyActive bool, limit int, category string) ([]domain.Part, error)
	Get(ctx context.Context, tallerID, id shared.ID) (domain.Part, error)
	GetMany(ctx context.Context, tallerID shared.ID, ids []shared.ID) ([]domain.Part, error)
	Insert(ctx context.Context, p domain.Part) error
	Update(ctx context.Context, p domain.Part, prevVersion int) error
	// AddStock suma (o resta si n < 0 sin bajar de cero) y registra el movimiento. Devuelve false si no alcanzó.
	Move(ctx context.Context, m Movement) (bool, error)
	Low(ctx context.Context, tallerID shared.ID) ([]domain.Part, error)
	// Returnable bloquea el repuesto y dice cuánto descontó la orden y no se ha devuelto (HU-38).
	Returnable(ctx context.Context, tallerID, partID, orderID shared.ID) (int, error)
	Movements(ctx context.Context, tallerID, partID shared.ID, limit int) ([]domain.Movement, error)
	ExistingRefs(ctx context.Context, tallerID shared.ID, refs []string) (map[string]bool, error)
	OrderParts(ctx context.Context, tallerID, orderID shared.ID) ([]domain.OrderPart, error)
	Showcase(ctx context.Context, tallerID shared.ID, q string, limit int) ([]domain.Part, error)
	SetPhoto(ctx context.Context, tallerID, id shared.ID, key, mime string) error
}

// ReturnResult dice qué pasó con la devolución, para mostrarlo en la orden (HU-38).
type ReturnResult struct {
	Returned int
	Left     int // lo que aún se puede devolver de esa orden
	Stock    int // existencia del repuesto después de devolver
}

// ErrReturnTooMany evita que una devolución infle la existencia por encima de lo que la orden usó (HU-38).
var ErrReturnTooMany = shared.Rule("devolucion_excede", "No puedes devolver más unidades de las que esa orden descontó.").WithField("cantidad")

// Movement es un movimiento del libro de inventario.
type Movement struct {
	TallerID shared.ID
	PartID   shared.ID
	Qty      int // positivo entra, negativo sale
	Reason   string
	OrderID  *shared.ID
	LineID   *shared.ID
	UserID   *shared.ID
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

type Service struct {
	uow    UnitOfWork
	repo   Repository
	audit  AuditLog
	events EventPublisher
	clock  Clock
	photos PhotoStore // vitrina de WhatsApp; se conecta con SetPhotoStore
}

func NewService(uow UnitOfWork, r Repository, a AuditLog, e EventPublisher, c Clock) *Service {
	return &Service{uow: uow, repo: r, audit: a, events: e, clock: c}
}

// Search busca repuestos (HU-34). all incluye inactivos; category "" = todas.
func (s *Service) Search(ctx context.Context, tallerID shared.ID, q string, all bool, category string) ([]domain.Part, error) {
	var out []domain.Part
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.repo.Search(ctx, tallerID, q, !all, 200, category)
		return err
	})
	return out, err
}

// Create crea un repuesto con referencia única en el taller (HU-32). La existencia inicial entra como movimiento.
func (s *Service) Create(ctx context.Context, actor shared.Actor, p domain.Part) (domain.Part, error) {
	p.ID, p.Active, p.Version = shared.NewID(), true, 1
	initial := p.Stock
	p.Stock = 0
	if err := p.Normalize(); err != nil {
		return domain.Part{}, err
	}
	if initial < 0 {
		return domain.Part{}, domain.ErrQty
	}
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		if err := s.repo.Insert(ctx, p); err != nil {
			return err
		}
		if initial > 0 {
			if _, err := s.repo.Move(ctx, Movement{TallerID: p.TallerID, PartID: p.ID, Qty: initial, Reason: "entrada", UserID: &actor.ID}); err != nil {
				return err
			}
			p.Stock = initial
		}
		return s.audit.Record(ctx, audit.Entry{TallerID: p.TallerID, Actor: actor, Action: "repuesto.crear", Entity: "repuesto",
			EntityID: p.ID, After: view(p), OccurredAt: s.clock.Now()})
	})
	return p, err
}

type Patch struct {
	Name     *string
	Kind     *string
	Cost     *int64
	Price    *int64
	Min      *int
	Active   *bool
	Brand    *string
	Category *string
	Location *string
	OEM      *string
	Showcase *bool
}

// Update edita o desactiva (HU-32). Desactivado deja de salir al cotizar; el historial lo conserva.
func (s *Service) Update(ctx context.Context, actor shared.Actor, tallerID, id shared.ID, version int, ch Patch) (domain.Part, error) {
	var out domain.Part
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		cur, err := s.repo.Get(ctx, tallerID, id)
		if err != nil {
			return err
		}
		if cur.Version != version {
			return shared.ErrVersionConflict
		}
		next := cur
		set(&next.Name, ch.Name)
		set(&next.Kind, ch.Kind)
		set(&next.Cost, ch.Cost)
		set(&next.Price, ch.Price)
		set(&next.Min, ch.Min)
		set(&next.Active, ch.Active)
		set(&next.Brand, ch.Brand)
		set(&next.Category, ch.Category)
		set(&next.Location, ch.Location)
		set(&next.OEM, ch.OEM)
		set(&next.Showcase, ch.Showcase)
		if err := next.Normalize(); err != nil {
			return err
		}
		if err := s.repo.Update(ctx, next, version); err != nil {
			return err
		}
		next.Version++
		out = next
		return s.audit.Record(ctx, audit.Entry{TallerID: tallerID, Actor: actor, Action: "repuesto.cambiar", Entity: "repuesto",
			EntityID: id, Before: view(cur), After: view(next), OccurredAt: s.clock.Now()})
	})
	return out, err
}

// AddEntry registra una entrada de mercancía (HU-35).
func (s *Service) AddEntry(ctx context.Context, actor shared.Actor, tallerID, id shared.ID, qty int) (domain.Part, error) {
	if qty <= 0 {
		return domain.Part{}, domain.ErrQty
	}
	var out domain.Part
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		if _, err := s.repo.Get(ctx, tallerID, id); err != nil {
			return err
		}
		if _, err := s.repo.Move(ctx, Movement{TallerID: tallerID, PartID: id, Qty: qty, Reason: "entrada", UserID: &actor.ID}); err != nil {
			return err
		}
		var err error
		out, err = s.repo.Get(ctx, tallerID, id)
		if err != nil {
			return err
		}
		return s.audit.Record(ctx, audit.Entry{TallerID: tallerID, Actor: actor, Action: "repuesto.entrada", Entity: "repuesto",
			EntityID: id, After: map[string]any{"cantidad": qty, "existencia": out.Stock}, OccurredAt: s.clock.Now()})
	})
	return out, err
}

// Low lista los repuestos en el mínimo o por debajo (HU-36).
func (s *Service) Low(ctx context.Context, tallerID shared.ID) ([]domain.Part, error) {
	var out []domain.Part
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.repo.Low(ctx, tallerID)
		return err
	})
	return out, err
}

// LineToDiscount es una línea aprobada que consume inventario.
type LineToDiscount struct {
	LineID shared.ID
	PartID shared.ID
	Qty    int
}

// Discount descuenta los repuestos de una orden aprobada (HU-33) en la transacción del llamador.
// Si no alcanza la existencia, queda un movimiento "pendiente_orden" y el inventario no baja de cero (Q-11).
// Es idempotente por línea (índice único linea_id + motivo).
func (s *Service) Discount(ctx context.Context, tallerID, orderID shared.ID, lines []LineToDiscount, by shared.Actor) ([]shared.ID, error) {
	var pending []shared.ID
	for _, l := range lines {
		lineID, oid := l.LineID, orderID
		ok, err := s.repo.Move(ctx, Movement{TallerID: tallerID, PartID: l.PartID, Qty: -l.Qty, Reason: "salida_orden",
			OrderID: &oid, LineID: &lineID, UserID: actorID(by)})
		if err != nil {
			return nil, err
		}
		if !ok {
			if _, err := s.repo.Move(ctx, Movement{TallerID: tallerID, PartID: l.PartID, Qty: -l.Qty, Reason: "pendiente_orden",
				OrderID: &oid, LineID: &lineID, UserID: actorID(by)}); err != nil {
				return nil, err
			}
			pending = append(pending, l.PartID)
		}
	}
	low, err := s.repo.Low(ctx, tallerID)
	if err != nil {
		return nil, err
	}
	for _, p := range low {
		if err := s.events.Publish(ctx, tallerID, shared.Event{Type: "inventory.bajo_minimo", AggregateID: p.ID,
			OccurredAt: s.clock.Now(), Payload: map[string]any{"referencia": p.Ref, "existencia": p.Stock}}); err != nil {
			return nil, err
		}
	}
	return pending, nil
}

// Return devuelve al inventario lo que no se usó (HU-38).
func (s *Service) Return(ctx context.Context, actor shared.Actor, tallerID, partID, orderID shared.ID, qty int) (ReturnResult, error) {
	if qty <= 0 {
		return ReturnResult{}, domain.ErrQty
	}
	var res ReturnResult
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		if _, err := s.repo.Get(ctx, tallerID, partID); err != nil {
			return err
		}
		left, err := s.repo.Returnable(ctx, tallerID, partID, orderID)
		if err != nil {
			return err
		}
		if qty > left {
			return ErrReturnTooMany
		}
		if _, err := s.repo.Move(ctx, Movement{TallerID: tallerID, PartID: partID, Qty: qty, Reason: "devolucion", OrderID: &orderID, UserID: &actor.ID}); err != nil {
			return err
		}
		p, err := s.repo.Get(ctx, tallerID, partID)
		if err != nil {
			return err
		}
		res = ReturnResult{Returned: qty, Left: left - qty, Stock: p.Stock}
		return s.audit.Record(ctx, audit.Entry{TallerID: tallerID, Actor: actor, Action: "repuesto.devolucion", Entity: "repuesto",
			EntityID: partID, After: map[string]any{"cantidad": qty, "orden_id": orderID.String(), "existencia": p.Stock}, OccurredAt: s.clock.Now()})
	})
	return res, err
}

// OrderParts resume por repuesto lo descontado, devuelto y pendiente de una orden (HU-33, HU-38).
func (s *Service) OrderParts(ctx context.Context, tallerID, orderID shared.ID) ([]domain.OrderPart, error) {
	var out []domain.OrderPart
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.repo.OrderParts(ctx, tallerID, orderID)
		return err
	})
	return out, err
}

// GetMany devuelve varios repuestos (lo usa la cotización dentro de su transacción).
func (s *Service) GetMany(ctx context.Context, tallerID shared.ID, ids []shared.ID) ([]domain.Part, error) {
	return s.repo.GetMany(ctx, tallerID, ids)
}

func actorID(a shared.Actor) *shared.ID {
	if a.ID == shared.NilID {
		return nil
	}
	id := a.ID
	return &id
}

func set[T any](dst *T, v *T) {
	if v != nil {
		*dst = *v
	}
}

func view(p domain.Part) map[string]any {
	return map[string]any{"referencia": p.Ref, "nombre": p.Name, "costo": p.Cost, "precio_venta": p.Price,
		"minimo": p.Min, "activo": p.Active, "marca": p.Brand, "categoria": p.Category, "ubicacion": p.Location, "referencia_oem": p.OEM,
		"ofrecer_whatsapp": p.Showcase}
}
