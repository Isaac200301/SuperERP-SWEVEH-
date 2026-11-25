-- Entrada con celular y PIN de 4 dígitos (HU-104, V-03, ADR-0037). Solo se guarda el hash argon2id del PIN
-- (con sal propia y el pepper del servidor), nunca el PIN. pin_cambiar: el dueño puso un PIN temporal (HU-80,
-- restablecer) y la persona debe cambiarlo al entrar. Los intentos fallidos se cuentan por celular en
-- identity.otp_bloqueo (la misma regla de 5 fallos y 15 minutos de HU-01), exista o no la cuenta.
-- identity.usuario es tabla de autenticación de plataforma, sin taller (Q-29): no lleva RLS.

-- +goose Up
ALTER TABLE identity.usuario ADD COLUMN pin_hash text CHECK (pin_hash IS NULL OR pin_hash LIKE '$argon2id$%');
ALTER TABLE identity.usuario ADD COLUMN pin_cambiar boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE identity.usuario DROP COLUMN pin_cambiar;
ALTER TABLE identity.usuario DROP COLUMN pin_hash;
