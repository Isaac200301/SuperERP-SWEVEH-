-- Auditoría de negocio solo-inserción (HU-88, docs/06 §10.4) y outbox transaccional (docs/06 §8.6).
-- Ambas se escriben en la misma transacción que el cambio de estado (R-03 punto 3).

-- +goose Up
CREATE TABLE audit.evento (
    id           uuid PRIMARY KEY,
    taller_id    uuid NOT NULL REFERENCES tenancy.taller(id),
    actor_tipo   text NOT NULL CHECK (actor_tipo IN ('usuario', 'agente', 'sistema')),
    actor_id     uuid,
    accion       text NOT NULL,
    entidad      text NOT NULL,
    entidad_id   uuid NOT NULL,
    antes        jsonb,
    despues      jsonb,
    request_id   text,
    ocurrido_en  timestamptz NOT NULL
);
CREATE INDEX evento_taller_fecha_idx ON audit.evento (taller_id, ocurrido_en DESC);
CREATE INDEX evento_taller_entidad_idx ON audit.evento (taller_id, entidad, entidad_id);

ALTER TABLE audit.evento ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit.evento FORCE ROW LEVEL SECURITY;
CREATE POLICY aislamiento_taller ON audit.evento
    USING (taller_id = tenancy.taller_actual())
    WITH CHECK (taller_id = tenancy.taller_actual());

-- Solo inserción: ni la API ni el dueño de la tabla pueden cambiar o borrar la bitácora.
-- +goose StatementBegin
CREATE FUNCTION audit.impedir_cambios() RETURNS trigger
    LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'audit.evento es solo de inserción';
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER evento_inmutable BEFORE UPDATE OR DELETE OR TRUNCATE ON audit.evento
    FOR EACH STATEMENT EXECUTE FUNCTION audit.impedir_cambios();

GRANT SELECT, INSERT ON audit.evento TO sweveh_api;

CREATE TABLE outbox.evento (
    id               uuid PRIMARY KEY,
    taller_id        uuid NOT NULL REFERENCES tenancy.taller(id),
    tipo             text NOT NULL,
    agregado_id      uuid NOT NULL,
    payload          jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at       timestamptz NOT NULL DEFAULT now(),
    proximo_intento  timestamptz NOT NULL DEFAULT now(),
    intentos         integer NOT NULL DEFAULT 0,
    procesado_en     timestamptz,
    ultimo_error     text
);
CREATE INDEX evento_pendiente_idx ON outbox.evento (proximo_intento) WHERE procesado_en IS NULL;

ALTER TABLE outbox.evento ENABLE ROW LEVEL SECURITY;
ALTER TABLE outbox.evento FORCE ROW LEVEL SECURITY;
CREATE POLICY aislamiento_taller ON outbox.evento
    USING (taller_id = tenancy.taller_actual())
    WITH CHECK (taller_id = tenancy.taller_actual());
-- El worker despacha eventos de todos los talleres por diseño (docs/06 §4.5); no tiene otros permisos.
CREATE POLICY despacho_worker ON outbox.evento TO sweveh_worker
    USING (true)
    WITH CHECK (true);

GRANT INSERT ON outbox.evento TO sweveh_api;
GRANT SELECT, UPDATE ON outbox.evento TO sweveh_worker;

-- +goose Down
DROP TABLE outbox.evento;
DROP TRIGGER evento_inmutable ON audit.evento;
DROP FUNCTION audit.impedir_cambios();
DROP TABLE audit.evento;
