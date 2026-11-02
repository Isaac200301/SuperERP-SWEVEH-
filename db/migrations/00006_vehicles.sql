-- Vehículos por placa, carro o moto, con su motor (HU-10, HU-11, RF-05).
-- Sin VIN (decisión del equipo, 1-nov-2026). Motor 1:1 como columnas: cilindraje en cc y turbo;
-- ambos nulos o uno nulo = "motor pendiente" (Q-23). cliente_id no lleva FK: es de otro módulo (ADR-09).

-- +goose Up
CREATE TABLE vehicles.vehiculo (
    id             uuid PRIMARY KEY,
    taller_id      uuid NOT NULL REFERENCES tenancy.taller(id),
    cliente_id     uuid NOT NULL,
    tipo           text NOT NULL CHECK (tipo IN ('carro', 'moto')),
    placa          text NOT NULL CHECK (placa ~ '^[A-Z0-9]{5,7}$'),
    formato_placa  text NOT NULL CHECK (formato_placa IN ('carro', 'moto', 'moto_antigua', 'especial')),
    marca          text CHECK (marca IS NULL OR length(marca) <= 60),
    linea          text CHECK (linea IS NULL OR length(linea) <= 60),
    modelo_anio    integer CHECK (modelo_anio IS NULL OR modelo_anio BETWEEN 1950 AND 2100),
    kilometraje    integer CHECK (kilometraje IS NULL OR kilometraje >= 0),
    cilindraje_cc  integer CHECK (cilindraje_cc IS NULL OR cilindraje_cc BETWEEN 50 AND 10000),
    turbo          boolean,
    version        integer NOT NULL DEFAULT 1,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    created_by     uuid,
    CONSTRAINT vehiculo_placa_unica UNIQUE (taller_id, placa)
);
CREATE INDEX vehiculo_cliente_idx ON vehicles.vehiculo (taller_id, cliente_id);

ALTER TABLE vehicles.vehiculo ENABLE ROW LEVEL SECURITY;
ALTER TABLE vehicles.vehiculo FORCE ROW LEVEL SECURITY;
CREATE POLICY aislamiento_taller ON vehicles.vehiculo
    USING (taller_id = tenancy.taller_actual())
    WITH CHECK (taller_id = tenancy.taller_actual());

GRANT SELECT, INSERT, UPDATE ON vehicles.vehiculo TO sweveh_api;

-- +goose Down
DROP TABLE vehicles.vehiculo;
