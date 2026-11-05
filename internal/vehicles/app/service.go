// Package app contiene los casos de uso de vehicles (HU-10, HU-11).
package app

import (
	"context"
	"errors"

	"sweveh/internal/audit"
	"sweveh/internal/shared"
	"sweveh/internal/vehicles/domain"
	"sweveh/internal/vehicles/ports"
)

var ErrCustomerRequired = shared.Validation("cliente_requerido",
	"Elige o registra primero al cliente dueño del vehículo.").WithField("clienteId")

type Service struct {
	uow       ports.UnitOfWork
	repo      ports.VehicleRepository
	customers ports.CustomerChecker
	audit     ports.AuditLog
	events    ports.EventPublisher
	clock     ports.Clock
	extra     Extra
	registrar CustomerRegistrar // importación de clientes y vehículos (HU-92)
}

func NewService(uow ports.UnitOfWork, repo ports.VehicleRepository, cc ports.CustomerChecker,
	a ports.AuditLog, e ports.EventPublisher, c ports.Clock) *Service {
	return &Service{uow: uow, repo: repo, customers: cc, audit: a, events: e, clock: c}
}

// RegisterVehicle es el comando de HU-10/HU-11: buscar por placa o crear.
type RegisterVehicle struct {
	TallerID       shared.ID
	Actor          shared.Actor
	Plate          string
	Kind           string
	ConfirmSpecial bool
	CustomerID     shared.ID
	Details        domain.Details
	Engine         domain.Engine
}

type RegisterResult struct {
	Vehicle  domain.Vehicle
	Existing bool
}

// Register abre el vehículo si la placa ya existe en el taller; si no, lo crea (HU-10).
func (s *Service) Register(ctx context.Context, cmd RegisterVehicle) (RegisterResult, error) {
	plate, err := domain.ParsePlate(cmd.Plate)
	if err != nil {
		return RegisterResult{}, err
	}
	var res RegisterResult
	err = s.uow.Do(ctx, func(ctx context.Context) error {
		existing, err := s.repo.ByPlate(ctx, cmd.TallerID, plate.String())
		if err == nil {
			res = RegisterResult{Vehicle: existing, Existing: true}
			return nil
		}
		if !errors.Is(err, domain.ErrVehicleNotFound) {
			return err
		}
		if cmd.CustomerID == shared.NilID {
			return ErrCustomerRequired
		}
		ok, err := s.customers.Exists(ctx, cmd.TallerID, cmd.CustomerID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrCustomerRequired
		}
		now := s.clock.Now()
		v, err := domain.NewVehicle(shared.NewID(), cmd.TallerID, domain.NewVehicleInput{
			Plate: plate, Kind: cmd.Kind, ConfirmSpecial: cmd.ConfirmSpecial,
			CustomerID: cmd.CustomerID, Details: cmd.Details, Engine: cmd.Engine,
		}, now)
		if err != nil {
			return err
		}
		inserted, err := s.repo.Insert(ctx, v, cmd.Actor.ID)
		if err != nil {
			return err
		}
		if !inserted {
			existing, err := s.repo.ByPlate(ctx, cmd.TallerID, plate.String())
			if err != nil {
				return err
			}
			res = RegisterResult{Vehicle: existing, Existing: true}
			return nil
		}
		if err := s.audit.Record(ctx, audit.Entry{
			TallerID: cmd.TallerID, Actor: cmd.Actor, Action: "vehiculo.crear", Entity: "vehiculo",
			EntityID: v.ID, After: auditView(v), OccurredAt: now,
		}); err != nil {
			return err
		}
		if err := s.events.Publish(ctx, cmd.TallerID, shared.Event{
			Type: "vehicles.vehiculo_creado", AggregateID: v.ID, OccurredAt: now,
			Payload: map[string]any{"placa": v.Plate.String(), "tipo": string(v.Kind)},
		}); err != nil {
			return err
		}
		res = RegisterResult{Vehicle: v}
		return nil
	})
	return res, err
}

// UpdateEngine es el comando de HU-11: completar o cambiar la ficha, con bitácora del valor anterior.
type UpdateEngine struct {
	TallerID        shared.ID
	Actor           shared.Actor
	VehicleID       shared.ID
	Engine          domain.Engine
	Details         domain.Details // solo color, carrocería y llanta
	ExpectedVersion int
}

func (s *Service) UpdateEngine(ctx context.Context, cmd UpdateEngine) (domain.Vehicle, error) {
	var out domain.Vehicle
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		v, err := s.repo.ByID(ctx, cmd.TallerID, cmd.VehicleID)
		if err != nil {
			return err
		}
		prevVersion := v.Version
		prev, err := v.UpdateEngine(cmd.Engine, cmd.Details, cmd.ExpectedVersion)
		if err != nil {
			return err
		}
		now := s.clock.Now()
		if err := s.repo.UpdateEngine(ctx, v, prevVersion, now); err != nil {
			return err
		}
		if err := s.audit.Record(ctx, audit.Entry{
			TallerID: cmd.TallerID, Actor: cmd.Actor, Action: "vehiculo.motor.cambiar", Entity: "vehiculo",
			EntityID: v.ID, Before: engineView(prev), After: sheetView(v), OccurredAt: now,
		}); err != nil {
			return err
		}
		if err := s.events.Publish(ctx, cmd.TallerID, shared.Event{
			Type: "vehicles.motor_cambiado", AggregateID: v.ID, OccurredAt: now,
		}); err != nil {
			return err
		}
		out = v
		return nil
	})
	return out, err
}

// ByPlate busca un vehículo por placa normalizada.
func (s *Service) ByPlate(ctx context.Context, tallerID shared.ID, raw string) (domain.Vehicle, error) {
	plate := domain.NormalizePlate(raw)
	var v domain.Vehicle
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		v, err = s.repo.ByPlate(ctx, tallerID, plate)
		return err
	})
	return v, err
}

// ByID devuelve un vehículo del taller.
func (s *Service) ByID(ctx context.Context, tallerID, id shared.ID) (domain.Vehicle, error) {
	var v domain.Vehicle
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		v, err = s.repo.ByID(ctx, tallerID, id)
		return err
	})
	return v, err
}

// ByIDs devuelve varios vehículos (listas de otros módulos).
func (s *Service) ByIDs(ctx context.Context, tallerID shared.ID, ids []shared.ID) ([]domain.Vehicle, error) {
	var out []domain.Vehicle
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.repo.ByIDs(ctx, tallerID, ids)
		return err
	})
	return out, err
}

func auditView(v domain.Vehicle) map[string]any {
	m := map[string]any{"placa": v.Plate.String(), "tipo": string(v.Kind), "formato_placa": string(v.Plate.Format())}
	for k, val := range engineView(v.Engine) {
		m[k] = val
	}
	return m
}

func engineView(e domain.Engine) map[string]any {
	return map[string]any{"cilindraje_cc": e.DisplacementCC, "turbo": e.Turbo, "energia": e.Energy, "aspiracion": e.Aspiration,
		"transmision": e.Transmission, "traccion": e.Drive, "ciclo": e.Cycle, "refrigeracion": e.Cooling}
}

func sheetView(v domain.Vehicle) map[string]any {
	m := engineView(v.Engine)
	m["color"], m["carroceria"], m["llanta"] = v.Details.Color, v.Details.Body, v.Details.Tire
	return m
}
