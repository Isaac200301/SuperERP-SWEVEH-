package db

import (
	"context"

	"github.com/jackc/pgx/v5"

	"sweveh/internal/shared"
)

// One ejecuta una consulta de una fila y la mapea a T por nombre de columna (etiquetas db).
// El SQL siempre es parametrizado: nunca se concatena entrada del usuario (R-05).
func One[T any](ctx context.Context, notFound *shared.Error, sql string, args ...any) (T, error) {
	var zero T
	c, err := Conn(ctx)
	if err != nil {
		return zero, err
	}
	rows, err := c.Query(ctx, sql, args...)
	if err != nil {
		return zero, err
	}
	v, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByNameLax[T])
	if err != nil {
		return zero, NotFoundIfNoRows(err, notFound)
	}
	return v, nil
}

// Many ejecuta una consulta y mapea todas las filas a T.
func Many[T any](ctx context.Context, sql string, args ...any) ([]T, error) {
	c, err := Conn(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := c.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByNameLax[T])
}

// Exec ejecuta una sentencia y devuelve las filas afectadas.
func Exec(ctx context.Context, sql string, args ...any) (int64, error) {
	c, err := Conn(ctx)
	if err != nil {
		return 0, err
	}
	tag, err := c.Exec(ctx, sql, args...)
	return tag.RowsAffected(), err
}

// Scalar devuelve un único valor.
func Scalar[T any](ctx context.Context, sql string, args ...any) (T, error) {
	var v T
	c, err := Conn(ctx)
	if err != nil {
		return v, err
	}
	err = c.QueryRow(ctx, sql, args...).Scan(&v)
	return v, err
}
