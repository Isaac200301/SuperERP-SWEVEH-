-- ADR-0028: el agente responde solo por defecto. Sin fila, el taller queda encendido.

-- +goose Up
CREATE TABLE assistant.ajuste (
    taller_id  uuid PRIMARY KEY REFERENCES tenancy.taller(id),
    auto_envio boolean NOT NULL DEFAULT true,
    updated_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE assistant.ajuste ENABLE ROW LEVEL SECURITY;
ALTER TABLE assistant.ajuste FORCE ROW LEVEL SECURITY;
CREATE POLICY aislamiento_taller ON assistant.ajuste USING (taller_id = tenancy.taller_actual()) WITH CHECK (taller_id = tenancy.taller_actual());
GRANT SELECT, INSERT, UPDATE ON assistant.ajuste TO sweveh_api, sweveh_worker;

-- +goose Down
DROP TABLE assistant.ajuste;
