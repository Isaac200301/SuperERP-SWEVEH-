// Package app contiene los casos de uso de catalog.
package app

import (
	"context"
	"strings"
	"time"

	"sweveh/internal/audit"
	"sweveh/internal/catalog/domain"
	"sweveh/internal/shared"
)

type Repository interface {
	List(ctx context.Context, tallerID shared.ID, includeInactive bool) ([]domain.Service, error)
	Get(ctx context.Context, tallerID, id shared.ID) (domain.Service, error)
	Insert(ctx context.Context, s domain.Service) error
	Update(ctx context.Context, s domain.Service, prevVersion int) error
	Delete(ctx context.Context, tallerID, id shared.ID) (bool, error)
}

type UnitOfWork interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

type AuditLog interface {
	Record(ctx context.Context, e audit.Entry) error
}

type Clock interface{ Now() time.Time }

type Service struct {
	uow   UnitOfWork
	repo  Repository
	audit AuditLog
	clock Clock
}

func NewService(uow UnitOfWork, r Repository, a AuditLog, c Clock) *Service {
	return &Service{uow: uow, repo: r, audit: a, clock: c}
}

func (s *Service) List(ctx context.Context, tallerID shared.ID, all bool) ([]domain.Service, error) {
	var out []domain.Service
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.repo.List(ctx, tallerID, all)
		return err
	})
	return out, err
}

func (s *Service) Get(ctx context.Context, tallerID, id shared.ID) (domain.Service, error) {
	var out domain.Service
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.repo.Get(ctx, tallerID, id)
		return err
	})
	return out, err
}

// Create agrega un servicio (HU-04, criterio 4).
func (s *Service) Create(ctx context.Context, actor shared.Actor, in domain.Service) (domain.Service, error) {
	in.ID, in.Active, in.Version = shared.NewID(), true, 1
	if err := in.Validate(); err != nil {
		return domain.Service{}, err
	}
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		if err := s.repo.Insert(ctx, in); err != nil {
			return err
		}
		return s.audit.Record(ctx, audit.Entry{TallerID: in.TallerID, Actor: actor, Action: "servicio.crear",
			Entity: "servicio", EntityID: in.ID, After: view(in), OccurredAt: s.clock.Now()})
	})
	return in, err
}

// Patch son los cambios permitidos de un servicio.
type Patch struct {
	Name        *string
	Kind        *string
	Price       **int64
	DurationMin **int
	Active      *bool
	Category    **string
	WarrantyDay **int
}

// Update cambia nombre, tipo, precio, duración o estado (HU-07). Los precios ya cotizados no cambian:
// la cotización guarda su propio precio (HU-07, criterio 3).
func (s *Service) Update(ctx context.Context, actor shared.Actor, tallerID, id shared.ID, version int, p Patch) (domain.Service, error) {
	var out domain.Service
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		cur, err := s.repo.Get(ctx, tallerID, id)
		if err != nil {
			return err
		}
		if cur.Version != version {
			return shared.ErrVersionConflict
		}
		next := cur
		if p.Name != nil {
			next.Name = *p.Name
		}
		if p.Kind != nil {
			next.Kind = *p.Kind
		}
		if p.Price != nil {
			next.Price = *p.Price
		}
		if p.DurationMin != nil {
			next.DurationMin = *p.DurationMin
		}
		if p.Active != nil {
			next.Active = *p.Active
		}
		if p.Category != nil {
			next.Category = *p.Category
		}
		if p.WarrantyDay != nil {
			next.WarrantyDay = *p.WarrantyDay
		}
		if err := next.Validate(); err != nil {
			return err
		}
		if err := s.repo.Update(ctx, next, version); err != nil {
			return err
		}
		next.Version++
		out = next
		return s.audit.Record(ctx, audit.Entry{TallerID: tallerID, Actor: actor, Action: "servicio.cambiar",
			Entity: "servicio", EntityID: id, Before: view(cur), After: view(next), OccurredAt: s.clock.Now()})
	})
	return out, err
}

// Remove borra un servicio. Si ya se usó en cotizaciones igual se puede: la cotización guarda la descripción.
func (s *Service) Remove(ctx context.Context, actor shared.Actor, tallerID, id shared.ID) error {
	return s.uow.Do(ctx, func(ctx context.Context) error {
		cur, err := s.repo.Get(ctx, tallerID, id)
		if err != nil {
			return err
		}
		if _, err := s.repo.Delete(ctx, tallerID, id); err != nil {
			return err
		}
		return s.audit.Record(ctx, audit.Entry{TallerID: tallerID, Actor: actor, Action: "servicio.borrar",
			Entity: "servicio", EntityID: id, Before: view(cur), OccurredAt: s.clock.Now()})
	})
}

// Preload inserta la precarga en una transacción ya abierta (la llama tenancy al terminar el paso 2).
// Idempotente: solo agrega los nombres que el taller no tiene, así repetir el paso 2 con una especialidad
// nueva suma sus servicios sin duplicar los que ya estaban (ni los renombrados por el taller).
func (s *Service) Preload(ctx context.Context, tallerID shared.ID, attends string, specialties []string) error {
	existing, err := s.repo.List(ctx, tallerID, true)
	if err != nil {
		return err
	}
	have := map[string]bool{}
	for _, e := range existing {
		have[strings.ToLower(e.Name)] = true
	}
	for _, sv := range domain.Preload(attends, specialties) {
		if have[strings.ToLower(sv.Name)] {
			continue
		}
		sv.ID, sv.TallerID = shared.NewID(), tallerID
		if err := s.repo.Insert(ctx, sv); err != nil {
			return err
		}
	}
	return nil
}

func view(s domain.Service) map[string]any {
	return map[string]any{"nombre": s.Name, "tipo": s.Kind, "precio": s.Price, "duracion_min": s.DurationMin, "activo": s.Active,
		"categoria": s.Category, "garantia_dias": s.WarrantyDay}
}
