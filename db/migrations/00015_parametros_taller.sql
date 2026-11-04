-- Parámetros del taller (decisión de Pablo, 3-nov-2026: "parametrizar todo"). Una fila por taller; sin fila valen
-- los valores por defecto de tenancy/domain.DefaultSettings, así un taller nuevo no tiene que configurar nada.
-- Columnas tipadas con CHECK para que la base también valide. RLS forzado como el resto de tablas.

-- +goose Up
CREATE TABLE tenancy.parametros (
    taller_id               uuid PRIMARY KEY REFERENCES tenancy.taller(id),
    usar_esperando_repuesto boolean NOT NULL DEFAULT false,
    usar_control_calidad    boolean NOT NULL DEFAULT false,
    fotos_minimas           integer NOT NULL DEFAULT 4 CHECK (fotos_minimas BETWEEN 0 AND 8),
    km_obligatorio          boolean NOT NULL DEFAULT true,
    items_carro             jsonb NOT NULL DEFAULT '[]'::jsonb,
    items_moto              jsonb NOT NULL DEFAULT '[]'::jsonb,
    iva_pct                 integer NOT NULL DEFAULT 19 CHECK (iva_pct BETWEEN 0 AND 30),
    precios_con_iva         boolean NOT NULL DEFAULT true,
    tarifa_hora             bigint CHECK (tarifa_hora IS NULL OR tarifa_hora >= 0),
    descuento_max_pct       integer NOT NULL DEFAULT 10 CHECK (descuento_max_pct BETWEEN 0 AND 100),
    descuento_recepcion     boolean NOT NULL DEFAULT false,
    vigencia_dias           integer NOT NULL DEFAULT 15 CHECK (vigencia_dias BETWEEN 1 AND 90),
    version                 integer NOT NULL DEFAULT 1,
    updated_at              timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE tenancy.parametros ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenancy.parametros FORCE ROW LEVEL SECURITY;
CREATE POLICY aislamiento_taller ON tenancy.parametros
    USING (taller_id = tenancy.taller_actual()) WITH CHECK (taller_id = tenancy.taller_actual());
GRANT SELECT, INSERT, UPDATE ON tenancy.parametros TO sweveh_api;

-- +goose Down
DROP TABLE tenancy.parametros;
