// Package postgres guarda los talleres.
package postgres

import (
	"context"

	"sweveh/internal/platform/db"
	"sweveh/internal/shared"
	"sweveh/internal/tenancy/adapters/out/postgres/sqlcgen"
	"sweveh/internal/tenancy/domain"
)

type Repository struct{}

func NewRepository() Repository { return Repository{} }

func (Repository) Insert(ctx context.Context, w domain.Workshop) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	return sqlcgen.New(conn).InsertWorkshop(ctx, sqlcgen.InsertWorkshopParams{ID: w.ID, Nombre: w.Name, CreatedAt: w.CreatedAt})
}

func (Repository) ByID(ctx context.Context, id shared.ID) (domain.Workshop, error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return domain.Workshop{}, err
	}
	r, err := sqlcgen.New(conn).GetWorkshop(ctx, id)
	if err != nil {
		return domain.Workshop{}, db.NotFoundIfNoRows(err, domain.ErrWorkshopNotFound)
	}
	return domain.Workshop{ID: r.ID, Name: r.Nombre, Version: int(r.Version)}, nil
}
