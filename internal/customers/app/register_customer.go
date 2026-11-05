// Package app contiene los casos de uso de customers.
package app

import (
	"context"
	"errors"

	"sweveh/internal/audit"
	"sweveh/internal/customers/domain"
	"sweveh/internal/customers/ports"
	"sweveh/internal/shared"
)

// RegisterCustomer es el comando de HU-09: buscar o crear por celular.
type RegisterCustomer struct {
	TallerID shared.ID
	Actor    shared.Actor
	Phone    string
	Name     string
	Email    *string
}

// RegisterResult dice si el cliente ya existía (HU-09, criterio 1).
type RegisterResult struct {
	Customer domain.Customer
	Existing bool
}

// Service agrupa los casos de uso del módulo.
type Service struct {
	uow    ports.UnitOfWork
	repo   ports.CustomerRepository
	audit  ports.AuditLog
	events ports.EventPublisher
	clock  ports.Clock
	extra  Extra
	mover  CustomerMover  // unir clientes repetidos (HU-15)
	eraser CustomerEraser // borrar los datos del titular (HU-87)
}

func NewService(uow ports.UnitOfWork, repo ports.CustomerRepository, a ports.AuditLog, e ports.EventPublisher, c ports.Clock) *Service {
	return &Service{uow: uow, repo: repo, audit: a, events: e, clock: c}
}

// Register busca el cliente por celular; si no existe lo crea con autorización pendiente.
func (s *Service) Register(ctx context.Context, cmd RegisterCustomer) (RegisterResult, error) {
	phone, err := shared.ParseColombianMobile(cmd.Phone)
	if err != nil {
		return RegisterResult{}, shared.ErrPhoneInvalid.WithField("celular")
	}
	var res RegisterResult
	err = s.uow.Do(ctx, func(ctx context.Context) error {
		existing, err := s.repo.ByPhone(ctx, cmd.TallerID, phone)
		if err == nil {
			res = RegisterResult{Customer: existing, Existing: true}
			return nil
		}
		if !errors.Is(err, domain.ErrCustomerNotFound) {
			return err
		}
		now := s.clock.Now()
		c, err := domain.NewCustomer(shared.NewID(), cmd.TallerID, phone, cmd.Name, cmd.Email, now)
		if err != nil {
			return err
		}
		inserted, err := s.repo.Insert(ctx, c, cmd.Actor.ID)
		if err != nil {
			return err
		}
		if !inserted { // otro usuario lo creó en este mismo instante
			existing, err := s.repo.ByPhone(ctx, cmd.TallerID, phone)
			if err != nil {
				return err
			}
			res = RegisterResult{Customer: existing, Existing: true}
			return nil
		}
		if err := s.audit.Record(ctx, audit.Entry{
			TallerID: cmd.TallerID, Actor: cmd.Actor, Action: "cliente.crear", Entity: "cliente",
			EntityID: c.ID, After: auditView(c), OccurredAt: now,
		}); err != nil {
			return err
		}
		if err := s.events.Publish(ctx, cmd.TallerID, shared.Event{
			Type: "customers.cliente_creado", AggregateID: c.ID, OccurredAt: now,
		}); err != nil {
			return err
		}
		res = RegisterResult{Customer: c}
		return nil
	})
	return res, err
}

// ByPhone busca un cliente por celular (HU-09, criterio 1).
func (s *Service) ByPhone(ctx context.Context, tallerID shared.ID, rawPhone string) (domain.Customer, error) {
	phone, err := shared.ParseColombianMobile(rawPhone)
	if err != nil {
		return domain.Customer{}, shared.ErrPhoneInvalid.WithField("celular")
	}
	var c domain.Customer
	err = s.uow.Do(ctx, func(ctx context.Context) error {
		c, err = s.repo.ByPhone(ctx, tallerID, phone)
		return err
	})
	return c, err
}

// ByID devuelve un cliente del taller.
func (s *Service) ByID(ctx context.Context, tallerID, id shared.ID) (domain.Customer, error) {
	var c domain.Customer
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		c, err = s.repo.ByID(ctx, tallerID, id)
		return err
	})
	return c, err
}

// ByIDs devuelve varios clientes (para las listas de otros módulos, sin N+1).
func (s *Service) ByIDs(ctx context.Context, tallerID shared.ID, ids []shared.ID) ([]domain.Customer, error) {
	var out []domain.Customer
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.repo.ByIDs(ctx, tallerID, ids)
		return err
	})
	return out, err
}

// auditView es lo que queda en la bitácora: sin el celular completo (minimización, R-10).
func auditView(c domain.Customer) map[string]any {
	return map[string]any{
		"nombre":              c.Name,
		"celular_final":       c.Phone[len(c.Phone)-4:],
		"autorizacion_estado": string(c.Consent),
	}
}
