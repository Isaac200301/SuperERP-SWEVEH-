-- name: GetUserByPhone :one
SELECT id, celular, nombre, activo FROM identity.usuario WHERE celular = $1;

-- name: GetUserByID :one
SELECT id, celular, nombre, activo FROM identity.usuario WHERE id = $1;

-- name: InsertUser :exec
INSERT INTO identity.usuario (id, celular, nombre, created_at, updated_at)
VALUES ($1, $2, $3, $4, $4);

-- name: InsertMembership :exec
INSERT INTO identity.membresia (taller_id, usuario_id, rol, created_at)
VALUES ($1, $2, $3, $4);

-- name: ListActiveMembershipsByUser :many
-- Requiere app.usuario_id (política de membresia).
SELECT taller_id, rol FROM identity.membresia
WHERE usuario_id = $1 AND activo
ORDER BY created_at, taller_id;

-- name: GetActiveMembership :one
SELECT rol FROM identity.membresia
WHERE taller_id = $1 AND usuario_id = $2 AND activo;

-- name: InsertOTPChallenge :exec
INSERT INTO identity.otp_desafio (id, celular, codigo_hash, expira_en, created_at)
VALUES ($1, $2, $3, $4, $5);

-- name: GetLatestOpenOTPChallenge :one
SELECT id, celular, codigo_hash, expira_en, usado_en, created_at FROM identity.otp_desafio
WHERE celular = $1 AND usado_en IS NULL
ORDER BY created_at DESC
LIMIT 1;

-- name: GetLatestOTPChallengeTime :one
SELECT created_at FROM identity.otp_desafio
WHERE celular = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: CloseOpenOTPChallenges :exec
UPDATE identity.otp_desafio SET usado_en = $2
WHERE celular = $1 AND usado_en IS NULL;

-- name: EnsureOTPLock :exec
INSERT INTO identity.otp_bloqueo (celular) VALUES ($1)
ON CONFLICT (celular) DO NOTHING;

-- name: GetOTPLockForUpdate :one
SELECT fallos, bloqueado_hasta FROM identity.otp_bloqueo WHERE celular = $1 FOR UPDATE;

-- name: GetOTPLock :one
SELECT fallos, bloqueado_hasta FROM identity.otp_bloqueo WHERE celular = $1;

-- name: UpdateOTPLock :exec
UPDATE identity.otp_bloqueo SET fallos = $2, bloqueado_hasta = $3, updated_at = $4
WHERE celular = $1;

-- name: InsertSession :exec
INSERT INTO identity.sesion (id, usuario_id, taller_id, created_at, expira_absoluto)
VALUES ($1, $2, $3, $4, $5);

-- name: RevokeSession :exec
UPDATE identity.sesion SET revocada_en = $2, motivo_revocacion = $3
WHERE id = $1 AND revocada_en IS NULL;

-- name: InsertRefreshToken :exec
INSERT INTO identity.refresh_token (hash, sesion_id, expira_en, created_at)
VALUES ($1, $2, $3, $4);

-- name: GetRefreshTokenForUpdate :one
-- Bloquea la fila: dos refresh simultáneos con el mismo token no pueden rotar los dos.
SELECT r.hash, r.sesion_id, r.expira_en, r.usado_en,
       s.usuario_id, s.taller_id, s.expira_absoluto, s.revocada_en
FROM identity.refresh_token r
JOIN identity.sesion s ON s.id = r.sesion_id
WHERE r.hash = $1
FOR UPDATE OF r;

-- name: MarkRefreshTokenUsed :exec
UPDATE identity.refresh_token SET usado_en = $2 WHERE hash = $1;

-- name: InsertConsent :exec
INSERT INTO identity.autorizacion_titular (id, usuario_id, texto, version_politica, canal, aceptada_en)
VALUES ($1, $2, $3, $4, $5, $6);
