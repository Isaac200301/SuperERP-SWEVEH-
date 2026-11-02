-- T1: tipo de taller y horario (HU-03, HU-08), contrato (HU-89), servicios (HU-04, HU-07),
-- checklist y fotos (HU-17, HU-18, HU-21), estados y entrega (HU-20, HU-25, HU-26),
-- cotización (HU-27), inventario (HU-32, HU-33, HU-34) y cambio de dueño (HU-12).

-- +goose Up
ALTER TABLE tenancy.taller
    ADD COLUMN atiende text CHECK (atiende IN ('carro', 'moto', 'ambos')),
    ADD COLUMN especialidades text[] NOT NULL DEFAULT '{}',
    ADD COLUMN ciudad text CHECK (ciudad IS NULL OR length(ciudad) <= 80),
    ADD COLUMN tamano text CHECK (tamano IN ('solo', '2a5', '6mas')),
    ADD COLUMN horario jsonb NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN contrato_version text,
    ADD COLUMN contrato_aceptado_en timestamptz,
    ADD COLUMN onboarding_completo boolean NOT NULL DEFAULT false;

CREATE SCHEMA catalog;
CREATE SCHEMA quotes;
CREATE SCHEMA inventory;
GRANT USAGE ON SCHEMA catalog, quotes, inventory TO sweveh_api;

