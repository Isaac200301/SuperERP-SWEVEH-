package db_test

// Prueba técnica del Sprint 0 (docs/05 §4 paso 3, riesgo R-A3): RLS + SET LOCAL + pool.
// Corre contra Postgres directo. Contra el pooler de Supabase (Supavisor, modo sesión)
// hay que repetirla en staging: eso sigue pendiente porque aún no existe el proyecto.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"sweveh/internal/platform/db"
	"sweveh/internal/platform/reqctx"
	"sweveh/internal/platform/testdb"
	"sweveh/internal/shared"
)

func countTalleres(t *testing.T, ctx context.Context, tx *db.TxManager) int {
	t.Helper()
	var n int
	err := tx.Do(ctx, func(ctx context.Context) error {
		c, err := db.Conn(ctx)
		if err != nil {
			return err
		}
		return c.QueryRow(ctx, "SELECT count(*) FROM tenancy.taller").Scan(&n)
	})
	if err != nil {
		t.Fatalf("contar talleres: %v", err)
	}
	return n
}

// Con una sola conexión en el pool, la transacción de B reutiliza la conexión de A:
// si SET LOCAL se filtrara, B vería el taller de A.
func TestSetLocalDoesNotLeakAcrossPooledTransactions(t *testing.T) {
	d := testdb.New(t)
	a := d.NewTenant(t, "Taller A")
	b := d.NewTenant(t, "Taller B")

	cfg := d.API.Config()
	cfg.MaxConns = 1
	single, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer single.Close()
	tx := db.NewTxManager(single)

	for i := 0; i < 20; i++ {
		for _, id := range []shared.ID{a, b} {
			var seen shared.ID
			err := tx.Do(reqctx.WithTenant(context.Background(), id), func(ctx context.Context) error {
				c, _ := db.Conn(ctx)
				return c.QueryRow(ctx, "SELECT id FROM tenancy.taller").Scan(&seen)
			})
			if err != nil || seen != id {
				t.Fatalf("vuelta %d: el taller %s vio %s (%v)", i, id, seen, err)
			}
		}
	}
	// Después de las transacciones, la misma conexión sin SET LOCAL no ve nada.
	var n int
	if err := single.QueryRow(context.Background(), "SELECT count(*) FROM tenancy.taller").Scan(&n); err != nil {
		t.Fatalf("consulta fuera de transacción: %v", err)
	}
	if n != 0 {
		t.Fatalf("sin taller fijado se ven %d talleres; debía ser 0", n)
	}
}

func TestNoTenantFailsClosed(t *testing.T) {
	d := testdb.New(t)
	d.NewTenant(t, "Taller A")
	if err := d.Tx.Do(context.Background(), func(context.Context) error { return nil }); !errors.Is(err, db.ErrNoTenant) {
		t.Fatalf("Do sin taller debía fallar con ErrNoTenant, dio %v", err)
	}
	if n := countTalleres(t, reqctx.WithTenant(context.Background(), shared.NewID()), d.Tx); n != 0 {
		t.Fatalf("un taller inexistente ve %d filas", n)
	}
}

func TestAPIRoleCannotWriteIntoAnotherTenant(t *testing.T) {
	d := testdb.New(t)
	a := d.NewTenant(t, "Taller A")
	b := d.NewTenant(t, "Taller B")
	err := d.Tx.Do(testdb.As(b), func(ctx context.Context) error {
		c, _ := db.Conn(ctx)
		_, err := c.Exec(ctx, `INSERT INTO customers.cliente (id, taller_id, celular, nombre) VALUES ($1, $2, '+573001112233', 'Intruso')`,
			shared.NewID(), a)
		return err
	})
	if err == nil || !strings.Contains(err.Error(), "row-level security") {
		t.Fatalf("B insertó un cliente en A: %v", err)
	}
}

func TestAPIRoleHasNoBypassAndOwnsNothing(t *testing.T) {
	d := testdb.New(t)
	ctx := context.Background()
	var bypass, super bool
	if err := d.API.QueryRow(ctx, "SELECT rolbypassrls, rolsuper FROM pg_roles WHERE rolname = current_user").Scan(&bypass, &super); err != nil {
		t.Fatal(err)
	}
	if bypass || super {
		t.Fatalf("sweveh_api no puede tener BYPASSRLS (%v) ni ser superusuario (%v)", bypass, super)
	}
	var owned int
	if err := d.API.QueryRow(ctx, "SELECT count(*) FROM pg_class WHERE relowner = (SELECT oid FROM pg_roles WHERE rolname = current_user)").Scan(&owned); err != nil {
		t.Fatal(err)
	}
	if owned != 0 {
		t.Fatalf("sweveh_api es dueño de %d objetos; no debe serlo (FORCE RLS aplica, pero sin DDL)", owned)
	}
	if _, err := d.API.Exec(ctx, "CREATE TABLE public.intruso (id int)"); err == nil {
		t.Fatal("sweveh_api pudo crear una tabla: no debe tener DDL")
	}
}

// Meta-prueba: toda tabla con columna taller_id (y el propio taller) tiene RLS habilitado y forzado
// con al menos una política. Si alguien agrega una tabla sin RLS, esta prueba falla (R-06).
func TestEveryTenantTableHasForcedRLS(t *testing.T) {
	d := testdb.New(t)
	rows, err := d.API.Query(context.Background(), `
		SELECT n.nspname || '.' || c.relname, c.relrowsecurity, c.relforcerowsecurity,
		       (SELECT count(*) FROM pg_policy p WHERE p.polrelid = c.oid)
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind = 'r'
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema')
		  AND (EXISTS (SELECT 1 FROM pg_attribute a WHERE a.attrelid = c.oid AND a.attname = 'taller_id' AND NOT a.attisdropped)
		       OR (n.nspname = 'tenancy' AND c.relname = 'taller'))`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	seen := 0
	for rows.Next() {
		var name string
		var rls, force bool
		var policies int
		if err := rows.Scan(&name, &rls, &force, &policies); err != nil {
			t.Fatal(err)
		}
		seen++
		// identity.sesion guarda taller_id pero es tabla de autenticación de plataforma (Q-29).
		if name == "identity.sesion" {
			continue
		}
		if !rls || !force || policies == 0 {
			t.Errorf("%s: RLS=%v FORCE=%v políticas=%d", name, rls, force, policies)
		}
	}
	if seen < 9 {
		t.Fatalf("solo se revisaron %d tablas; ¿cambió el esquema?", seen)
	}
}

func TestAuditIsInsertOnly(t *testing.T) {
	d := testdb.New(t)
	a := d.NewTenant(t, "Taller A")
	ctx := testdb.As(a)
	err := d.Tx.Do(ctx, func(ctx context.Context) error {
		c, _ := db.Conn(ctx)
		_, err := c.Exec(ctx, `INSERT INTO audit.evento (id, taller_id, actor_tipo, accion, entidad, entidad_id, ocurrido_en)
			VALUES ($1, $2, 'sistema', 'prueba', 'prueba', $1, now())`, shared.NewID(), a)
		return err
	})
	if err != nil {
		t.Fatalf("insertar auditoría: %v", err)
	}
	for _, stmt := range []string{"UPDATE audit.evento SET accion = 'cambiada'", "DELETE FROM audit.evento"} {
		err := d.Tx.Do(ctx, func(ctx context.Context) error {
			c, _ := db.Conn(ctx)
			_, err := c.Exec(ctx, stmt)
			return err
		})
		if err == nil {
			t.Fatalf("%q debía fallar: la bitácora es solo de inserción", stmt)
		}
	}
}
