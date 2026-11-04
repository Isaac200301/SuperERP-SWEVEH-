-- ADR-0027: WhatsApp de prueba por WAHA (no oficial, solo demo). El número del taller queda como "waha:<sesión>".

-- +goose Up
ALTER TABLE messaging.canal DROP CONSTRAINT canal_proveedor_check;
ALTER TABLE messaging.canal ADD CONSTRAINT canal_proveedor_check CHECK (proveedor IN ('prueba_local', 'meta_prueba', 'meta', 'waha'));

-- +goose Down
UPDATE messaging.canal SET activo = false WHERE proveedor = 'waha';
DELETE FROM messaging.canal WHERE proveedor = 'waha';
ALTER TABLE messaging.canal DROP CONSTRAINT canal_proveedor_check;
ALTER TABLE messaging.canal ADD CONSTRAINT canal_proveedor_check CHECK (proveedor IN ('prueba_local', 'meta_prueba', 'meta'));
