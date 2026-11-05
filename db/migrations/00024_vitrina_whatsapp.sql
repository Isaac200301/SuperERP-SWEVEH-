-- ADR-0029: vitrina por WhatsApp. El taller marca qué repuestos o accesorios ofrece el asesor, con una foto.
-- La foto vive en el almacenamiento de archivos (clave taller/archivo, como la evidencia de las órdenes).

-- +goose Up
ALTER TABLE inventory.articulo
    ADD COLUMN ofrecer_whatsapp boolean NOT NULL DEFAULT false,
    ADD COLUMN foto_clave       text CHECK (foto_clave IS NULL OR foto_clave ~ '^[a-f0-9-]{36}/[a-f0-9-]{36}$'),
    ADD COLUMN foto_tipo        text CHECK (foto_tipo IS NULL OR foto_tipo IN ('image/jpeg', 'image/png', 'image/webp'));
CREATE INDEX articulo_vitrina_idx ON inventory.articulo (taller_id) WHERE ofrecer_whatsapp AND activo;

-- +goose Down
DROP INDEX inventory.articulo_vitrina_idx;
ALTER TABLE inventory.articulo DROP COLUMN ofrecer_whatsapp, DROP COLUMN foto_clave, DROP COLUMN foto_tipo;
