---
name: sweveh-db-migraciones-sqlc
description: Base de datos de SWEVEH en Supabase/Postgres con pgx y sqlc: esquema por módulo, convenciones de nombres en español snake_case, migraciones seguras y reversibles, consultas sqlc, tipos de dinero/fechas/placa, índices y plan de cambios sin caída. Úsala al crear tablas, migraciones o consultas.
---

# Base de datos, migraciones y sqlc

Fuente: `docs/06` §5 y `docs/08` (convenciones). Decisiones: Supabase para base de datos = **equipo**; sqlc + pgx sin ORM, un esquema por módulo = `Propuesta` (ADR). **AQ-02 (región de Supabase) es ⛔ pendiente**; no cierres la región sin el equipo. Referencias: https://docs.sqlc.dev · https://github.com/jackc/pgx · https://www.postgresql.org/docs/current/ · https://supabase.com/docs/guides/database/overview

## 1. Convenciones (docs/08, propuesta del equipo)

- Código Go en inglés; **tablas y columnas en español snake_case**, plural para tablas (`ordenes`, `vehiculos`), `id uuid` como PK, `taller_id` en toda tabla de negocio.
- Esquema Postgres por módulo: `identity.`, `tenancy.`, `customers.`, `vehicles.`, `orders.`, `quotes.`, `inventory.`, `auditoria.`, `messaging.`.
- Tiempos: `timestamptz` (UTC). Dinero: `bigint` en **pesos** (COP sin decimales) con `CHECK (monto >= 0)` salvo ajustes explícitos; nunca `float`/`numeric` sin necesidad. Cilindraje: `integer` en cc. Placa: `text` en mayúsculas, sin espacios, con `CHECK` de formato por tipo de vehículo. Celular: E.164 `text`.
- Estados como `text` con `CHECK (estado IN (...))` (más simple de evolucionar que `ENUM`) y transición validada en el dominio.
- `version integer NOT NULL DEFAULT 1` en agregados editables (`sweveh-concurrencia-idempotencia`).
- `creado_en`, `actualizado_en`, `creado_por` donde aplique. Borrado lógico solo si hay razón de negocio/retención; si no, borrado real con auditoría.
- Únicos **por taller**: `UNIQUE (taller_id, celular)`, `UNIQUE (taller_id, placa)` (RF-04, RF-05).
- Sin VIN. Motor del vehículo: `cilindraje_cc integer`, `turbo boolean`, y estado "pendiente" modelado explícitamente (Q-23), por ejemplo `motor_completo boolean`/columnas nulas + `CHECK` coherente.

## 2. Migraciones

Herramienta: elegir una (golang-migrate, goose o atlas; decisión de bajo impacto, anótala en el PR/ADR). Reglas:

1. Archivos numerados y **solo hacia adelante en producción**; el `down` sirve en local/CI. Nunca edites una migración ya aplicada en staging/producción: crea otra.
2. Cada migración es pequeña, atómica y **idempotente donde sea posible** (`IF NOT EXISTS` solo cuando aporte).
3. **Compatibilidad adelante/atrás** (despliegue sin caída), patrón *expand → migrate → contract*:
   - Expand: añadir columna nullable / tabla nueva.
   - Backfill por lotes (no un `UPDATE` gigante que bloquee).
   - Cambiar el código para leer/escribir lo nuevo.
   - Contract: eliminar lo viejo en un release posterior.
4. Operaciones peligrosas en tablas grandes: `CREATE INDEX CONCURRENTLY` (fuera de transacción), `ADD CONSTRAINT … NOT VALID` y luego `VALIDATE CONSTRAINT`, evitar `ALTER COLUMN TYPE` que reescribe la tabla, cuidar `lock_timeout`/`statement_timeout`.
5. Toda tabla nueva: RLS + política + prueba de aislamiento (`sweveh-multitenant-rls`); permisos `GRANT` mínimos a `sweveh_api`/`sweveh_worker`.
6. Las migraciones las aplica el rol `sweveh_migrator` en CI/CD (no la API al arrancar en producción).
7. Datos de ejemplo van en `seed`, **no** en migraciones. Datos **Ficticio** únicamente.
8. Prueba en CI: aplicar todas las migraciones desde cero + `down`/`up` y comparar el esquema final con el esperado.

