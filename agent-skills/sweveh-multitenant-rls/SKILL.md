---
name: sweveh-multitenant-rls
description: Aislamiento entre talleres en SWEVEH con taller_id, Row Level Security de Postgres/Supabase y SET LOCAL app.taller_id, incluida la prueba obligatoria con dos talleres y el plan B si falla con el pooler. Úsala al crear tablas, consultas, migraciones o cualquier acceso a datos.
---

# Multi-tenant con RLS (R-06)

Regla: **ningún taller puede ver ni modificar datos de otro**. Fuente: `docs/02` R-06 y `docs/06` (§5 datos, riesgo R-A3). Estado: ADR de multi-tenant por `taller_id` + RLS = `Propuesta`; la **compatibilidad de `SET LOCAL` con el pooler de Supabase es `Inferencia`** y se prueba en el Sprint 0 antes de apoyar el diseño en ella. Referencias: https://www.postgresql.org/docs/current/ddl-rowsecurity.html · https://supabase.com/docs/guides/database/postgres/row-level-security

## 1. Receta para cada tabla de negocio

```sql
-- db/migrations/0007_orders.up.sql
CREATE TABLE orders.ordenes (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    taller_id   uuid NOT NULL REFERENCES tenancy.talleres(id),
    numero      integer NOT NULL,
    estado      text NOT NULL,
    version     integer NOT NULL DEFAULT 1,
    creado_en   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (taller_id, numero)
);
CREATE INDEX ON orders.ordenes (taller_id, estado);

ALTER TABLE orders.ordenes ENABLE ROW LEVEL SECURITY;
ALTER TABLE orders.ordenes FORCE ROW LEVEL SECURITY;   -- aplica también al dueño de la tabla

CREATE POLICY ordenes_aislamiento ON orders.ordenes
    USING      (taller_id = current_setting('app.taller_id', true)::uuid)
    WITH CHECK (taller_id = current_setting('app.taller_id', true)::uuid);
```

- `current_setting(..., true)` devuelve `NULL` si no hay valor → la comparación da NULL → **no se ven filas** (falla cerrado).
- `taller_id` siempre `NOT NULL`, con FK, y **primer campo de los índices** compuestos.
- Una tabla por esquema de módulo (`orders.`, `vehicles.`, …); no mezclar.
- Convención: tablas y columnas en español snake_case (docs/08).

## 2. Cómo llega el `taller_id`

1. Sale del **JWT validado en el servidor** (claim firmado), jamás del cuerpo, la URL o un encabezado del cliente.
2. El middleware lo coloca en el `context`.
3. El `UnitOfWork` abre la transacción y ejecuta, como primera sentencia:

```go
func (u *PgUnitOfWork) Do(ctx context.Context, fn func(context.Context, Tx) error) error {
	tid, ok := tenancy.FromContext(ctx)
	if !ok { return ErrNoTenant } // falla cerrado
	return pgx.BeginFunc(ctx, u.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT set_config('app.taller_id', $1, true)", tid.String()); err != nil {
			return err
		}
		return fn(ctx, newTx(tx))
	})
}
```

`set_config(..., true)` equivale a `SET LOCAL`: vale solo dentro de la transacción. **Nunca** `SET` (sesión) con un pool: se filtraría a otra solicitud.

## 3. Roles de base de datos

| Rol | Uso | Permisos |
|---|---|---|
| `sweveh_api` | La API | `NOBYPASSRLS`, sin DDL, solo CRUD en sus esquemas |
| `sweveh_worker` | Worker de outbox | igual; lee `outbox` con su política |
| `sweveh_migrator` | Migraciones (CI) | DDL; **no** lo usa la API |
| `sweveh_admin_erp` | Realm del Administrador del ERP | acceso cruzado explícito, solo por rutas del realm admin y auditado |

- Nunca uses la `service_role` de Supabase ni el usuario `postgres` en la API.
- El realm del **Administrador del ERP** (HU-74) opera entre talleres por diseño: aislado en otro rol/rutas, con auditoría, y sin pasar por las rutas del taller.

## 4. Prueba de aislamiento (obligatoria en cada PR que agrega una tabla)

```go
func TestOrders_Isolation(t *testing.T) {
	db := testdb.New(t)                       // Postgres real, rol sweveh_api
	a, b := db.NewTenant(t), db.NewTenant(t)  // dos talleres
	idA := db.As(a).CreateOrder(t)

	// B no la ve ni por ID
	if _, err := db.As(b).GetOrder(t, idA); !errors.Is(err, ErrNotFound) { t.Fatalf("B vio la orden de A: %v", err) }
	// B no la lista
	if n := len(db.As(b).ListOrders(t)); n != 0 { t.Fatalf("B lista %d órdenes de A", n) }
	// B no puede modificarla ni insertar con taller_id de A
	if err := db.As(b).UpdateOrder(t, idA); err == nil { t.Fatal("B modificó orden de A") }
	if err := db.As(b).InsertWithTenant(t, a.ID); err == nil { t.Fatal("B insertó en A") }
	// sin taller en el contexto: no ve nada
	if n := len(db.AsNoTenant().ListOrders(t)); n != 0 { t.Fatal("sin tenant debe ver 0") }
}
```

Añade también prueba **HTTP** (con tokens de A y B): el recurso de A devuelve 404 para B (no 403, para no revelar existencia).

## 5. Capas de defensa (no confíes solo en RLS)

1. JWT con `taller_id` + verificación de propiedad del objeto en la capa de aplicación (anti-IDOR).
2. Consultas con filtro `taller_id` explícito cuando sea natural (mejora planes e índices).
3. RLS como red de seguridad final.
4. Pruebas de aislamiento en CI.

## 6. Plan B si `SET LOCAL` + pooler no funciona

Si la prueba del Sprint 0 muestra que el modo de transacción del pooler de Supabase pierde `set_config` (o no es fiable):
1. **No improvises.** Esto es una pregunta de la lista "PARA y pregunta" (`docs/02` §3.3 punto 6).
2. Plan B documentado en `docs/06` (R-A3): conexión directa/sesión para la API con pool propio, o filtrado por `taller_id` en cada consulta con RLS como respaldo.
3. Registra un ADR y notifica a Isaac o Pablo con la evidencia (salida real de la prueba).

## 7. Errores comunes

- Olvidar `FORCE ROW LEVEL SECURITY` (el dueño de la tabla se salta la política).
- Vistas y funciones `SECURITY DEFINER` que evitan RLS: evítalas o audítalas.
- Índices únicos globales (`UNIQUE (numero)`) en vez de `(taller_id, numero)`.
- Cachés o colas sin `taller_id` en la clave.
- Datos reales en pruebas: usa datos sintéticos (**Ficticio**) (R-10).
- Registrar el `taller_id` desde el cuerpo de la solicitud "porque ya viene".

## 8. Checklist

- [ ] Tabla con `taller_id NOT NULL`, FK, `ENABLE` + `FORCE RLS` y política con `USING` y `WITH CHECK`.
- [ ] Índices con `taller_id` como prefijo.
- [ ] `UnitOfWork` fija `app.taller_id` con `set_config(..., true)`.
- [ ] Prueba de aislamiento con dos talleres (BD y HTTP) en el PR.
- [ ] La API corre con un rol sin `BYPASSRLS`.
