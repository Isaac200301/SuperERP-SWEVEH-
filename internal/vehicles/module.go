// Package vehicles es la fachada pública del módulo de vehículos.
package vehicles

import (
	"context"

	"sweveh/internal/shared"
	"sweveh/internal/vehicles/app"
	"sweveh/internal/vehicles/domain"
)

// Snapshot es lo que otros módulos ven de un vehículo.
type Snapshot struct {
	ID            shared.ID
	CustomerID    shared.ID
	Plate         string
	Kind          string
	Brand         *string
	Line          *string
	Year          *int
	EnginePending bool
}

// Directory es la fachada de lectura para otros módulos.
type Directory struct{ svc *app.Service }

func NewDirectory(svc *app.Service) Directory { return Directory{svc: svc} }

// ByID devuelve el vehículo del taller o el error NotFound del módulo.
func (d Directory) ByID(ctx context.Context, tallerID, id shared.ID) (Snapshot, error) {
	v, err := d.svc.ByID(ctx, tallerID, id)
	if err != nil {
		return Snapshot{}, err
	}
	return snapshot(v), nil
}

// ByPlate devuelve el vehículo con esa placa en el taller.
func (d Directory) ByPlate(ctx context.Context, tallerID shared.ID, plate string) (Snapshot, error) {
	v, err := d.svc.ByPlate(ctx, tallerID, plate)
	if err != nil {
		return Snapshot{}, err
	}
	return snapshot(v), nil
}

// SnapshotsByIDs devuelve un mapa id → vehículo para listas.
func (d Directory) SnapshotsByIDs(ctx context.Context, tallerID shared.ID, ids []shared.ID) (map[shared.ID]Snapshot, error) {
	list, err := d.svc.ByIDs(ctx, tallerID, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[shared.ID]Snapshot, len(list))
	for _, v := range list {
		out[v.ID] = snapshot(v)
	}
	return out, nil
}

// SearchPlate busca por parte de la placa (HU-14).
func (d Directory) SearchPlate(ctx context.Context, tallerID shared.ID, q string) ([]Snapshot, error) {
	list, err := d.svc.SearchPlate(ctx, tallerID, q)
	return snapshots(list), err
}

// ByCustomer lista los vehículos de un cliente (HU-12, HU-57).
func (d Directory) ByCustomer(ctx context.Context, tallerID, customerID shared.ID) ([]Snapshot, error) {
	list, err := d.svc.ByCustomer(ctx, tallerID, customerID)
	return snapshots(list), err
}

func snapshots(list []domain.Vehicle) []Snapshot {
	out := make([]Snapshot, 0, len(list))
	for _, v := range list {
		out = append(out, snapshot(v))
	}
	return out
}

func snapshot(v domain.Vehicle) Snapshot {
	return Snapshot{
		ID: v.ID, CustomerID: v.CustomerID, Plate: v.Plate.String(), Kind: string(v.Kind),
		Brand: v.Details.Brand, Line: v.Details.Line, Year: v.Details.Year,
		EnginePending: v.Engine.Pending(v.Kind),
	}
}

// MoveCustomer pasa los vehículos de un cliente a otro al unir clientes repetidos (HU-15).
func (d Directory) MoveCustomer(ctx context.Context, tallerID, from, to shared.ID) (int, error) {
	return d.svc.MoveCustomer(ctx, tallerID, from, to)
}

// AnonymizeCustomer cambia la placa de los vehículos del cliente por una que no lo identifica (HU-87).
func (d Directory) AnonymizeCustomer(ctx context.Context, tallerID, customerID shared.ID) (map[shared.ID]string, error) {
	return d.svc.AnonymizeCustomer(ctx, tallerID, customerID)
}