Ejemplo:

```sql
-- db/migrations/0012_vehiculos.up.sql
CREATE SCHEMA IF NOT EXISTS vehicles;

CREATE TABLE vehicles.vehiculos (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    taller_id     uuid NOT NULL REFERENCES tenancy.talleres(id),
    cliente_id    uuid NOT NULL REFERENCES customers.clientes(id),
    tipo          text NOT NULL CHECK (tipo IN ('carro','moto')),
    placa         text NOT NULL CHECK (placa = upper(placa) AND placa !~ '\s'),
    marca         text,
    modelo        text,
    cilindraje_cc integer CHECK (cilindraje_cc > 0),
    turbo         boolean,
    version       integer NOT NULL DEFAULT 1,
    creado_en     timestamptz NOT NULL DEFAULT now(),
    UNIQUE (taller_id, placa)
);
-- + RLS, política y GRANTs (ver sweveh-multitenant-rls)
```
(Ilustrativo: los campos exactos salen de las HU y del glosario; si dudas, aplica R-04.)

## 3. sqlc

`sqlc.yaml` con motor `postgresql`, `sql_package: pgx/v5`, un paquete generado por módulo en `internal/<modulo>/adapters/out/postgres/sqlcgen`, `emit_interface` solo si hace falta, tipos personalizados (`uuid`, `timestamptz`) mapeados a los del proyecto.

```sql
-- db/queries/vehicles.sql
-- name: GetVehicleByPlate :one
SELECT * FROM vehicles.vehiculos WHERE placa = $1;       -- RLS limita al taller; no hace falta taller_id, pero es válido añadirlo

-- name: ListVehiclesByCustomer :many
SELECT * FROM vehicles.vehiculos
 WHERE cliente_id = $1
 ORDER BY creado_en DESC, id DESC
 LIMIT $2;
```
- Una consulta por necesidad real; `SELECT *` solo cuando de verdad se usa la fila completa.
- Paginación por **cursor** (`WHERE (creado_en, id) < ($1, $2)`), no por `OFFSET` en tablas grandes.
- Consultas del repositorio devuelven tipos del dominio mediante un *mapper* explícito; los structs de sqlc no salen del adaptador.
- `make gen` falla si hay diferencias sin commitear (CI).
- Transacciones con `pgx.BeginFunc`; el `UnitOfWork` entrega `sqlcgen.Queries` ligadas a la `Tx`.

## 4. Rendimiento

- Índices con `taller_id` como prefijo y según consultas reales; revisa con `EXPLAIN (ANALYZE, BUFFERS)` en consultas críticas.
- Evita N+1: una consulta con `JOIN` dentro del **mismo módulo**; entre módulos, pide por lotes (`WHERE id = ANY($1)`) vía fachada.
- Límites de filas en toda lista; timeouts de sentencia (`statement_timeout` por rol).
- Pool de `pgx` dimensionado a los límites del plan de Supabase (**`Sin dato`** del límite exacto del plan; leer la documentación vigente) y compatible con el pooler (prueba del Sprint 0).

## 5. Seguridad de datos

- Datos personales mínimos (R-10); cifrado en reposo/transito lo da la plataforma (verificar en la documentación vigente de Supabase, `Sin dato` si no se comprobó).
- No uses `service_role` en la API. Claves y URLs por variables de entorno.
- Copias de seguridad y restauración: política y prueba de restauración (`docs/06` §12); frecuencia exacta depende del plan, marcada `Sin dato` hasta verificar.
- Borrado/retención automática diaria auditada; cuidado con claves foráneas y auditoría.

## 6. Checklist

- [ ] Esquema del módulo, `taller_id`, índices, `UNIQUE` por taller, `CHECK` de dominio.
- [ ] RLS + política + `GRANT` + prueba de aislamiento.
- [ ] Migración compatible hacia atrás; sin bloqueos largos.
- [ ] Consultas sqlc revisadas; `make gen` sin diferencias.
- [ ] Mapper al dominio; ningún struct de sqlc fuera del adaptador.
- [ ] Datos de prueba **Ficticio**.
