-- Identidad: usuarios por celular, membresías por taller, OTP y sesiones (HU-01, HU-02, docs/06 §6).
-- usuario, otp_* y sesion/refresh_token son tablas de autenticación de plataforma: se consultan
-- antes de conocer el taller (inicio de sesión). Solo las toca el módulo identity.
-- Decisión provisional registrada como Q-29 en docs/07.

-- +goose Up
CREATE TABLE identity.usuario (
    id          uuid PRIMARY KEY,
    celular     text NOT NULL CHECK (celular ~ '^\+573[0-9]{9}$'),
    nombre      text NOT NULL CHECK (length(btrim(nombre)) BETWEEN 1 AND 120),
    activo      boolean NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT usuario_celular_unico UNIQUE (celular)
);

-- Rol del usuario en un taller. Visible para su taller o para el propio usuario al iniciar sesión.
CREATE TABLE identity.membresia (
    taller_id   uuid NOT NULL REFERENCES tenancy.taller(id),
    usuario_id  uuid NOT NULL REFERENCES identity.usuario(id),
    rol         text NOT NULL CHECK (rol IN ('dueno', 'recepcion', 'mecanico')),
    activo      boolean NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (taller_id, usuario_id)
);
CREATE INDEX membresia_usuario_idx ON identity.membresia (usuario_id);

ALTER TABLE identity.membresia ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity.membresia FORCE ROW LEVEL SECURITY;
CREATE POLICY aislamiento_taller ON identity.membresia
    USING (taller_id = tenancy.taller_actual() OR usuario_id = identity.usuario_actual())
    WITH CHECK (taller_id = tenancy.taller_actual());

-- Código de un solo uso. Solo se guarda el HMAC del código, nunca el código.
CREATE TABLE identity.otp_desafio (
    id           uuid PRIMARY KEY,
    celular      text NOT NULL CHECK (celular ~ '^\+573[0-9]{9}$'),
    codigo_hash  bytea NOT NULL,
    expira_en    timestamptz NOT NULL,
    usado_en     timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX otp_desafio_celular_idx ON identity.otp_desafio (celular, created_at DESC);

-- Intentos fallidos por celular: 5 fallos bloquean 15 minutos (HU-01).
CREATE TABLE identity.otp_bloqueo (
    celular          text PRIMARY KEY,
    fallos           integer NOT NULL DEFAULT 0 CHECK (fallos >= 0),
    bloqueado_hasta  timestamptz,
    updated_at       timestamptz NOT NULL DEFAULT now()
);

-- Una sesión es una familia de refresh tokens; si uno se reutiliza, se revoca la familia.
CREATE TABLE identity.sesion (
    id                 uuid PRIMARY KEY,
    usuario_id         uuid NOT NULL REFERENCES identity.usuario(id),
    taller_id          uuid NOT NULL REFERENCES tenancy.taller(id),
    created_at         timestamptz NOT NULL DEFAULT now(),
    expira_absoluto    timestamptz NOT NULL,
    revocada_en        timestamptz,
    motivo_revocacion  text
);
CREATE INDEX sesion_usuario_idx ON identity.sesion (usuario_id);

CREATE TABLE identity.refresh_token (
    hash        bytea PRIMARY KEY,
    sesion_id   uuid NOT NULL REFERENCES identity.sesion(id),
    expira_en   timestamptz NOT NULL,
    usado_en    timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX refresh_token_sesion_idx ON identity.refresh_token (sesion_id);

-- Prueba de la autorización del dueño sobre sus propios datos (HU-02, Ley 1581, Decreto 1377 art. 8).
CREATE TABLE identity.autorizacion_titular (
    id               uuid PRIMARY KEY,
    usuario_id       uuid NOT NULL REFERENCES identity.usuario(id),
    texto            text NOT NULL,
    version_politica text NOT NULL,
    canal            text NOT NULL CHECK (canal IN ('web', 'whatsapp', 'mostrador')),
    aceptada_en      timestamptz NOT NULL
);
CREATE INDEX autorizacion_titular_usuario_idx ON identity.autorizacion_titular (usuario_id);

GRANT SELECT, INSERT, UPDATE ON identity.usuario TO sweveh_api;
GRANT SELECT, INSERT, UPDATE ON identity.membresia TO sweveh_api;
GRANT SELECT, INSERT, UPDATE ON identity.otp_desafio TO sweveh_api;
GRANT SELECT, INSERT, UPDATE ON identity.otp_bloqueo TO sweveh_api;
GRANT SELECT, INSERT, UPDATE ON identity.sesion TO sweveh_api;
GRANT SELECT, INSERT, UPDATE ON identity.refresh_token TO sweveh_api;
GRANT SELECT, INSERT ON identity.autorizacion_titular TO sweveh_api;

-- +goose Down
DROP TABLE identity.autorizacion_titular;
DROP TABLE identity.refresh_token;
DROP TABLE identity.sesion;
DROP TABLE identity.otp_bloqueo;
DROP TABLE identity.otp_desafio;
DROP TABLE identity.membresia;
DROP TABLE identity.usuario;
