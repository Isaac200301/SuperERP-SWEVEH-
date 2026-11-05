-- HU-93 (ADR-0032): pasar la lista de precios con una foto. La IA lee la foto y cada fila queda como propuesta para que
-- el dueño la corrija y la apruebe en Por revisar; solo al aprobarla se vuelve servicio. La foto vive en el
-- almacenamiento de archivos y se muestra al lado de cada fila.

-- +goose Up
CREATE TABLE catalog.propuesta (
    id            uuid PRIMARY KEY,
    taller_id     uuid NOT NULL REFERENCES tenancy.taller(id),
    lote_id       uuid NOT NULL,
    foto_clave    text NOT NULL CHECK (foto_clave ~ '^[a-f0-9-]{36}/[a-f0-9-]{36}$'),
    foto_tipo     text NOT NULL CHECK (foto_tipo IN ('image/jpeg', 'image/png', 'image/webp')),
    nombre        text NOT NULL CHECK (length(btrim(nombre)) BETWEEN 1 AND 120),
    precio        bigint CHECK (precio IS NULL OR precio >= 0),
    tipo_vehiculo text NOT NULL CHECK (tipo_vehiculo IN ('carro', 'moto', 'ambos')),
    estado        text NOT NULL DEFAULT 'pendiente' CHECK (estado IN ('pendiente', 'aprobada', 'descartada')),
    servicio_id   uuid,
    created_at    timestamptz NOT NULL DEFAULT now(),
    resuelto_en   timestamptz,
    resuelto_por  uuid
);
CREATE INDEX propuesta_pendiente_idx ON catalog.propuesta (taller_id, created_at) WHERE estado = 'pendiente';
ALTER TABLE catalog.propuesta ENABLE ROW LEVEL SECURITY;
ALTER TABLE catalog.propuesta FORCE ROW LEVEL SECURITY;
CREATE POLICY aislamiento_taller ON catalog.propuesta USING (taller_id = tenancy.taller_actual()) WITH CHECK (taller_id = tenancy.taller_actual());
GRANT SELECT, INSERT, UPDATE ON catalog.propuesta TO sweveh_api, sweveh_worker;

-- +goose Down
DROP TABLE catalog.propuesta;
