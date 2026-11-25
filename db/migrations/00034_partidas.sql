-- Cotización por partidas (V-07, ADR-0040, HU-105): un bloque con nombre y un valor que pone el taller, con un detalle
-- interno opcional que NO cambia el valor ni el total. "Mostrar detalle al cliente" decide si la vista del cliente y el
-- PDF lo enseñan. Conviven con las líneas detalladas (servicio, repuesto).
-- El detalle se guarda aparte por línea; la cotización lo reemplaza entero al guardar (igual que sus líneas).

-- +goose Up
ALTER TABLE quotes.linea DROP CONSTRAINT linea_tipo_check;
ALTER TABLE quotes.linea ADD CONSTRAINT linea_tipo_check CHECK (tipo IN ('servicio', 'repuesto', 'partida'));

ALTER TABLE quotes.cotizacion ADD COLUMN mostrar_detalle boolean NOT NULL DEFAULT false;

CREATE TABLE quotes.partida_detalle (
    id             uuid PRIMARY KEY,
    taller_id      uuid NOT NULL REFERENCES tenancy.taller(id),
    cotizacion_id  uuid NOT NULL REFERENCES quotes.cotizacion(id) ON DELETE CASCADE,
    linea_id       uuid NOT NULL,
    descripcion    text NOT NULL CHECK (length(btrim(descripcion)) BETWEEN 1 AND 200),
    cantidad       integer NOT NULL DEFAULT 1 CHECK (cantidad > 0),
    valor_ref      bigint CHECK (valor_ref IS NULL OR valor_ref >= 0),
    orden          integer NOT NULL CHECK (orden >= 0),
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX partida_detalle_cotizacion_idx ON quotes.partida_detalle (taller_id, cotizacion_id);

ALTER TABLE quotes.partida_detalle ENABLE ROW LEVEL SECURITY;
ALTER TABLE quotes.partida_detalle FORCE ROW LEVEL SECURITY;
CREATE POLICY aislamiento_taller ON quotes.partida_detalle
    USING (taller_id = tenancy.taller_actual()) WITH CHECK (taller_id = tenancy.taller_actual());

-- El api y el worker (aprobar desde WhatsApp guarda la cotización entera, 00019) con los mismos privilegios.
GRANT SELECT, INSERT, UPDATE, DELETE ON quotes.partida_detalle TO sweveh_api;
GRANT SELECT, INSERT, UPDATE, DELETE ON quotes.partida_detalle TO sweveh_worker;

-- +goose Down
DROP TABLE quotes.partida_detalle;
ALTER TABLE quotes.cotizacion DROP COLUMN mostrar_detalle;
DELETE FROM quotes.linea WHERE tipo = 'partida';
ALTER TABLE quotes.linea DROP CONSTRAINT linea_tipo_check;
ALTER TABLE quotes.linea ADD CONSTRAINT linea_tipo_check CHECK (tipo IN ('servicio', 'repuesto'));
