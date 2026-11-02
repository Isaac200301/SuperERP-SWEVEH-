-- T2–T4: autorización del cliente (HU-85, HU-86, HU-95), WhatsApp (HU-24, HU-47, HU-101), automatizaciones
-- (HU-40 a HU-48), agente y bandeja (HU-51 a HU-67, HU-99) y rol del Administrador del ERP (HU-74).

-- +goose Up
CREATE TABLE customers.autorizacion (
    id                uuid PRIMARY KEY,
    taller_id         uuid NOT NULL REFERENCES tenancy.taller(id),
    cliente_id        uuid NOT NULL REFERENCES customers.cliente(id),
    estado            text NOT NULL CHECK (estado IN ('autorizada', 'rechazada', 'revocada')),
    texto             text NOT NULL,
    version_politica  text NOT NULL,
    canal             text NOT NULL CHECK (canal IN ('whatsapp', 'mostrador', 'importacion')),
    ref               text,
    ocurrida_en       timestamptz NOT NULL,
    usuario_id        uuid
);
CREATE INDEX autorizacion_cliente_idx ON customers.autorizacion (taller_id, cliente_id, ocurrida_en);

CREATE SCHEMA messaging;
CREATE SCHEMA automation;
CREATE SCHEMA assistant;
GRANT USAGE ON SCHEMA messaging, automation, assistant TO sweveh_api, sweveh_worker;
GRANT USAGE ON SCHEMA orders, customers, vehicles, quotes, inventory, catalog, identity, audit TO sweveh_worker;

-- Número de WhatsApp del taller. El webhook lo encuentra por phone_number_id antes de conocer el taller.
CREATE TABLE messaging.canal (
    id               uuid PRIMARY KEY,
    taller_id        uuid NOT NULL REFERENCES tenancy.taller(id),
    proveedor        text NOT NULL CHECK (proveedor IN ('prueba_local', 'meta_prueba', 'meta')),
    phone_number_id  text NOT NULL,
    numero_visible   text,
    activo           boolean NOT NULL DEFAULT true,
    created_at       timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT canal_phone_unico UNIQUE (phone_number_id)
);

CREATE TABLE messaging.conversacion (
    id                  uuid PRIMARY KEY,
    taller_id           uuid NOT NULL REFERENCES tenancy.taller(id),
    cliente_id          uuid,
    celular             text NOT NULL,
    ultimo_entrante_en  timestamptz,
    modo                text NOT NULL DEFAULT 'agente' CHECK (modo IN ('agente', 'humano')),
    pide_humano         boolean NOT NULL DEFAULT false,
    created_at          timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT conversacion_celular_unica UNIQUE (taller_id, celular)
);

CREATE TABLE messaging.mensaje (
    id               uuid PRIMARY KEY,
    taller_id        uuid NOT NULL REFERENCES tenancy.taller(id),
    conversacion_id  uuid NOT NULL REFERENCES messaging.conversacion(id),
    direccion        text NOT NULL CHECK (direccion IN ('entrante', 'saliente')),
    wamid            text,
    tipo             text NOT NULL CHECK (tipo IN ('texto', 'botones', 'plantilla', 'audio', 'imagen', 'respuesta_boton')),
    cuerpo           text,
    payload          jsonb NOT NULL DEFAULT '{}'::jsonb,
    estado           text NOT NULL CHECK (estado IN ('recibido', 'pendiente', 'enviado', 'entregado', 'leido', 'fallido')),
    origen           text NOT NULL CHECK (origen IN ('cliente', 'automatizacion', 'agente', 'persona')),
    categoria        text CHECK (categoria IN ('servicio', 'utilidad', 'marketing', 'autenticacion')),
    orden_id         uuid,
    automatizacion   text,
    error            text,
    intentos         integer NOT NULL DEFAULT 0,
    created_at       timestamptz NOT NULL DEFAULT now(),
    enviado_en       timestamptz
);
-- Idempotencia del webhook: un wamid se guarda una sola vez (R-07, R-09).
CREATE UNIQUE INDEX mensaje_wamid_unico ON messaging.mensaje (wamid) WHERE wamid IS NOT NULL;
CREATE INDEX mensaje_conversacion_idx ON messaging.mensaje (taller_id, conversacion_id, created_at);
CREATE INDEX mensaje_orden_idx ON messaging.mensaje (taller_id, orden_id) WHERE orden_id IS NOT NULL;
CREATE INDEX mensaje_pendiente_idx ON messaging.mensaje (created_at) WHERE estado = 'pendiente';

-- Cuerpo crudo del webhook: se guarda antes de saber el taller y lo procesa el worker.
CREATE TABLE messaging.webhook_evento (
    id            uuid PRIMARY KEY,
    recibido_en   timestamptz NOT NULL DEFAULT now(),
    cuerpo        jsonb NOT NULL,
    procesado_en  timestamptz,
    error         text
);
CREATE INDEX webhook_pendiente_idx ON messaging.webhook_evento (recibido_en) WHERE procesado_en IS NULL;

CREATE TABLE automation.regla (
    taller_id  uuid NOT NULL REFERENCES tenancy.taller(id),
    codigo     text NOT NULL CHECK (codigo ~ '^AU-0[1-9]$'),
    activa     boolean NOT NULL,
    texto      text CHECK (texto IS NULL OR length(texto) <= 1000),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (taller_id, codigo)
);

