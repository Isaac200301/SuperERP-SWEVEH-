-- Roles de base de datos de SWEVEH (docs/06 §5.3, skill sweveh-multitenant-rls §3).
-- Lo ejecuta UNA vez una persona con privilegios de administración, antes de la primera migración.
-- Uso con psql (las contraseñas llegan por variables, nunca se escriben aquí):
--   psql "$ADMIN_URL" -v migrator_pw="..." -v api_pw="..." -v worker_pw="..." -f db/bootstrap/roles.sql
--
-- sweveh_migrator: dueño de las tablas, aplica DDL. Solo lo usa el job de migraciones.
-- sweveh_api:      la API. NOBYPASSRLS, sin DDL, solo los GRANT que dan las migraciones.
-- sweveh_worker:   el worker del outbox. NOBYPASSRLS.

CREATE ROLE sweveh_migrator LOGIN NOBYPASSRLS PASSWORD :'migrator_pw';
CREATE ROLE sweveh_api      LOGIN NOBYPASSRLS NOINHERIT PASSWORD :'api_pw';
CREATE ROLE sweveh_worker   LOGIN NOBYPASSRLS NOINHERIT PASSWORD :'worker_pw';

-- Límite de tiempo por sentencia para la API y el worker (valor inicial, a calibrar).
ALTER ROLE sweveh_api    SET statement_timeout = '5s';
ALTER ROLE sweveh_worker SET statement_timeout = '15s';
