-- HU-44: el recordatorio de la cita (AU-06) lleva la dirección del taller. Es opcional y se pide en Mi taller, no en el
-- registro (HU-06: los datos se piden cuando hacen falta).

-- +goose Up
ALTER TABLE tenancy.taller ADD COLUMN direccion text CHECK (direccion IS NULL OR length(direccion) BETWEEN 1 AND 160);

-- +goose Down
ALTER TABLE tenancy.taller DROP COLUMN direccion;
