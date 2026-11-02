-- Órdenes de servicio (HU-16, RF-07) con número consecutivo por taller (R-07: contador)
-- e historial de estados inmutable (docs/02 §10). vehiculo_id y cliente_id sin FK: otros módulos (ADR-09).

-- +goose Up
CREATE TABLE orders.contador (
    taller_id  uuid PRIMARY KEY REFERENCES tenancy.taller(id),
    ultimo     integer NOT NULL CHECK (ultimo > 0)
);

ALTER TABLE orders.contador ENABLE ROW LEVEL SECURITY;
ALTER TABLE orders.contador FORCE ROW LEVEL SECURITY;
CREATE POLICY aislamiento_taller ON orders.contador
    USING (taller_id = tenancy.taller_actual())
    WITH CHECK (taller_id = tenancy.taller_actual());

CREATE TABLE orders.orden (
    id             uuid PRIMARY KEY,
    taller_id      uuid NOT NULL REFERENCES tenancy.taller(id),
    numero         integer NOT NULL CHECK (numero > 0),
    vehiculo_id    uuid NOT NULL,
    cliente_id     uuid NOT NULL,
    placa          text NOT NULL,
    tipo_vehiculo  text NOT NULL CHECK (tipo_vehiculo IN ('carro', 'moto')),
    estado         text NOT NULL CHECK (estado IN ('por_recibir', 'recibido', 'en_diagnostico',
                       'esperando_aprobacion', 'en_reparacion', 'listo', 'entregado', 'cancelada')),
    sintoma        text NOT NULL CHECK (length(btrim(sintoma)) BETWEEN 1 AND 2000),
    version        integer NOT NULL DEFAULT 1,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    created_by     uuid,
    CONSTRAINT orden_numero_unico UNIQUE (taller_id, numero)
);
CREATE INDEX orden_vehiculo_idx ON orders.orden (taller_id, vehiculo_id);
CREATE INDEX orden_recientes_idx ON orders.orden (taller_id, created_at DESC, id DESC);

ALTER TABLE orders.orden ENABLE ROW LEVEL SECURITY;
ALTER TABLE orders.orden FORCE ROW LEVEL SECURITY;
CREATE POLICY aislamiento_taller ON orders.orden
    USING (taller_id = tenancy.taller_actual())
    WITH CHECK (taller_id = tenancy.taller_actual());

CREATE TABLE orders.estado_historial (
    id           uuid PRIMARY KEY,
    taller_id    uuid NOT NULL REFERENCES tenancy.taller(id),
    orden_id     uuid NOT NULL REFERENCES orders.orden(id),
    desde        text,
    hacia        text NOT NULL,
    usuario_id   uuid,
    ocurrido_en  timestamptz NOT NULL
);
CREATE INDEX estado_historial_orden_idx ON orders.estado_historial (taller_id, orden_id, ocurrido_en);

ALTER TABLE orders.estado_historial ENABLE ROW LEVEL SECURITY;
ALTER TABLE orders.estado_historial FORCE ROW LEVEL SECURITY;
CREATE POLICY aislamiento_taller ON orders.estado_historial
    USING (taller_id = tenancy.taller_actual())
    WITH CHECK (taller_id = tenancy.taller_actual());

GRANT SELECT, INSERT, UPDATE ON orders.contador TO sweveh_api;
GRANT SELECT, INSERT, UPDATE ON orders.orden TO sweveh_api;
-- Historial inmutable: la API solo inserta y lee.
GRANT SELECT, INSERT ON orders.estado_historial TO sweveh_api;

-- +goose Down
DROP TABLE orders.estado_historial;
DROP TABLE orders.orden;
DROP TABLE orders.contador;
