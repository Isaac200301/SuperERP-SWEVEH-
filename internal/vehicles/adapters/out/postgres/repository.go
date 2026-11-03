// Package postgres implementa los puertos de vehicles con sqlc.
package postgres

import (
	"context"
	"time"

	"sweveh/internal/platform/db"
	"sweveh/internal/shared"
	"sweveh/internal/vehicles/adapters/out/postgres/sqlcgen"
	"sweveh/internal/vehicles/domain"
)

type Repository struct{}

func NewRepository() Repository { return Repository{} }

func queries(ctx context.Context) (*sqlcgen.Queries, error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	return sqlcgen.New(conn), nil
}

func (Repository) ByPlate(ctx context.Context, tallerID shared.ID, plate string) (domain.Vehicle, error) {
	q, err := queries(ctx)
	if err != nil {
		return domain.Vehicle{}, err
	}
	r, err := q.GetVehicleByPlate(ctx, sqlcgen.GetVehicleByPlateParams{TallerID: tallerID, Placa: plate})
	if err != nil {
		return domain.Vehicle{}, db.NotFoundIfNoRows(err, domain.ErrVehicleNotFound)
	}
	return toDomain(sqlcgen.GetVehicleByIDRow(r)), nil
}

func (Repository) ByID(ctx context.Context, tallerID, id shared.ID) (domain.Vehicle, error) {
	q, err := queries(ctx)
	if err != nil {
		return domain.Vehicle{}, err
	}
	r, err := q.GetVehicleByID(ctx, sqlcgen.GetVehicleByIDParams{TallerID: tallerID, ID: id})
	if err != nil {
		return domain.Vehicle{}, db.NotFoundIfNoRows(err, domain.ErrVehicleNotFound)
	}
	return toDomain(r), nil
}

func (Repository) ByIDs(ctx context.Context, tallerID shared.ID, ids []shared.ID) ([]domain.Vehicle, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	q, err := queries(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.ListVehiclesByIDs(ctx, sqlcgen.ListVehiclesByIDsParams{TallerID: tallerID, Ids: ids})
	if err != nil {
		return nil, err
	}
	out := make([]domain.Vehicle, 0, len(rows))
	for _, r := range rows {
		out = append(out, toDomain(sqlcgen.GetVehicleByIDRow(r)))
	}
	return out, nil
}

func (Repository) Insert(ctx context.Context, v domain.Vehicle, createdBy shared.ID) (bool, error) {
	q, err := queries(ctx)
	if err != nil {
		return false, err
	}
	var by *shared.ID
	if createdBy != shared.NilID {
		by = &createdBy
	}
	n, err := q.InsertVehicle(ctx, sqlcgen.InsertVehicleParams{
		ID: v.ID, TallerID: v.TallerID, ClienteID: v.CustomerID, Tipo: string(v.Kind),
		Placa: v.Plate.String(), FormatoPlaca: string(v.Plate.Format()),
		Marca: v.Details.Brand, Linea: v.Details.Line,
		ModeloAnio: toInt32(v.Details.Year), Kilometraje: toInt32(v.Details.Mileage),
		CilindrajeCc: toInt32(v.Engine.DisplacementCC), Turbo: v.Engine.Turbo,
		Energia: v.Engine.Energy, Aspiracion: v.Engine.Aspiration, Transmision: v.Engine.Transmission,
		Traccion: v.Engine.Drive, Ciclo: v.Engine.Cycle, Refrigeracion: v.Engine.Cooling,
		Color: v.Details.Color, Carroceria: v.Details.Body, Llanta: v.Details.Tire,
		CreatedAt: v.CreatedAt, CreatedBy: by,
	})
	return n == 1, err
}

func (Repository) UpdateEngine(ctx context.Context, v domain.Vehicle, prevVersion int, at time.Time) error {
	q, err := queries(ctx)
	if err != nil {
		return err
	}
	n, err := q.UpdateVehicleEngine(ctx, sqlcgen.UpdateVehicleEngineParams{
		TallerID: v.TallerID, ID: v.ID, CilindrajeCc: toInt32(v.Engine.DisplacementCC), Turbo: v.Engine.Turbo,
		Energia: v.Engine.Energy, Aspiracion: v.Engine.Aspiration, Transmision: v.Engine.Transmission,
		Traccion: v.Engine.Drive, Ciclo: v.Engine.Cycle, Refrigeracion: v.Engine.Cooling,
		Color: v.Details.Color, Carroceria: v.Details.Body, Llanta: v.Details.Tire,
		UpdatedAt: at, Version: int32(prevVersion),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return shared.ErrVersionConflict
	}
	return nil
}

func toDomain(r sqlcgen.GetVehicleByIDRow) domain.Vehicle {
	return domain.Vehicle{
		ID: r.ID, TallerID: r.TallerID, CustomerID: r.ClienteID, Kind: domain.Kind(r.Tipo),
		Plate: domain.RestorePlate(r.Placa, domain.PlateFormat(r.FormatoPlaca)),
		Details: domain.Details{
			Brand: r.Marca, Line: r.Linea, Year: toInt(r.ModeloAnio), Mileage: toInt(r.Kilometraje),
			Color: r.Color, Body: r.Carroceria, Tire: r.Llanta,
		},
		Engine: domain.Engine{DisplacementCC: toInt(r.CilindrajeCc), Turbo: r.Turbo, Energy: r.Energia, Aspiration: r.Aspiracion,
			Transmission: r.Transmision, Drive: r.Traccion, Cycle: r.Ciclo, Cooling: r.Refrigeracion},
		Version:   int(r.Version),
		CreatedAt: r.CreatedAt,
	}
}

func toInt32(p *int) *int32 {
	if p == nil {
		return nil
	}
	v := int32(*p)
	return &v
}

func toInt(p *int32) *int {
	if p == nil {
		return nil
	}
	v := int(*p)
	return &v
}
