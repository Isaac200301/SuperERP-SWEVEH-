-- HU-19: el cliente responde la constancia de recibo (AU-01) con "Está bien" o "Algo no cuadra". La orden guarda
-- la primera respuesta y su hora; "observado" la marca para que el dueño la revise.

-- +goose Up
ALTER TABLE orders.orden
    ADD COLUMN recibo_respuesta     text CHECK (recibo_respuesta IN ('conforme', 'observado')),
    ADD COLUMN recibo_respondido_en timestamptz,
    ADD CONSTRAINT recibo_respuesta_con_hora CHECK ((recibo_respuesta IS NULL) = (recibo_respondido_en IS NULL));

-- +goose Down
ALTER TABLE orders.orden
    DROP CONSTRAINT recibo_respuesta_con_hora,
    DROP COLUMN recibo_respondido_en,
    DROP COLUMN recibo_respuesta;
