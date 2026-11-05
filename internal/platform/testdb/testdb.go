// Package testdb da una base Postgres real y aislada para pruebas de integración:
// crea una base nueva, aplica las migraciones con sweveh_migrator y conecta como
// sweveh_api y sweveh_worker (sin BYPASSRLS), igual que en producción.
//
// Necesita SWEVEH_TEST_ADMIN_URL (superusuario del Postgres local o del servicio de CI).
// Con `go test -short` las pruebas de integración se saltan explícitamente; sin -short y
// sin la variable, FALLAN: una prueba de aislamiento no se salta en silencio (R-15).
package testdb

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"sweveh/db/migrations"
	"sweveh/internal/platform/db"
	"sweveh/internal/platform/reqctx"
	"sweveh/internal/shared"
)

// DB es una base de prueba con un pool por rol.
type DB struct {
	Name string
	API  *pgxpool.Pool // rol sweveh_api
	// Migrator es el rol dueño de las tablas (sweveh_migrator), sujeto a RLS: lo usan las cargas de datos
	// controladas, como el taller ficticio de la demo (HU-97).
	Migrator *pgxpool.Pool
	Worker   *pgxpool.Pool // rol sweveh_worker
	Tx       *db.TxManager
}

// New crea la base y registra su limpieza al terminar la prueba.
func New(t testing.TB) *DB {
	t.Helper()
	if testing.Short() {
		t.Skip("prueba de integración: se omite con -short")
	}
	adminURL := os.Getenv("SWEVEH_TEST_ADMIN_URL")
	if adminURL == "" {
		t.Fatal("SWEVEH_TEST_ADMIN_URL no está definida: usa `make test` (levanta Postgres) o corre con -short")
	}
	pw := os.Getenv("SWEVEH_TEST_ROLE_PASSWORD")
	if pw == "" {
		pw = "sweveh_dev_only"
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatalf("testdb: conectar como admin: %v", err)
	}
	defer func() { _ = admin.Close(ctx) }()

	ensureRoles(t, ctx, admin, pw)
	name := "sweveh_t_" + randHex(6)
	for _, stmt := range []string{
		fmt.Sprintf("CREATE DATABASE %s OWNER sweveh_migrator", name),
		fmt.Sprintf("REVOKE ALL ON DATABASE %s FROM PUBLIC", name),
		fmt.Sprintf("GRANT CONNECT ON DATABASE %s TO sweveh_api, sweveh_worker", name),
	} {
		if _, err := admin.Exec(ctx, stmt); err != nil {
			t.Fatalf("testdb: %s: %v", stmt, err)
		}
	}
	t.Cleanup(func() {
		c, err := pgx.Connect(context.Background(), adminURL)
		if err != nil {
			return
		}
		defer func() { _ = c.Close(context.Background()) }()
		_, _ = c.Exec(context.Background(), fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", name))
	})

	migrate(t, roleURL(t, adminURL, "sweveh_migrator", pw, name))
	d := &DB{Name: name}
	d.API = pool(t, roleURL(t, adminURL, "sweveh_api", pw, name))
	d.Migrator = pool(t, roleURL(t, adminURL, "sweveh_migrator", pw, name))
	d.Worker = pool(t, roleURL(t, adminURL, "sweveh_worker", pw, name))
	d.Tx = db.NewTxManager(d.API)
	return d
}

func ensureRoles(t testing.TB, ctx context.Context, admin *pgx.Conn, pw string) {
	t.Helper()
	// Varios paquetes corren en paralelo: un candado consultivo evita crear el rol dos veces.
	if _, err := admin.Exec(ctx, "SELECT pg_advisory_lock(7349001)"); err != nil {
		t.Fatalf("testdb: candado: %v", err)
	}
	defer func() { _, _ = admin.Exec(ctx, "SELECT pg_advisory_unlock(7349001)") }()
	roles := []struct{ name, extra string }{
		{"sweveh_migrator", ""}, {"sweveh_api", "NOINHERIT"}, {"sweveh_worker", "NOINHERIT"},
	}
	for _, r := range roles {
		var exists bool
		if err := admin.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = $1)", r.name).Scan(&exists); err != nil {
			t.Fatalf("testdb: %v", err)
		}
		if exists {
			continue
		}
		stmt := fmt.Sprintf("CREATE ROLE %s LOGIN NOBYPASSRLS %s PASSWORD '%s'", r.name, r.extra, pw)
		if _, err := admin.Exec(ctx, stmt); err != nil {
			t.Fatalf("testdb: crear rol %s: %v", r.name, err)
		}
	}
}

func migrate(t testing.TB, u string) {
	t.Helper()
	conn, err := sql.Open("pgx", u)
	if err != nil {
		t.Fatalf("testdb: abrir migrador: %v", err)
	}
	defer func() { _ = conn.Close() }()
	p, err := goose.NewProvider(goose.DialectPostgres, conn, migrations.FS)
	if err != nil {
		t.Fatalf("testdb: goose: %v", err)
	}
	if _, err := p.Up(context.Background()); err != nil {
		t.Fatalf("testdb: migraciones: %v", err)
	}
}

func pool(t testing.TB, u string) *pgxpool.Pool {
	t.Helper()
	p, err := db.NewPool(context.Background(), db.PoolConfig{URL: u, MaxConns: 20, MaxConnLifetime: time.Minute})
	if err != nil {
		t.Fatalf("testdb: pool: %v", err)
	}
	t.Cleanup(p.Close)
	return p
}

func roleURL(t testing.TB, adminURL, user, pw, dbname string) string {
	t.Helper()
	u, err := url.Parse(adminURL)
	if err != nil {
		t.Fatalf("testdb: url: %v", err)
	}
	u.User = url.UserPassword(user, pw)
	u.Path = "/" + dbname
	return u.String()
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// NewTenant crea un taller (como lo haría el registro) y devuelve su id.
func (d *DB) NewTenant(t testing.TB, name string) shared.ID {
	t.Helper()
	id := shared.NewID()
	err := d.Tx.DoWithSettings(context.Background(), map[string]string{"app.taller_id": id.String()}, func(ctx context.Context) error {
		c, err := db.Conn(ctx)
		if err != nil {
			return err
		}
		_, err = c.Exec(ctx, "INSERT INTO tenancy.taller (id, nombre) VALUES ($1, $2)", id, name)
		return err
	})
	if err != nil {
		t.Fatalf("testdb: crear taller: %v", err)
	}
	return id
}

// As devuelve un context con el taller (y un usuario ficticio) como lo pondría la sesión.
func As(tallerID shared.ID) context.Context {
	return reqctx.WithPrincipal(context.Background(), reqctx.Principal{
		UserID: shared.NewID(), TallerID: tallerID, SessionID: shared.NewID(), Roles: []string{"dueno"},
	})
}
