-- HU-15: unir clientes repetidos. El celular es único por taller (RF-04), así que el repetido es la misma persona con
-- otro celular o escrita dos veces. Al unir, sus vehículos, órdenes y conversaciones pasan al cliente que se conserva y
-- el repetido queda marcado (no se borra: la bitácora y sus autorizaciones siguen apuntando a él). Si vuelve a escribir
-- desde el celular del repetido, se le reconoce como el cliente que se conservó.

-- +goose Up
ALTER TABLE customers.cliente
    ADD COLUMN fusionado_con uuid REFERENCES customers.cliente(id),
    ADD COLUMN fusionado_en  timestamptz,
    ADD CONSTRAINT cliente_fusion_completa CHECK ((fusionado_con IS NULL) = (fusionado_en IS NULL)),
    ADD CONSTRAINT cliente_fusion_otro CHECK (fusionado_con IS NULL OR fusionado_con <> id);

-- +goose Down
ALTER TABLE customers.cliente DROP COLUMN fusionado_en, DROP COLUMN fusionado_con;
