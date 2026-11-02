-- Esquemas por módulo (docs/06 §5.2) y funciones de contexto para RLS (R-06).
-- Requiere que existan los roles sweveh_api y sweveh_worker (db/bootstrap/roles.sql).

-- +goose Up
CREATE SCHEMA tenancy;
CREATE SCHEMA identity;
CREATE SCHEMA customers;
CREATE SCHEMA vehicles;
CREATE SCHEMA orders;
CREATE SCHEMA audit;
CREATE SCHEMA outbox;

GRANT USAGE ON SCHEMA tenancy, identity, customers, vehicles, orders, audit, outbox TO sweveh_api;
GRANT USAGE ON SCHEMA tenancy, outbox TO sweveh_worker;

-- Taller de la transacción actual. NULLIF porque, tras un SET LOCAL, la variable
-- puede volver como '' en vez de NULL; con NULL la política no ve filas (falla cerrado).
CREATE FUNCTION tenancy.taller_actual() RETURNS uuid
    LANGUAGE sql STABLE PARALLEL SAFE
    AS $$ SELECT NULLIF(current_setting('app.taller_id', true), '')::uuid $$;

-- Usuario que inicia sesión antes de conocer su taller (solo identity lo fija).
CREATE FUNCTION identity.usuario_actual() RETURNS uuid
    LANGUAGE sql STABLE PARALLEL SAFE
    AS $$ SELECT NULLIF(current_setting('app.usuario_id', true), '')::uuid $$;

GRANT EXECUTE ON FUNCTION tenancy.taller_actual() TO sweveh_api, sweveh_worker;
GRANT EXECUTE ON FUNCTION identity.usuario_actual() TO sweveh_api;

-- +goose Down
DROP SCHEMA outbox CASCADE;
DROP SCHEMA audit CASCADE;
DROP SCHEMA orders CASCADE;
DROP SCHEMA vehicles CASCADE;
DROP SCHEMA customers CASCADE;
DROP SCHEMA identity CASCADE;
DROP SCHEMA tenancy CASCADE;
