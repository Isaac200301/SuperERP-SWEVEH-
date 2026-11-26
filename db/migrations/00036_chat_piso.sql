-- Asistente del piso (R8, V-11, ADR-0041, HU-107): la conversación del mecánico con el asistente interno del taller.
-- Empieza en un chat general (orden_id null) y, cuando hay placa, se asocia a la orden y sigue dentro de ella. La
-- propuesta pendiente vive en el chat (con versión optimista, R-07) y solo se aplica a la orden al Confirmar. Cada
-- confirmación guarda su Idempotency-Key: repetirla devuelve el mismo resultado sin aplicar dos veces.

-- +goose Up
CREATE TABLE assistant.chat_piso (
    id           uuid PRIMARY KEY,
    taller_id    uuid NOT NULL REFERENCES tenancy.taller(id),
    usuario_id   uuid NOT NULL,
    orden_id     uuid REFERENCES orders.orden(id),
    propuesta    jsonb NOT NULL DEFAULT '{}'::jsonb,
    version      integer NOT NULL DEFAULT 1,
    creado_en    timestamptz NOT NULL DEFAULT now(),
    actualizado_en timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX chat_piso_orden_idx ON assistant.chat_piso (taller_id, orden_id, actualizado_en);
CREATE INDEX chat_piso_usuario_idx ON assistant.chat_piso (taller_id, usuario_id, actualizado_en);

CREATE TABLE assistant.chat_piso_mensaje (
    id         uuid PRIMARY KEY,
    taller_id  uuid NOT NULL REFERENCES tenancy.taller(id),
    chat_id    uuid NOT NULL REFERENCES assistant.chat_piso(id),
    rol        text NOT NULL CHECK (rol IN ('usuario', 'asistente')),
    texto      text NOT NULL CHECK (length(texto) <= 4000),
    adjuntos   jsonb NOT NULL DEFAULT '[]'::jsonb,
    opciones   jsonb NOT NULL DEFAULT '[]'::jsonb,
    propuesta  jsonb,
    creado_en  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX chat_piso_mensaje_chat_idx ON assistant.chat_piso_mensaje (taller_id, chat_id, creado_en);

CREATE TABLE assistant.chat_piso_confirmacion (
    taller_id  uuid NOT NULL REFERENCES tenancy.taller(id),
    clave      text NOT NULL CHECK (length(clave) BETWEEN 8 AND 100),
    chat_id    uuid NOT NULL REFERENCES assistant.chat_piso(id),
    resultado  jsonb NOT NULL,
    creado_por uuid NOT NULL,
    creado_en  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (taller_id, clave)
);

-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
    FOREACH t IN ARRAY ARRAY['assistant.chat_piso', 'assistant.chat_piso_mensaje', 'assistant.chat_piso_confirmacion']
    LOOP
        EXECUTE format('ALTER TABLE %s ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format('ALTER TABLE %s FORCE ROW LEVEL SECURITY', t);
        EXECUTE format('CREATE POLICY aislamiento_taller ON %s USING (taller_id = tenancy.taller_actual()) WITH CHECK (taller_id = tenancy.taller_actual())', t);
    END LOOP;
END $$;
-- +goose StatementEnd

-- Sin DELETE: la conversación queda como historia de la orden. El worker con los mismos privilegios que el api (00019).
GRANT SELECT, INSERT, UPDATE ON assistant.chat_piso, assistant.chat_piso_mensaje TO sweveh_api, sweveh_worker;
GRANT SELECT, INSERT ON assistant.chat_piso_confirmacion TO sweveh_api, sweveh_worker;

-- +goose Down
DROP TABLE assistant.chat_piso_confirmacion;
DROP TABLE assistant.chat_piso_mensaje;
DROP TABLE assistant.chat_piso;
