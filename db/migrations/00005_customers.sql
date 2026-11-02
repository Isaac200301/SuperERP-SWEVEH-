-- Clientes del taller, identificados por celular (HU-09, RF-04). Sin duplicados por taller.

-- +goose Up
CREATE TABLE customers.cliente (
    id                   uuid PRIMARY KEY,
    taller_id            uuid NOT NULL REFERENCES tenancy.taller(id),
    celular              text NOT NULL CHECK (celular ~ '^\+573[0-9]{9}$'),
    nombre               text NOT NULL CHECK (length(btrim(nombre)) BETWEEN 1 AND 120),
    correo               text CHECK (correo IS NULL OR correo ~ '^[^@[:space:]]+@[^@[:space:]]+\.[^@[:space:]]+$'),
    autorizacion_estado  text NOT NULL DEFAULT 'pendiente'
                         CHECK (autorizacion_estado IN ('pendiente', 'autorizada', 'rechazada')),
    version              integer NOT NULL DEFAULT 1,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),
    created_by           uuid,
    CONSTRAINT cliente_celular_unico UNIQUE (taller_id, celular)
);

ALTER TABLE customers.cliente ENABLE ROW LEVEL SECURITY;
ALTER TABLE customers.cliente FORCE ROW LEVEL SECURITY;
CREATE POLICY aislamiento_taller ON customers.cliente
    USING (taller_id = tenancy.taller_actual())
    WITH CHECK (taller_id = tenancy.taller_actual());

GRANT SELECT, INSERT, UPDATE ON customers.cliente TO sweveh_api;

-- +goose Down
DROP TABLE customers.cliente;
