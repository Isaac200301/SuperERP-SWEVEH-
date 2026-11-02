-- name: InsertOutboxEvent :exec
INSERT INTO outbox.evento (id, taller_id, tipo, agregado_id, payload, created_at, proximo_intento)
VALUES ($1, $2, $3, $4, $5, $6, $6);

-- name: ClaimOutboxBatch :many
-- Varios workers pueden correr a la vez: SKIP LOCKED reparte los eventos sin repetirlos (R-07).
SELECT id, taller_id, tipo, agregado_id, payload, intentos
FROM outbox.evento
WHERE procesado_en IS NULL AND proximo_intento <= $1
ORDER BY proximo_intento, id
LIMIT $2
FOR UPDATE SKIP LOCKED;

-- name: MarkOutboxProcessed :exec
UPDATE outbox.evento SET procesado_en = $2, intentos = intentos + 1, ultimo_error = NULL WHERE id = $1;

-- name: MarkOutboxFailed :exec
UPDATE outbox.evento SET intentos = intentos + 1, ultimo_error = $2, proximo_intento = $3 WHERE id = $1;
