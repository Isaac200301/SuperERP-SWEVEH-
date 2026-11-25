-- Lo gastado contra lo cotizado (V-08, ADR-0040, HU-106): el taller anota lo que va gastando en una orden (una frase o
-- la foto de la factura) para saber cuánto le queda de lo cotizado. Es interno: nunca sale en la vista del cliente ni
-- en el PDF. Un gasto no se borra: se anula, con bitácora.

-- +goose Up
CREATE TABLE orders.gasto (
    id           uuid PRIMARY KEY,
    taller_id    uuid NOT NULL REFERENCES tenancy.taller(id),
    orden_id     uuid NOT NULL REFERENCES orders.orden(id),
    descripcion  text NOT NULL CHECK (length(btrim(descripcion)) BETWEEN 1 AND 200),
    valor        bigint NOT NULL CHECK (valor > 0),
    origen       text NOT NULL CHECK (origen IN ('manual', 'asistente', 'factura')),
    foto_id      uuid REFERENCES orders.evidencia(id),
    creado_por   uuid,
    creado_en    timestamptz NOT NULL DEFAULT now(),
    anulado      boolean NOT NULL DEFAULT false,
    anulado_en   timestamptz,
    anulado_por  uuid
);
CREATE INDEX gasto_orden_idx ON orders.gasto (taller_id, orden_id, creado_en);

ALTER TABLE orders.gasto ENABLE ROW LEVEL SECURITY;
ALTER TABLE orders.gasto FORCE ROW LEVEL SECURITY;
CREATE POLICY aislamiento_taller ON orders.gasto
    USING (taller_id = tenancy.taller_actual()) WITH CHECK (taller_id = tenancy.taller_actual());

-- Sin DELETE: un gasto se anula. El worker con los mismos privilegios que el api (00019).
GRANT SELECT, INSERT, UPDATE ON orders.gasto TO sweveh_api;
GRANT SELECT, INSERT, UPDATE ON orders.gasto TO sweveh_worker;

-- +goose Down
DROP TABLE orders.gasto;
