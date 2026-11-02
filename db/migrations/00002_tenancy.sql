-- Taller (inquilino). Raíz del aislamiento: cada taller solo se ve a sí mismo.

-- +goose Up
CREATE TABLE tenancy.taller (
    id          uuid PRIMARY KEY,
    nombre      text NOT NULL CHECK (length(btrim(nombre)) BETWEEN 1 AND 120),
    version     integer NOT NULL DEFAULT 1,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE tenancy.taller ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenancy.taller FORCE ROW LEVEL SECURITY;
CREATE POLICY aislamiento_taller ON tenancy.taller
    USING (id = tenancy.taller_actual())
    WITH CHECK (id = tenancy.taller_actual());

GRANT SELECT, INSERT, UPDATE ON tenancy.taller TO sweveh_api;

-- +goose Down
DROP TABLE tenancy.taller;