CREATE TABLE catalog.servicio (
    id             uuid PRIMARY KEY,
    taller_id      uuid NOT NULL REFERENCES tenancy.taller(id),
    nombre         text NOT NULL CHECK (length(btrim(nombre)) BETWEEN 1 AND 120),
    tipo_vehiculo  text NOT NULL CHECK (tipo_vehiculo IN ('carro', 'moto', 'ambos')),
    precio         bigint CHECK (precio IS NULL OR precio >= 0),
    duracion_min   integer CHECK (duracion_min IS NULL OR duracion_min > 0),
    activo         boolean NOT NULL DEFAULT true,
    version        integer NOT NULL DEFAULT 1,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX servicio_taller_idx ON catalog.servicio (taller_id, activo, nombre);

ALTER TABLE orders.orden
    ADD COLUMN mecanico_id uuid,
    ADD COLUMN fecha_prometida timestamptz,
    ADD COLUMN ejemplo boolean NOT NULL DEFAULT false,
    ADD COLUMN entregado_a text,
    ADD COLUMN motivo_cancelacion text,
    ADD COLUMN cerrada_en timestamptz;
ALTER TABLE orders.estado_historial ADD COLUMN motivo text;
CREATE INDEX orden_estado_idx ON orders.orden (taller_id, estado);

CREATE TABLE orders.checklist (
    orden_id        uuid PRIMARY KEY REFERENCES orders.orden(id),
    taller_id       uuid NOT NULL REFERENCES tenancy.taller(id),
    tipo            text NOT NULL CHECK (tipo IN ('carro', 'moto')),
    kilometraje     integer CHECK (kilometraje IS NULL OR kilometraje >= 0),
    combustible     text CHECK (combustible IN ('vacio', '1/4', '1/2', '3/4', 'lleno', 'no_aplica')),
    accesorios      text[] NOT NULL DEFAULT '{}',
    salvedades      jsonb NOT NULL DEFAULT '[]'::jsonb,
    completado_por  uuid,
    completado_en   timestamptz,
    version         integer NOT NULL DEFAULT 1,
    updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE orders.evidencia (
    id               uuid PRIMARY KEY,
    taller_id        uuid NOT NULL REFERENCES tenancy.taller(id),
    orden_id         uuid NOT NULL REFERENCES orders.orden(id),
    lado             text CHECK (lado IN ('frente', 'trasera', 'izquierdo', 'derecho', 'tablero', 'otro')),
    nota             text CHECK (nota IS NULL OR length(nota) <= 2000),
    storage_key      text,
    content_type     text,
    mostrar_cliente  boolean NOT NULL DEFAULT false,
    etapa            text NOT NULL,
    autor_id         uuid,
    created_at       timestamptz NOT NULL DEFAULT now(),
    CHECK (storage_key IS NOT NULL OR nota IS NOT NULL)
);
CREATE INDEX evidencia_orden_idx ON orders.evidencia (taller_id, orden_id, created_at);

CREATE TABLE quotes.cotizacion (
    id              uuid PRIMARY KEY,
    taller_id       uuid NOT NULL REFERENCES tenancy.taller(id),
    orden_id        uuid NOT NULL,
    numero          integer NOT NULL DEFAULT 1,
    estado          text NOT NULL CHECK (estado IN ('borrador', 'enviada', 'aprobada', 'rechazada', 'dudas')),
    total           bigint NOT NULL DEFAULT 0 CHECK (total >= 0),
    enviada_en      timestamptz,
    respondida_en   timestamptz,
    canal_respuesta text,
    respuesta_ref   text,
    version         integer NOT NULL DEFAULT 1,
    created_at      timestamptz NOT NULL DEFAULT now(),
    created_by      uuid,
    CONSTRAINT cotizacion_numero_unico UNIQUE (taller_id, orden_id, numero)
);

CREATE TABLE quotes.linea (
    id               uuid PRIMARY KEY,
    taller_id        uuid NOT NULL REFERENCES tenancy.taller(id),
    cotizacion_id    uuid NOT NULL REFERENCES quotes.cotizacion(id) ON DELETE CASCADE,
    tipo             text NOT NULL CHECK (tipo IN ('servicio', 'repuesto')),
    ref_id           uuid,
    descripcion      text NOT NULL CHECK (length(btrim(descripcion)) BETWEEN 1 AND 200),
    cantidad         integer NOT NULL CHECK (cantidad > 0),
    precio_unitario  bigint NOT NULL CHECK (precio_unitario >= 0),
    sin_stock        boolean NOT NULL DEFAULT false,
    aprobada         boolean,
    created_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX linea_cotizacion_idx ON quotes.linea (taller_id, cotizacion_id);

CREATE TABLE inventory.articulo (
    id             uuid PRIMARY KEY,
    taller_id      uuid NOT NULL REFERENCES tenancy.taller(id),
    referencia     text NOT NULL CHECK (referencia ~ '^[A-Z0-9][A-Z0-9-]{0,39}$'),
    nombre         text NOT NULL CHECK (length(btrim(nombre)) BETWEEN 1 AND 120),
    tipo_vehiculo  text NOT NULL DEFAULT 'ambos' CHECK (tipo_vehiculo IN ('carro', 'moto', 'ambos')),
    costo          bigint NOT NULL DEFAULT 0 CHECK (costo >= 0),
    precio_venta   bigint NOT NULL DEFAULT 0 CHECK (precio_venta >= 0),
    existencia     integer NOT NULL DEFAULT 0 CHECK (existencia >= 0),
    minimo         integer NOT NULL DEFAULT 0 CHECK (minimo >= 0),
    activo         boolean NOT NULL DEFAULT true,
    version        integer NOT NULL DEFAULT 1,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT articulo_referencia_unica UNIQUE (taller_id, referencia)
);

CREATE TABLE inventory.movimiento (
    id           uuid PRIMARY KEY,
    taller_id    uuid NOT NULL REFERENCES tenancy.taller(id),
    articulo_id  uuid NOT NULL REFERENCES inventory.articulo(id),
    cantidad     integer NOT NULL CHECK (cantidad <> 0),
    motivo       text NOT NULL CHECK (motivo IN ('entrada', 'salida_orden', 'pendiente_orden', 'devolucion', 'ajuste', 'importacion')),
    orden_id     uuid,
    linea_id     uuid,
    usuario_id   uuid,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX movimiento_articulo_idx ON inventory.movimiento (taller_id, articulo_id, created_at);
-- Idempotencia del descuento: una línea de cotización descuenta una sola vez (R-07).
CREATE UNIQUE INDEX movimiento_linea_unica ON inventory.movimiento (linea_id, motivo) WHERE linea_id IS NOT NULL;

CREATE TABLE vehicles.propietario_historial (
    id                uuid PRIMARY KEY,
    taller_id         uuid NOT NULL REFERENCES tenancy.taller(id),
    vehiculo_id       uuid NOT NULL REFERENCES vehicles.vehiculo(id),
    cliente_anterior  uuid NOT NULL,
    cliente_nuevo     uuid NOT NULL,
    cambiado_en       timestamptz NOT NULL,
    usuario_id        uuid
);

-- RLS y permisos de todas las tablas nuevas (R-06).
-- +goose StatementBegin
DO $$
DECLARE t text;
BEGIN
    FOREACH t IN ARRAY ARRAY['catalog.servicio', 'orders.checklist', 'orders.evidencia', 'quotes.cotizacion',
        'quotes.linea', 'inventory.articulo', 'inventory.movimiento', 'vehicles.propietario_historial']
    LOOP
        EXECUTE format('ALTER TABLE %s ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format('ALTER TABLE %s FORCE ROW LEVEL SECURITY', t);
        EXECUTE format('CREATE POLICY aislamiento_taller ON %s USING (taller_id = tenancy.taller_actual()) WITH CHECK (taller_id = tenancy.taller_actual())', t);
        EXECUTE format('GRANT SELECT, INSERT, UPDATE ON %s TO sweveh_api', t);
    END LOOP;
END $$;
-- +goose StatementEnd
GRANT DELETE ON quotes.linea, catalog.servicio TO sweveh_api;

-- +goose Down
DROP TABLE vehicles.propietario_historial;
DROP SCHEMA inventory CASCADE;
DROP SCHEMA quotes CASCADE;
DROP TABLE orders.evidencia;
DROP TABLE orders.checklist;
ALTER TABLE orders.estado_historial DROP COLUMN motivo;
ALTER TABLE orders.orden DROP COLUMN mecanico_id, DROP COLUMN fecha_prometida, DROP COLUMN ejemplo,
    DROP COLUMN entregado_a, DROP COLUMN motivo_cancelacion, DROP COLUMN cerrada_en;
DROP SCHEMA catalog CASCADE;
ALTER TABLE tenancy.taller DROP COLUMN atiende, DROP COLUMN especialidades, DROP COLUMN ciudad, DROP COLUMN tamano,
    DROP COLUMN horario, DROP COLUMN contrato_version, DROP COLUMN contrato_aceptado_en, DROP COLUMN onboarding_completo;
