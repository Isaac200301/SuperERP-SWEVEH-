// Package app contiene los casos de uso de orders.
package app

import (
	"context"

	"sweveh/internal/audit"
	"sweveh/internal/orders/domain"
	"sweveh/internal/orders/ports"
	"sweveh/internal/shared"
)

type Service struct {
	uow       ports.UnitOfWork
	repo      ports.OrderRepository
	vehicles  ports.VehicleDirectory
	customers ports.CustomerDirectory
	audit     ports.AuditLog
	events    ports.EventPublisher
	clock     ports.Clock

	markExample func(ctx context.Context, tallerID, id shared.ID) error
}

func NewService(uow ports.UnitOfWork, repo ports.OrderRepository, v ports.VehicleDirectory,
	c ports.CustomerDirectory, a ports.AuditLog, e ports.EventPublisher, clk ports.Clock) *Service {
	return &Service{uow: uow, repo: repo, vehicles: v, customers: c, audit: a, events: e, clock: clk}
}

// OpenOrder es el comando de HU-16: abrir una orden desde la placa.
type OpenOrder struct {
	TallerID         shared.ID
	Actor            shared.Actor
	VehicleID        shared.ID // si viene, manda sobre la placa
	Plate            string
	Symptom          string
	InitialState     domain.State
	ConfirmDuplicate bool
	Example          bool // orden de ejemplo del registro (HU-05)
}

// SetExampleMarker conecta el repositorio que marca órdenes de ejemplo.
func (s *Service) SetExampleMarker(f func(ctx context.Context, tallerID, id shared.ID) error) {
	s.markExample = f
}

// Open crea la orden con su número, historial, auditoría y evento en una sola transacción.
func (s *Service) Open(ctx context.Context, cmd OpenOrder) (domain.Order, error) {
	var out domain.Order
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		v, err := s.vehicle(ctx, cmd)
		if err != nil {
			return err
		}
		open, err := s.repo.CountOpenByVehicle(ctx, cmd.TallerID, v.ID)
		if err != nil {
			return err
		}
		in := domain.NewOrderInput{
			Vehicle: domain.VehicleSnapshot{
				ID: v.ID, CustomerID: v.CustomerID, Plate: v.Plate, Kind: v.Kind, EnginePending: v.EnginePending,
			},
			Symptom: cmd.Symptom, InitialState: cmd.InitialState,
			OpenOrdersOfVehicle: open, ConfirmDuplicate: cmd.ConfirmDuplicate, CreatedBy: cmd.Actor.ID,
		}
		// Validar antes de pedir número: una orden rechazada no gasta consecutivo.
		if err := domain.ValidateOpen(in); err != nil {
			return err
		}
		number, err := s.repo.NextNumber(ctx, cmd.TallerID)
		if err != nil {
			return err
		}
		now := s.clock.Now()
		o, err := domain.NewOrder(shared.NewID(), cmd.TallerID, number, in, now)
		if err != nil {
			return err
		}
		if err := s.repo.Insert(ctx, o); err != nil {
			return err
		}
		if err := s.repo.InsertHistory(ctx, o, nil, cmd.Actor.ID, now); err != nil {
			return err
		}
		if cmd.Example {
			if err := s.markExample(ctx, cmd.TallerID, o.ID); err != nil {
				return err
			}
			o.Example = true
		}
		if err := s.audit.Record(ctx, audit.Entry{
			TallerID: cmd.TallerID, Actor: cmd.Actor, Action: "orden.abrir", Entity: "orden", EntityID: o.ID,
			After:      map[string]any{"numero": o.Code(), "placa": o.Plate, "estado": string(o.State)},
			OccurredAt: now,
		}); err != nil {
			return err
		}
		if err := s.events.Publish(ctx, cmd.TallerID, shared.Event{
			Type: "orders.orden_abierta", AggregateID: o.ID, OccurredAt: now,
			Payload: map[string]any{"numero": o.Number, "estado": string(o.State), "vehiculo_id": o.VehicleID.String()},
		}); err != nil {
			return err
		}
		out = o
		return nil
	})
	return out, err
}

func (s *Service) vehicle(ctx context.Context, cmd OpenOrder) (ports.Vehicle, error) {
	if cmd.VehicleID != shared.NilID {
		return s.vehicles.ByID(ctx, cmd.TallerID, cmd.VehicleID)
	}
	return s.vehicles.ByPlate(ctx, cmd.TallerID, cmd.Plate)
}

// OrderView es una orden con los datos de cliente y vehículo para mostrar.
type OrderView struct {
	Order    domain.Order
	Customer *ports.Customer
	Vehicle  *ports.Vehicle
	History  []ports.HistoryEntry
}

// Recent lista las órdenes más recientes (panel, HU-23 más adelante).

// decorate agrega cliente y vehículo pidiéndolos por lotes a sus fachadas (sin N+1).
func (s *Service) decorate(ctx context.Context, tallerID shared.ID, list []domain.Order) ([]OrderView, error) {
	vIDs := make([]shared.ID, 0, len(list))
	cIDs := make([]shared.ID, 0, len(list))
	for _, o := range list {
		vIDs = append(vIDs, o.VehicleID)
		cIDs = append(cIDs, o.CustomerID)
	}
	vs, err := s.vehicles.ByIDs(ctx, tallerID, vIDs)
	if err != nil {
		return nil, err
	}
	cs, err := s.customers.ByIDs(ctx, tallerID, cIDs)
	if err != nil {
		return nil, err
	}
	out := make([]OrderView, 0, len(list))
	for _, o := range list {
		view := OrderView{Order: o}
		if v, ok := vs[o.VehicleID]; ok {
			view.Vehicle = &v
		}
		if c, ok := cs[o.CustomerID]; ok {
			view.Customer = &c
		}
		out = append(out, view)
	}
	return out, nil
}

// Get devuelve una orden completa con su historial de estados.
func (f *Flow) Get(ctx context.Context, tallerID, id shared.ID) (OrderView, error) {
	var out OrderView
	err := f.uow.Do(ctx, func(ctx context.Context) error {
		o, err := f.repo.Load(ctx, tallerID, id)
		if err != nil {
			return err
		}
		h, err := f.base.repo.History(ctx, tallerID, id)
		if err != nil {
			return err
		}
		views, err := f.base.decorate(ctx, tallerID, []domain.Order{o})
		if err != nil {
			return err
		}
		out = views[0]
		out.History = h
		return nil
	})
	return out, err
}
