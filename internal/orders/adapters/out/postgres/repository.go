// Package postgres implementa los puertos de orders con sqlc.
package postgres

import (
	"context"
	"time"

	"sweveh/internal/orders/adapters/out/postgres/sqlcgen"
	"sweveh/internal/orders/domain"
	"sweveh/internal/orders/ports"
	"sweveh/internal/platform/db"
	"sweveh/internal/shared"
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

func (Repository) NextNumber(ctx context.Context, tallerID shared.ID) (int, error) {
	q, err := queries(ctx)
	if err != nil {
		return 0, err
	}
	n, err := q.NextOrderNumber(ctx, tallerID)
	return int(n), err
}

func (Repository) CountOpenByVehicle(ctx context.Context, tallerID, vehicleID shared.ID) (int, error) {
	q, err := queries(ctx)
	if err != nil {
		return 0, err
	}
	n, err := q.CountOpenOrdersByVehicle(ctx, sqlcgen.CountOpenOrdersByVehicleParams{TallerID: tallerID, VehiculoID: vehicleID})
	return int(n), err
}

func (Repository) Insert(ctx context.Context, o domain.Order) error {
	q, err := queries(ctx)
	if err != nil {
		return err
	}
	return q.InsertOrder(ctx, sqlcgen.InsertOrderParams{
		ID: o.ID, TallerID: o.TallerID, Numero: int32(o.Number), VehiculoID: o.VehicleID,
		ClienteID: o.CustomerID, Placa: o.Plate, TipoVehiculo: o.VehicleKind, Estado: string(o.State),
		Sintoma: o.Symptom, CreatedAt: o.CreatedAt, CreatedBy: optID(o.CreatedBy),
	})
}

func (Repository) InsertHistory(ctx context.Context, o domain.Order, from *domain.State, by shared.ID, at time.Time) error {
	q, err := queries(ctx)
	if err != nil {
		return err
	}
	var desde *string
	if from != nil {
		s := string(*from)
		desde = &s
	}
	return q.InsertStateHistory(ctx, sqlcgen.InsertStateHistoryParams{
		ID: shared.NewID(), TallerID: o.TallerID, OrdenID: o.ID, Desde: desde, Hacia: string(o.State),
		UsuarioID: optID(by), OcurridoEn: at,
	})
}

func (Repository) ByID(ctx context.Context, tallerID, id shared.ID) (domain.Order, error) {
	q, err := queries(ctx)
	if err != nil {
		return domain.Order{}, err
	}
	r, err := q.GetOrder(ctx, sqlcgen.GetOrderParams{TallerID: tallerID, ID: id})
	if err != nil {
		return domain.Order{}, db.NotFoundIfNoRows(err, domain.ErrOrderNotFound)
	}
	return toDomain(r), nil
}

func (Repository) Recent(ctx context.Context, tallerID shared.ID, limit int, cursor *ports.Cursor) ([]domain.Order, error) {
	q, err := queries(ctx)
	if err != nil {
		return nil, err
	}
	p := sqlcgen.ListRecentOrdersParams{TallerID: tallerID, Limit: int32(limit)}
	if cursor != nil {
		at, id := cursor.CreatedAt, cursor.ID
		p.CursorAt, p.CursorID = &at, &id
	}
	rows, err := q.ListRecentOrders(ctx, p)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Order, 0, len(rows))
	for _, r := range rows {
		out = append(out, toDomain(sqlcgen.GetOrderRow(r)))
	}
	return out, nil
}

func (Repository) History(ctx context.Context, tallerID, id shared.ID) ([]ports.HistoryEntry, error) {
	q, err := queries(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.ListStateHistory(ctx, sqlcgen.ListStateHistoryParams{TallerID: tallerID, OrdenID: id})
	if err != nil {
		return nil, err
	}
	out := make([]ports.HistoryEntry, 0, len(rows))
	for _, r := range rows {
		e := ports.HistoryEntry{To: domain.State(r.Hacia), UserID: r.UsuarioID, OccurredAt: r.OcurridoEn}
		if r.Desde != nil {
			s := domain.State(*r.Desde)
			e.From = &s
		}
		out = append(out, e)
	}
	return out, nil
}

func toDomain(r sqlcgen.GetOrderRow) domain.Order {
	o := domain.Order{
		ID: r.ID, TallerID: r.TallerID, Number: int(r.Numero), VehicleID: r.VehiculoID, CustomerID: r.ClienteID,
		Plate: r.Placa, VehicleKind: r.TipoVehiculo, State: domain.State(r.Estado), Symptom: r.Sintoma,
		Version: int(r.Version), CreatedAt: r.CreatedAt,
	}
	if r.CreatedBy != nil {
		o.CreatedBy = *r.CreatedBy
	}
	return o
}

func optID(id shared.ID) *shared.ID {
	if id == shared.NilID {
		return nil
	}
	return &id
}
