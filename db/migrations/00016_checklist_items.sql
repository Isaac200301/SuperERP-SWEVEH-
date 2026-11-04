-- Recepción con checklist editable (parámetros del taller): cada checklist guarda su copia de los ítems revisados
-- (clave, nombre, estado bien|mal|no_tiene, nota), así cambiar la lista del taller no altera recepciones hechas.

-- +goose Up
ALTER TABLE orders.checklist ADD COLUMN items jsonb NOT NULL DEFAULT '[]'::jsonb;

-- +goose Down
ALTER TABLE orders.checklist DROP COLUMN items;
