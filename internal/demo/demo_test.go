package demo_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"sweveh/internal/demo"
	"sweveh/internal/platform/db"
	"sweveh/internal/platform/storage"
	"sweveh/internal/platform/testdb"
	"sweveh/internal/shared"
)

// in corre fn dentro del taller (RLS) con el rol dueño de las tablas.
func in(t *testing.T, d *testdb.DB, tallerID shared.ID, fn func(ctx context.Context) error) {
	t.Helper()
	if err := db.NewTxManager(d.Migrator).DoWithSettings(context.Background(), map[string]string{"app.taller_id": tallerID.String()}, fn); err != nil {
		t.Fatal(err)
	}
}

func count(ctx context.Context, t *testing.T, sql string, args ...any) int {
	t.Helper()
	n, err := db.Scalar[int](ctx, sql, args...)
	if err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return n
}

// HU-97 y HU-98: la carga deja el taller ficticio como lo describe docs/03 y se puede reiniciar sin tocar la base a
// mano: la demo anterior queda archivada y el dueño entra al taller nuevo.
func TestSeedAndReset(t *testing.T) {
	d := testdb.New(t)
	st, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	deps := demo.Deps{API: d.API, Bootstrap: d.Migrator, Storage: st, Phones: demo.DefaultPhones}
	start := time.Now()
	res, err := demo.Seed(context.Background(), deps, false)
	if err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took > 2*time.Minute {
		t.Fatalf("la carga tardó %v", took)
	}

	in(t, d, res.TallerID, func(ctx context.Context) error {
		if n := count(ctx, t, `SELECT count(*) FROM customers.cliente`); n != 5 {
			t.Errorf("clientes: %d", n)
		}
		// HU-98: la moto de Sebastián.
		if n := count(ctx, t, `SELECT count(*) FROM vehicles.vehiculo WHERE placa = 'XVN43F' AND tipo = 'moto' AND marca = 'Honda'
			AND linea = 'CB 190R' AND modelo_anio = 2021`); n != 1 {
			t.Errorf("moto de Sebastián: %d", n)
		}
		// HU-97, criterio 2: todos con motor menos el Picanto de Carlos.
		if n := count(ctx, t, `SELECT count(*) FROM vehicles.vehiculo WHERE cilindraje_cc IS NULL`); n != 1 {
			t.Errorf("vehículos sin motor: %d", n)
		}
		if n := count(ctx, t, `SELECT count(*) FROM vehicles.vehiculo WHERE cilindraje_cc IS NULL AND placa = 'WMR552'`); n != 1 {
			t.Error("el Aveo de Marta (sin órdenes) debía quedar con motor pendiente")
		}
		want := map[int]string{31: "entregado", 38: "entregado", 40: "listo", 41: "por_recibir", 42: "en_diagnostico", 43: "en_reparacion", 44: "esperando_aprobacion"}
		for num, state := range want {
			if n := count(ctx, t, `SELECT count(*) FROM orders.orden WHERE numero = $1 AND estado = $2 AND NOT ejemplo`, num, state); n != 1 {
				t.Errorf("OS-%04d %s: %d", num, state, n)
			}
		}
		if n := count(ctx, t, `SELECT count(*) FROM orders.orden WHERE ejemplo AND estado = 'entregado'
			AND cerrada_en >= now() - interval '31 days'`); n != 20 {
			t.Errorf("órdenes de ejemplo entregadas en 30 días: %d", n)
		}
		if n := count(ctx, t, `SELECT count(*) FROM orders.orden WHERE numero = 43 AND fecha_prometida IS NOT NULL AND mecanico_id IS NOT NULL`); n != 1 {
			t.Error("OS-0043 sin fecha prometida o sin mecánico")
		}
		if n := count(ctx, t, `SELECT count(*) FROM orders.orden o JOIN quotes.cotizacion q ON q.orden_id = o.id
			WHERE o.numero = 44 AND q.enviada_en <= now() - interval '25 hours'`); n != 1 {
			t.Error("la cotización de OS-0044 debía llevar unas 26 horas esperando")
		}
		stock := map[string]int{"PF-SPK": 4, "AC-2050": 12, "FA-SPK": 3, "LF-DOT4": 6, "BH-H4": 0}
		for ref, n := range stock {
			if got := count(ctx, t, `SELECT existencia FROM inventory.articulo WHERE referencia = $1`, ref); got != n {
				t.Errorf("existencia de %s: %d", ref, got)
			}
		}
		if n := count(ctx, t, `SELECT count(*) FROM outbox.evento WHERE procesado_en IS NULL`); n != 0 {
			t.Errorf("eventos sin procesar (saldrían por WhatsApp): %d", n)
		}
		return nil
	})

	// Lo que ve Hernán con los datos de la demo: HU-76 (pendientes con su botón), HU-45 (4 días sin recoger) y HU-75
	// (el ticket explicado frente al mes pasado y con las órdenes que lo mueven).
	read, err := demo.AsOwner(deps, "/api/v1/today", "/api/v1/dashboard")
	if err != nil {
		t.Fatal(err)
	}
	today, dash := read[0], read[1]
	var seen []string
	for _, x := range today["pendientes"].([]any) {
		p := x.(map[string]any)
		seen = append(seen, p["tipo"].(string)+": "+p["titulo"].(string)+" "+p["detalle"].(string))
	}
	all := strings.Join(seen, " | ")
	if !strings.Contains(all, "aprobacion: GBW118 no ha aprobado OS-0044") || !strings.Contains(all, "JNX907 · 4 días sin recoger") {
		t.Fatalf("HU-76 / HU-45, pendientes de hoy: %s", all)
	}
	for _, x := range dash["kpis"].([]any) {
		k := x.(map[string]any)
		if k["codigo"] == "ticket" && (!strings.Contains(k["queSignifica"].(string), "mes pasado") || !strings.Contains(k["queSignifica"].(string), "sobre todo OS-")) {
			t.Fatalf("HU-75: %v", k["queSignifica"])
		}
		if k["codigo"] == "aprobaciones" && !strings.HasPrefix(k["texto"].(string), "1 · 2") {
			t.Fatalf("HU-69 con la demo: %v", k)
		}
	}

	if _, err := demo.Seed(context.Background(), deps, false); !errors.Is(err, demo.ErrExists) {
		t.Fatalf("cargar dos veces sin reiniciar: %v", err)
	}
	again, err := demo.Seed(context.Background(), deps, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Archived) != 1 || again.Archived[0] != res.TallerID || again.TallerID == res.TallerID {
		t.Fatalf("reinicio: %+v", again)
	}
	in(t, d, res.TallerID, func(ctx context.Context) error {
		name, err := db.Scalar[string](ctx, `SELECT nombre FROM tenancy.taller WHERE id = $1`, res.TallerID)
		if err != nil || !strings.Contains(name, "demo archivada") {
			t.Errorf("taller anterior: %q %v", name, err)
		}
		if n := count(ctx, t, `SELECT count(*) FROM identity.membresia WHERE taller_id = $1 AND activo`, res.TallerID); n != 0 {
			t.Errorf("membresías activas en la demo archivada: %d", n)
		}
		return nil
	})
	in(t, d, again.TallerID, func(ctx context.Context) error {
		if n := count(ctx, t, `SELECT count(*) FROM orders.orden WHERE numero BETWEEN 40 AND 44`); n != 5 {
			t.Errorf("órdenes del guion tras reiniciar: %d", n)
		}
		if n := count(ctx, t, `SELECT count(*) FROM identity.membresia WHERE taller_id = $1 AND activo`, again.TallerID); n != 3 {
			t.Errorf("Hernán, Jhon y Yesica en el taller nuevo: %d", n)
		}
		return nil
	})
}
