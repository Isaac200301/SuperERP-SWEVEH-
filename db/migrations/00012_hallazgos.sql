-- Diagnóstico en la orden (HU-103 propuesta, 3-nov-2026): hallazgos por sistema con gravedad y códigos de
-- falla del escáner. Los registra una persona, no la IA. Aislamiento por taller con RLS forzado.

-- +goose Up
CREATE TABLE orders.hallazgo (
    id             uuid PRIMARY KEY,
    taller_id      uuid NOT NULL REFERENCES tenancy.taller(id),
    orden_id       uuid NOT NULL REFERENCES orders.orden(id),
    sistema        text NOT NULL CHECK (sistema IN ('motor', 'frenos', 'suspension', 'direccion', 'electrico', 'transmision',
                                                    'refrigeracion', 'llantas', 'carroceria', 'otro')),
    descripcion    text NOT NULL CHECK (length(descripcion) BETWEEN 1 AND 1000),
    gravedad       text NOT NULL CHECK (gravedad IN ('urgente', 'recomendado', 'puede_esperar')),
    codigos_falla  text[] NOT NULL DEFAULT '{}',
    autor_id       uuid,
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX hallazgo_orden_idx ON orders.hallazgo (taller_id, orden_id, created_at);

ALTER TABLE orders.hallazgo ENABLE ROW LEVEL SECURITY;
ALTER TABLE orders.hallazgo FORCE ROW LEVEL SECURITY;
CREATE POLICY aislamiento_taller ON orders.hallazgo
    USING (taller_id = tenancy.taller_actual()) WITH CHECK (taller_id = tenancy.taller_actual());
GRANT SELECT, INSERT, DELETE ON orders.hallazgo TO sweveh_api;

-- +goose Down
DROP TABLE orders.hallazgo;
