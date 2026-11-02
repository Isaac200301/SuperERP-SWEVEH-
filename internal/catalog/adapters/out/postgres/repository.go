// Package postgres guarda los servicios del taller.
package postgres

import (
	"context"

	"sweveh/internal/catalog/domain"
	"sweveh/internal/platform/db"
	"sweveh/internal/shared"
)

type Repository struct{}

type row struct {
	ID          shared.ID `db:"id"`
	TallerID    shared.ID `db:"taller_id"`
	Nombre      string    `db:"nombre"`
	Tipo        string    `db:"tipo_vehiculo"`
	Precio      *int64    `db:"precio"`
	DuracionMin *int      `db:"duracion_min"`
	Activo      bool      `db:"activo"`
	Version     int       `db:"version"`
}

func (r row) domain() domain.Service {
	return domain.Service{ID: r.ID, TallerID: r.TallerID, Name: r.Nombre, Kind: r.Tipo, Price: r.Precio,
		DurationMin: r.DuracionMin, Active: r.Activo, Version: r.Version}
}

const cols = `id, taller_id, nombre, tipo_vehiculo, precio, duracion_min, activo, version`

func (Repository) List(ctx context.Context, tallerID shared.ID, all bool) ([]domain.Service, error) {
	rows, err := db.Many[row](ctx, `SELECT `+cols+` FROM catalog.servicio WHERE taller_id = $1 AND (activo OR $2) ORDER BY nombre`, tallerID, all)
	out := make([]domain.Service, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.domain())
	}
	return out, err
}

func (Repository) Get(ctx context.Context, tallerID, id shared.ID) (domain.Service, error) {
	r, err := db.One[row](ctx, domain.ErrNotFound, `SELECT `+cols+` FROM catalog.servicio WHERE taller_id = $1 AND id = $2`, tallerID, id)
	return r.domain(), err
}

func (Repository) Insert(ctx context.Context, s domain.Service) error {
	_, err := db.Exec(ctx, `INSERT INTO catalog.servicio (id, taller_id, nombre, tipo_vehiculo, precio, duracion_min, activo)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, s.ID, s.TallerID, s.Name, s.Kind, s.Price, s.DurationMin, s.Active)
	return err
}

func (Repository) Update(ctx context.Context, s domain.Service, prev int) error {
	n, err := db.Exec(ctx, `UPDATE catalog.servicio SET nombre = $3, tipo_vehiculo = $4, precio = $5, duracion_min = $6,
		activo = $7, version = version + 1, updated_at = now() WHERE taller_id = $1 AND id = $2 AND version = $8`,
		s.TallerID, s.ID, s.Name, s.Kind, s.Price, s.DurationMin, s.Active, prev)
	if err == nil && n == 0 {
		return shared.ErrVersionConflict
	}
	return err
}

func (Repository) Delete(ctx context.Context, tallerID, id shared.ID) (bool, error) {
	n, err := db.Exec(ctx, `DELETE FROM catalog.servicio WHERE taller_id = $1 AND id = $2`, tallerID, id)
	return n == 1, err
}
