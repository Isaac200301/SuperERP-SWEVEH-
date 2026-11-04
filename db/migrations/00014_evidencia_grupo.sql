-- Lo que encontró Pablo probando la orden (3-nov-2026): varias fotos por nota (grupo), fotos de cada hallazgo,
-- mano de obra estimada en el diagnóstico y precio editable solo donde lo pone una persona (RF-17).

-- +goose Up
ALTER TABLE orders.hallazgo
    ADD COLUMN mano_obra bigint CHECK (mano_obra IS NULL OR mano_obra >= 0);

ALTER TABLE orders.evidencia
    ADD COLUMN grupo_id    uuid,
    ADD COLUMN hallazgo_id uuid REFERENCES orders.hallazgo(id) ON DELETE SET NULL;
CREATE INDEX evidencia_grupo_idx ON orders.evidencia (taller_id, orden_id, grupo_id);

-- Precio libre: ítem sin referencia o servicio sin precio de lista. Los precios de lista no se editan en la cotización.
ALTER TABLE quotes.linea
    ADD COLUMN precio_libre boolean NOT NULL DEFAULT false;
UPDATE quotes.linea SET precio_libre = true WHERE tipo = 'servicio' AND ref_id IS NULL;

-- +goose Down
ALTER TABLE quotes.linea DROP COLUMN precio_libre;
DROP INDEX orders.evidencia_grupo_idx;
ALTER TABLE orders.evidencia DROP COLUMN grupo_id, DROP COLUMN hallazgo_id;
ALTER TABLE orders.hallazgo DROP COLUMN mano_obra;
