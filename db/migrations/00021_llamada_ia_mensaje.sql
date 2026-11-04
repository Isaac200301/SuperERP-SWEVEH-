-- HU-99: cada llamada a la IA queda ligada al mensaje que la originó (o a la migración, más adelante).
-- Sin llave foránea: el mensaje vive en otro esquema (messaging) y el registro de costo no debe impedir borrarlo.

-- +goose Up
ALTER TABLE assistant.llamada_ia ADD COLUMN mensaje_id uuid;
CREATE INDEX llamada_ia_mensaje_idx ON assistant.llamada_ia (taller_id, mensaje_id);

-- +goose Down
DROP INDEX assistant.llamada_ia_mensaje_idx;
ALTER TABLE assistant.llamada_ia DROP COLUMN mensaje_id;
