// Package db da el pool de Postgres y la unidad de trabajo: toda operación de
// negocio corre dentro de una transacción que fija app.taller_id con
// set_config(..., true), el equivalente parametrizado de SET LOCAL (R-06).
package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"sweveh/internal/platform/reqctx"
	"sweveh/internal/shared"
)

// DBTX es lo que necesitan las consultas generadas por sqlc.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// ErrNoTenant: se intentó tocar datos de negocio sin taller en el context. Falla cerrado.
var ErrNoTenant = errors.New("db: operación sin taller en el contexto")

// ErrNoTx: un repositorio se usó fuera de una unidad de trabajo.
var ErrNoTx = errors.New("db: consulta fuera de una transacción")

// PoolConfig son los valores "a calibrar" de docs/06 §5.4; viven en configuración.
type PoolConfig struct {
	URL             string
	MaxConns        int32
	MaxConnLifetime time.Duration
	// SimpleProtocol se activa si se usa el pooler en modo transacción (puerto 6543).
	SimpleProtocol bool
}

// NewPool abre el pool y verifica la conexión.
func NewPool(ctx context.Context, c PoolConfig) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(c.URL)
	if err != nil {
		return nil, fmt.Errorf("db: url inválida: %w", err)
	}
	if c.MaxConns > 0 {
		cfg.MaxConns = c.MaxConns
	}
	if c.MaxConnLifetime > 0 {
		cfg.MaxConnLifetime = c.MaxConnLifetime
	}
	if c.SimpleProtocol {
		cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("db: no se pudo crear el pool: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: no responde: %w", err)
	}
	return pool, nil
}

type keyTx struct{}

// TxManager abre transacciones con el taller fijado.
type TxManager struct {
	pool *pgxpool.Pool
}

func NewTxManager(pool *pgxpool.Pool) *TxManager { return &TxManager{pool: pool} }

// Do ejecuta fn en una transacción con app.taller_id = taller del context.
// Si ya hay una transacción en el context, se une a ella (una sola transacción
// por caso de uso aunque llame fachadas de otros módulos).
func (m *TxManager) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(keyTx{}).(pgx.Tx); ok {
		return fn(ctx)
	}
	tid, ok := reqctx.TenantFrom(ctx)
	if !ok {
		return ErrNoTenant
	}
	return m.run(ctx, map[string]string{"app.taller_id": tid.String()}, false, fn)
}

// DoWithSettings es para los pocos flujos que corren antes de conocer el taller
// (inicio de sesión: app.usuario_id). Cada uso debe estar justificado en su módulo.
func (m *TxManager) DoWithSettings(ctx context.Context, settings map[string]string, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(keyTx{}).(pgx.Tx); ok {
		return errors.New("db: DoWithSettings no se puede anidar")
	}
	return m.run(ctx, settings, true, fn)
}

func (m *TxManager) run(ctx context.Context, settings map[string]string, settable bool, fn func(ctx context.Context) error) error {
	return pgx.BeginFunc(ctx, m.pool, func(tx pgx.Tx) error {
		for k, v := range settings {
			if err := setLocal(ctx, tx, k, v); err != nil {
				return err
			}
		}
		return fn(context.WithValue(context.WithValue(ctx, keyTx{}, tx), keySettable{}, settable))
	})
}

type keySettable struct{}

// ErrNotSettable: SetLocal solo se permite dentro de DoWithSettings, nunca en una
// transacción de negocio abierta con Do (allí el taller lo fija la sesión y no cambia).
var ErrNotSettable = errors.New("db: SetLocal fuera de DoWithSettings")

// SetLocal fija una variable de la transacción actual. Lo usa identity cuando conoce
// el taller a mitad del inicio de sesión (después de leer la membresía o el refresh).
func (m *TxManager) SetLocal(ctx context.Context, name, value string) error {
	tx, ok := ctx.Value(keyTx{}).(pgx.Tx)
	if !ok {
		return ErrNoTx
	}
	if settable, _ := ctx.Value(keySettable{}).(bool); !settable {
		return ErrNotSettable
	}
	return setLocal(ctx, tx, name, value)
}

func setLocal(ctx context.Context, tx pgx.Tx, name, value string) error {
	// set_config(nombre, valor, true) = SET LOCAL: muere al terminar la transacción.
	if _, err := tx.Exec(ctx, "SELECT set_config($1, $2, true)", name, value); err != nil {
		return fmt.Errorf("db: fijar %s: %w", name, err)
	}
	return nil
}

// Conn devuelve la transacción activa del context.
func Conn(ctx context.Context) (DBTX, error) {
	tx, ok := ctx.Value(keyTx{}).(pgx.Tx)
	if !ok {
		return nil, ErrNoTx
	}
	return tx, nil
}

// IsUniqueViolation indica si el error es una violación de UNIQUE (23505) sobre la restricción dada.
func IsUniqueViolation(err error, constraint string) bool {
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || pg.Code != "23505" {
		return false
	}
	return constraint == "" || pg.ConstraintName == constraint
}

// NotFoundIfNoRows convierte pgx.ErrNoRows en el error de dominio indicado.
func NotFoundIfNoRows(err error, notFound *shared.Error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return notFound
	}
	return err
}
