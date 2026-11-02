// Package postgres guarda repuestos y movimientos.
package postgres

import (
	"context"
	"strings"

	"sweveh/internal/inventory/app"
	"sweveh/internal/inventory/domain"
	"sweveh/internal/platform/db"
	"sweveh/internal/shared"
)

type Repository struct{}

type row struct {
	ID         shared.ID `db:"id"`
	TallerID   shared.ID `db:"taller_id"`
	Referencia string    `db:"referencia"`
	Nombre     string    `db:"nombre"`
	Tipo       string    `db:"tipo_vehiculo"`
	Costo      int64     `db:"costo"`
	Precio     int64     `db:"precio_venta"`
	Existencia int       `db:"existencia"`
	Minimo     int       `db:"minimo"`
	Activo     bool      `db:"activo"`
	Version    int       `db:"version"`
}

func (r row) domain() domain.Part {
	return domain.Part{ID: r.ID, TallerID: r.TallerID, Ref: r.Referencia, Name: r.Nombre, Kind: r.Tipo, Cost: r.Costo,
		Price: r.Precio, Stock: r.Existencia, Min: r.Minimo, Active: r.Activo, Version: r.Version}
}

func list(rows []row, err error) ([]domain.Part, error) {
	out := make([]domain.Part, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.domain())
	}
	return out, err
}

const cols = `id, taller_id, referencia, nombre, tipo_vehiculo, costo, precio_venta, existencia, minimo, activo, version`

// escapeLike evita que % y _ del usuario actúen como comodines.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func (Repository) Search(ctx context.Context, tallerID shared.ID, q string, onlyActive bool, limit int) ([]domain.Part, error) {
	pat := "%" + escapeLike(strings.TrimSpace(q)) + "%"
	return list(db.Many[row](ctx, `SELECT `+cols+` FROM inventory.articulo
		WHERE taller_id = $1 AND (activo OR NOT $2) AND (nombre ILIKE $3 OR referencia ILIKE $3)
		ORDER BY nombre LIMIT $4`, tallerID, onlyActive, pat, limit))
}

func (Repository) Get(ctx context.Context, tallerID, id shared.ID) (domain.Part, error) {
	r, err := db.One[row](ctx, domain.ErrNotFound, `SELECT `+cols+` FROM inventory.articulo WHERE taller_id = $1 AND id = $2`, tallerID, id)
	return r.domain(), err
}

func (Repository) GetMany(ctx context.Context, tallerID shared.ID, ids []shared.ID) ([]domain.Part, error) {
	return list(db.Many[row](ctx, `SELECT `+cols+` FROM inventory.articulo WHERE taller_id = $1 AND id = ANY($2)`, tallerID, ids))
}

func (Repository) Insert(ctx context.Context, p domain.Part) error {
	_, err := db.Exec(ctx, `INSERT INTO inventory.articulo (id, taller_id, referencia, nombre, tipo_vehiculo, costo, precio_venta, minimo)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, p.ID, p.TallerID, p.Ref, p.Name, p.Kind, p.Cost, p.Price, p.Min)
	if db.IsUniqueViolation(err, "articulo_referencia_unica") {
		return domain.ErrRefTaken
	}
	return err
}

func (Repository) Update(ctx context.Context, p domain.Part, prev int) error {
	n, err := db.Exec(ctx, `UPDATE inventory.articulo SET nombre = $3, tipo_vehiculo = $4, costo = $5, precio_venta = $6,
		minimo = $7, activo = $8, version = version + 1, updated_at = now() WHERE taller_id = $1 AND id = $2 AND version = $9`,
		p.TallerID, p.ID, p.Name, p.Kind, p.Cost, p.Price, p.Min, p.Active, prev)
	if err == nil && n == 0 {
		return shared.ErrVersionConflict
	}
	return err
}

// Move: el descuento es atómico (UPDATE ... WHERE existencia >= n, R-07) y el movimiento va en la misma transacción.
// Un movimiento "pendiente_orden" no toca la existencia. Repetir la misma línea no descuenta dos veces.
func (Repository) Move(ctx context.Context, m app.Movement) (bool, error) {
	if m.LineID != nil {
		dup, err := db.Scalar[bool](ctx, `SELECT EXISTS (SELECT 1 FROM inventory.movimiento WHERE linea_id = $1
			AND motivo IN ('salida_orden', 'pendiente_orden'))`, *m.LineID)
		if err != nil || dup {
			return true, err
		}
	}
	if m.Reason != "pendiente_orden" {
		n, err := db.Exec(ctx, `UPDATE inventory.articulo SET existencia = existencia + $3, updated_at = now()
			WHERE taller_id = $1 AND id = $2 AND existencia + $3 >= 0`, m.TallerID, m.PartID, m.Qty)
		if err != nil {
			return false, err
		}
		if n == 0 {
			return false, nil
		}
	}
	_, err := db.Exec(ctx, `INSERT INTO inventory.movimiento (id, taller_id, articulo_id, cantidad, motivo, orden_id, linea_id, usuario_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, shared.NewID(), m.TallerID, m.PartID, m.Qty, m.Reason, m.OrderID, m.LineID, m.UserID)
	return err == nil, err
}

func (Repository) Low(ctx context.Context, tallerID shared.ID) ([]domain.Part, error) {
	return list(db.Many[row](ctx, `SELECT `+cols+` FROM inventory.articulo WHERE taller_id = $1 AND activo AND existencia <= minimo
		ORDER BY nombre`, tallerID))
}
