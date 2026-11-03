-- Inventario completo (Bloque 2): marca, categoría, ubicación y referencia original del repuesto; categoría y
-- garantía del servicio. Las categorías coinciden con los sistemas del diagnóstico (00012). RLS ya forzado (00008).

-- +goose Up
ALTER TABLE inventory.articulo
    ADD COLUMN marca          text CHECK (marca IS NULL OR length(marca) <= 60),
    ADD COLUMN categoria      text CHECK (categoria IN ('motor', 'frenos', 'suspension', 'direccion', 'electrico', 'transmision',
                                                        'refrigeracion', 'llantas', 'carroceria', 'filtros', 'lubricantes', 'otro')),
    ADD COLUMN ubicacion      text CHECK (ubicacion IS NULL OR length(ubicacion) <= 40),
    ADD COLUMN referencia_oem text CHECK (referencia_oem IS NULL OR referencia_oem ~ '^[A-Z0-9][A-Z0-9-]{0,39}$');
CREATE INDEX articulo_movimiento_idx ON inventory.movimiento (taller_id, articulo_id, created_at);

ALTER TABLE catalog.servicio
    ADD COLUMN categoria     text CHECK (categoria IN ('motor', 'frenos', 'suspension', 'direccion', 'electrico', 'transmision',
                                                       'refrigeracion', 'llantas', 'carroceria', 'filtros', 'lubricantes', 'otro')),
    ADD COLUMN garantia_dias integer CHECK (garantia_dias IS NULL OR garantia_dias BETWEEN 0 AND 3650);

-- +goose Down
DROP INDEX inventory.articulo_movimiento_idx;
ALTER TABLE inventory.articulo DROP COLUMN marca, DROP COLUMN categoria, DROP COLUMN ubicacion, DROP COLUMN referencia_oem;
ALTER TABLE catalog.servicio DROP COLUMN categoria, DROP COLUMN garantia_dias;
