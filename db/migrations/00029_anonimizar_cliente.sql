-- HU-87: el titular pide que borren sus datos. El cliente se anonimiza (nombre, celular y correo) y se conservan sus
-- órdenes y montos sin identificarlo. El celular queda como "anonimo-<id>" para no romper la unicidad por taller.

-- +goose Up
ALTER TABLE customers.cliente ADD COLUMN anonimizado_en timestamptz;
ALTER TABLE customers.cliente DROP CONSTRAINT cliente_celular_check;
ALTER TABLE customers.cliente ADD CONSTRAINT cliente_celular_check CHECK (
    celular ~ '^\+573[0-9]{9}$' OR (anonimizado_en IS NOT NULL AND celular ~ '^anonimo-[0-9a-f-]{36}$'));

-- +goose Down
ALTER TABLE customers.cliente DROP CONSTRAINT cliente_celular_check;
ALTER TABLE customers.cliente ADD CONSTRAINT cliente_celular_check CHECK (celular ~ '^\+573[0-9]{9}$');
ALTER TABLE customers.cliente DROP COLUMN anonimizado_en;