CREATE TABLE automation.ejecucion (
    id          uuid PRIMARY KEY,
    taller_id   uuid NOT NULL REFERENCES tenancy.taller(id),
    codigo      text NOT NULL,
    clave       text NOT NULL,
    orden_id    uuid,
    cliente_id  uuid,
    resultado   text NOT NULL CHECK (resultado IN ('enviado', 'omitido', 'fallido')),
    detalle     text,
    mensaje_id  uuid,
    created_at  timestamptz NOT NULL DEFAULT now(),
    -- Una ejecución por evento y automatización (los eventos llegan "al menos una vez").
    CONSTRAINT ejecucion_unica UNIQUE (taller_id, codigo, clave)
);
CREATE INDEX ejecucion_fecha_idx ON automation.ejecucion (taller_id, codigo, created_at);

CREATE TABLE assistant.borrador (
    id                  uuid PRIMARY KEY,
    taller_id           uuid NOT NULL REFERENCES tenancy.taller(id),
    conversacion_id     uuid NOT NULL,
    mensaje_entrante_id uuid NOT NULL,
    celular             text NOT NULL,
    texto_entrante      text,
    intencion           text NOT NULL,
    texto_propuesto     text NOT NULL,
    texto_final         text,
    fuentes             jsonb NOT NULL DEFAULT '[]'::jsonb,
    accion              jsonb,
    estado              text NOT NULL CHECK (estado IN ('pendiente', 'enviado', 'editado_enviado', 'lo_contesto_yo', 'descartado', 'auto_enviado')),
    origen              text NOT NULL CHECK (origen IN ('ia', 'plantilla')),
    created_at          timestamptz NOT NULL DEFAULT now(),
    resuelto_en         timestamptz,
    resuelto_por        uuid,
    CONSTRAINT borrador_por_mensaje UNIQUE (mensaje_entrante_id)
);
CREATE INDEX borrador_pendiente_idx ON assistant.borrador (taller_id, estado, created_at);

CREATE TABLE assistant.llamada_ia (
    id              uuid PRIMARY KEY,
    taller_id       uuid NOT NULL REFERENCES tenancy.taller(id),
    proveedor       text NOT NULL,
    modelo          text NOT NULL,
    proposito       text NOT NULL,
    tokens_entrada  integer NOT NULL DEFAULT 0,
    tokens_salida   integer NOT NULL DEFAULT 0,
    costo_micro_usd bigint NOT NULL DEFAULT 0,
    latencia_ms     integer NOT NULL DEFAULT 0,
    resultado       text NOT NULL CHECK (resultado IN ('ok', 'invalido', 'error', 'tope')),
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX llamada_ia_fecha_idx ON assistant.llamada_ia (taller_id, created_at);

-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
    FOREACH t IN ARRAY ARRAY['customers.autorizacion', 'messaging.canal', 'messaging.conversacion', 'messaging.mensaje',
        'automation.regla', 'automation.ejecucion', 'assistant.borrador', 'assistant.llamada_ia']
    LOOP
        EXECUTE format('ALTER TABLE %s ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format('ALTER TABLE %s FORCE ROW LEVEL SECURITY', t);
        EXECUTE format('CREATE POLICY aislamiento_taller ON %s USING (taller_id = tenancy.taller_actual()) WITH CHECK (taller_id = tenancy.taller_actual())', t);
        EXECUTE format('GRANT SELECT, INSERT, UPDATE ON %s TO sweveh_api, sweveh_worker', t);
    END LOOP;
END $$;
-- +goose StatementEnd

-- El worker resuelve el taller de un mensaje entrante por phone_number_id (docs/06 §5.3, excepción
-- controlada): solo se ve la fila del canal cuyo número se fijó en app.phone_number_id. Sin SECURITY DEFINER.
CREATE POLICY canal_por_numero ON messaging.canal FOR SELECT
    USING (phone_number_id = NULLIF(current_setting('app.phone_number_id', true), ''));

GRANT INSERT ON messaging.webhook_evento TO sweveh_api;
GRANT SELECT, UPDATE ON messaging.webhook_evento TO sweveh_worker;

-- El worker procesa eventos de todos los talleres: fija app.taller_id del evento antes de tocar datos,
-- así que comparte la política normal. Necesita leer y escribir las tablas de negocio con ese taller.
GRANT SELECT, INSERT, UPDATE ON ALL TABLES IN SCHEMA orders, customers, vehicles, quotes, inventory, catalog, audit TO sweveh_worker;
GRANT SELECT ON tenancy.taller TO sweveh_worker;
GRANT SELECT ON identity.membresia, identity.usuario TO sweveh_worker;
GRANT EXECUTE ON FUNCTION tenancy.taller_actual() TO sweveh_worker;
-- Los consumidores publican eventos nuevos (p. ej. "enviar mensaje") desde el worker.
GRANT INSERT ON outbox.evento TO sweveh_worker;
-- Las tareas diarias (AU-06, AU-07) recorren los talleres: el worker ve la lista de ids y luego trabaja
-- cada taller con su app.taller_id (todas las demás tablas siguen aisladas).
CREATE POLICY taller_worker_lista ON tenancy.taller FOR SELECT TO sweveh_worker USING (true);

-- +goose Down
REVOKE SELECT, INSERT, UPDATE ON ALL TABLES IN SCHEMA orders, customers, vehicles, quotes, inventory, catalog, audit FROM sweveh_worker;
DROP SCHEMA assistant CASCADE;
DROP SCHEMA automation CASCADE;
DROP SCHEMA messaging CASCADE;
DROP TABLE customers.autorizacion;
