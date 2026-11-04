package app

import (
	"context"

	"sweveh/internal/audit"
	"sweveh/internal/orders/domain"
	"sweveh/internal/orders/ports"
	"sweveh/internal/shared"
)

// Findings agrupa los casos de uso del diagnóstico en la orden (HU-103 propuesta). Cada cambio deja
// auditoría en la misma transacción (R-06).
type Findings struct {
	uow    ports.UnitOfWork
	orders ports.FlowRepository
	repo   ports.FindingRepository
	audit  ports.AuditLog
	clock  ports.Clock
}

func NewFindings(uow ports.UnitOfWork, orders ports.FlowRepository, repo ports.FindingRepository, a ports.AuditLog, c ports.Clock) *Findings {
	return &Findings{uow: uow, orders: orders, repo: repo, audit: a, clock: c}
}

// FindingInput es lo que escribe el mecánico o la recepción.
type FindingInput struct {
	TallerID, OrderID shared.ID
	Actor             shared.Actor
	System            string
	Description       string
	Severity          string
	FaultCodes        []string
	Labor             *int64
}

func (s *Findings) Add(ctx context.Context, in FindingInput) (domain.Finding, error) {
	var out domain.Finding
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		o, err := s.orders.Load(ctx, in.TallerID, in.OrderID)
		if err != nil {
			return err
		}
		now := s.clock.Now()
		f, err := domain.NewFinding(shared.NewID(), o, in.System, in.Description, in.Severity, in.FaultCodes, actorID(in.Actor), now)
		if err != nil {
			return err
		}
		if f, err = f.WithLabor(in.Labor); err != nil {
			return err
		}
		if err := s.repo.InsertFinding(ctx, f); err != nil {
			return err
		}
		out = f
		return s.audit.Record(ctx, audit.Entry{TallerID: in.TallerID, Actor: in.Actor, Action: "orden.hallazgo.agregar", Entity: "orden",
			EntityID: o.ID, After: findingView(f), OccurredAt: now})
	})
	return out, err
}

func (s *Findings) List(ctx context.Context, tallerID, orderID shared.ID) ([]domain.Finding, error) {
	var out []domain.Finding
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		if _, err := s.orders.Load(ctx, tallerID, orderID); err != nil {
			return err
		}
		var err error
		out, err = s.repo.ListFindings(ctx, tallerID, orderID)
		return err
	})
	return out, err
}

func (s *Findings) Remove(ctx context.Context, tallerID, orderID, id shared.ID, actor shared.Actor) error {
	return s.uow.Do(ctx, func(ctx context.Context) error {
		o, err := s.orders.Load(ctx, tallerID, orderID)
		if err != nil {
			return err
		}
		if !o.CanRemoveFindings() {
			return domain.ErrFindingLocked
		}
		f, err := s.repo.DeleteFinding(ctx, tallerID, orderID, id)
		if err != nil {
			return err
		}
		return s.audit.Record(ctx, audit.Entry{TallerID: tallerID, Actor: actor, Action: "orden.hallazgo.borrar", Entity: "orden",
			EntityID: o.ID, Before: findingView(f), OccurredAt: s.clock.Now()})
	})
}

func findingView(f domain.Finding) map[string]any {
	return map[string]any{"hallazgo_id": f.ID.String(), "sistema": f.System, "descripcion": f.Description, "gravedad": f.Severity,
		"codigos_falla": f.FaultCodes, "mano_obra": f.Labor}
}
