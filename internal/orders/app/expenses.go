package app

import (
	"context"

	"sweveh/internal/audit"
	"sweveh/internal/orders/domain"
	"sweveh/internal/orders/ports"
	"sweveh/internal/shared"
)

// Expenses es lo gastado contra lo cotizado (V-08, ADR-0040, HU-106). Interno del taller.
type Expenses struct {
	uow    ports.UnitOfWork
	orders ports.FlowRepository
	repo   ports.ExpenseRepository
	quoted ports.QuotedTotal
	audit  ports.AuditLog
	clock  ports.Clock
}

func NewExpenses(uow ports.UnitOfWork, orders ports.FlowRepository, repo ports.ExpenseRepository, quoted ports.QuotedTotal, a ports.AuditLog, c ports.Clock) *Expenses {
	return &Expenses{uow: uow, orders: orders, repo: repo, quoted: quoted, audit: a, clock: c}
}

// ExpenseInput es lo que anota la persona (o propone el asistente y la persona confirma, R8). ID lo genera la
// pantalla: repetir el mismo envío no anota el gasto dos veces.
type ExpenseInput struct {
	TallerID, OrderID, ID shared.ID
	Actor                 shared.Actor
	Description           string
	Value                 int64
	Origin                string
	PhotoID               *shared.ID
}

var ErrExpensePhoto = shared.Validation("gasto_foto", "La foto de la factura no es de esta orden.").WithField("fotoId")

// Add anota un gasto y devuelve cómo va la orden.
func (s *Expenses) Add(ctx context.Context, in ExpenseInput) (domain.Expense, domain.Spend, error) {
	var (
		out   domain.Expense
		spend domain.Spend
	)
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		if _, err := s.orders.Load(ctx, in.TallerID, in.OrderID); err != nil {
			return err
		}
		if in.PhotoID != nil {
			ok, err := s.repo.EvidenceInOrder(ctx, in.TallerID, in.OrderID, *in.PhotoID)
			if err != nil {
				return err
			}
			if !ok {
				return ErrExpensePhoto
			}
		}
		now := s.clock.Now()
		e, err := domain.NewExpense(in.ID, in.OrderID, in.Description, in.Value, in.Origin, in.PhotoID, actorID(in.Actor), now)
		if err != nil {
			return err
		}
		inserted, err := s.repo.InsertExpense(ctx, in.TallerID, e)
		if err != nil {
			return err
		}
		if out, err = s.repo.GetExpense(ctx, in.TallerID, in.OrderID, e.ID); err != nil {
			return err
		}
		if spend, err = s.spend(ctx, in.TallerID, in.OrderID); err != nil {
			return err
		}
		if !inserted { // el mismo envío otra vez: nada nuevo en la bitácora
			return nil
		}
		return s.audit.Record(ctx, audit.Entry{TallerID: in.TallerID, Actor: in.Actor, Action: "orden.gasto.anotar", Entity: "orden",
			EntityID: in.OrderID, After: map[string]any{"gasto_id": e.ID.String(), "descripcion": e.Description, "valor": e.Value,
				"origen": e.Origin}, OccurredAt: now})
	})
	return out, spend, err
}

// Void anula un gasto (no se borra). Anular dos veces no cambia nada ni repite la bitácora.
func (s *Expenses) Void(ctx context.Context, tallerID, orderID, id shared.ID, actor shared.Actor) (domain.Spend, error) {
	var spend domain.Spend
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		e, err := s.repo.GetExpense(ctx, tallerID, orderID, id)
		if err != nil {
			return err
		}
		changed, err := s.repo.VoidExpense(ctx, tallerID, orderID, id, actorID(actor))
		if err != nil {
			return err
		}
		if spend, err = s.spend(ctx, tallerID, orderID); err != nil || !changed {
			return err
		}
		return s.audit.Record(ctx, audit.Entry{TallerID: tallerID, Actor: actor, Action: "orden.gasto.anular", Entity: "orden",
			EntityID: orderID, Before: map[string]any{"gasto_id": id.String(), "valor": e.Value}, OccurredAt: s.clock.Now()})
	})
	return spend, err
}

// List devuelve los gastos de la orden y el resumen cotizado / gastado / quedan.
func (s *Expenses) List(ctx context.Context, tallerID, orderID shared.ID) ([]domain.Expense, domain.Spend, error) {
	var (
		out   []domain.Expense
		spend domain.Spend
	)
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		if _, err := s.orders.Load(ctx, tallerID, orderID); err != nil {
			return err
		}
		var err error
		if out, err = s.repo.ListExpenses(ctx, tallerID, orderID); err != nil {
			return err
		}
		quoted, err := s.quoted.Quoted(ctx, tallerID, orderID)
		spend = domain.SpendOf(quoted, out)
		return err
	})
	return out, spend, err
}

// SpentByOpenOrder: lo gastado en las órdenes abiertas con gastos, en la transacción del llamador (KPI 5).
func (s *Expenses) SpentByOpenOrder(ctx context.Context, tallerID shared.ID) (map[shared.ID]int64, error) {
	return s.repo.SpentByOpenOrder(ctx, tallerID)
}

func (s *Expenses) spend(ctx context.Context, tallerID, orderID shared.ID) (domain.Spend, error) {
	list, err := s.repo.ListExpenses(ctx, tallerID, orderID)
	if err != nil {
		return domain.Spend{}, err
	}
	quoted, err := s.quoted.Quoted(ctx, tallerID, orderID)
	return domain.SpendOf(quoted, list), err
}
