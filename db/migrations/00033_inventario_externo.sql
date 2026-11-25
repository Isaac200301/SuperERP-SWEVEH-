-- Inventario apagado (V-06, ADR-0039, HU-110): "Llevo el inventario en otro programa". Con true los repuestos se
-- escriben a mano en la cotización, al aprobar no se descuentan existencias y /parts no se escribe. false por
-- defecto: los talleres que ya existen siguen igual; el registro nuevo lo propone marcado (Q-41).

-- +goose Up
ALTER TABLE tenancy.parametros ADD COLUMN inventario_externo boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE tenancy.parametros DROP COLUMN inventario_externo;
