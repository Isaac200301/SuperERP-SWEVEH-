// Package directory adapta las fachadas de vehicles y customers a los puertos de orders.
// Es el único lugar de orders que conoce otros módulos, y solo por su fachada pública.
package directory

import (
	"context"

	"sweveh/internal/customers"
	"sweveh/internal/orders/ports"
	"sweveh/internal/shared"
	"sweveh/internal/vehicles"
)

type Vehicles struct{ d vehicles.Directory }

func NewVehicles(d vehicles.Directory) Vehicles { return Vehicles{d: d} }

func (v Vehicles) ByID(ctx context.Context, tallerID, id shared.ID) (ports.Vehicle, error) {
	s, err := v.d.ByID(ctx, tallerID, id)
	return toVehicle(s), err
}

func (v Vehicles) ByPlate(ctx context.Context, tallerID shared.ID, plate string) (ports.Vehicle, error) {
	s, err := v.d.ByPlate(ctx, tallerID, plate)
	return toVehicle(s), err
}

func (v Vehicles) ByIDs(ctx context.Context, tallerID shared.ID, ids []shared.ID) (map[shared.ID]ports.Vehicle, error) {
	m, err := v.d.SnapshotsByIDs(ctx, tallerID, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[shared.ID]ports.Vehicle, len(m))
	for k, s := range m {
		out[k] = toVehicle(s)
	}
	return out, nil
}

func toVehicle(s vehicles.Snapshot) ports.Vehicle {
	return ports.Vehicle{
		ID: s.ID, CustomerID: s.CustomerID, Plate: s.Plate, Kind: s.Kind,
		Brand: s.Brand, Line: s.Line, Year: s.Year, EnginePending: s.EnginePending,
	}
}

type Customers struct{ d customers.Directory }

func NewCustomers(d customers.Directory) Customers { return Customers{d: d} }

func (c Customers) ByIDs(ctx context.Context, tallerID shared.ID, ids []shared.ID) (map[shared.ID]ports.Customer, error) {
	m, err := c.d.SummariesByIDs(ctx, tallerID, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[shared.ID]ports.Customer, len(m))
	for k, s := range m {
		out[k] = ports.Customer{ID: s.ID, Name: s.Name}
	}
	return out, nil
}
