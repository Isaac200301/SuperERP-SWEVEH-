-- name: InsertAuditEvent :exec
INSERT INTO audit.evento (id, taller_id, actor_tipo, actor_id, accion, entidad, entidad_id, antes, despues,
                          request_id, ocurrido_en)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);

-- name: ListAuditByEntity :many
SELECT id, actor_tipo, actor_id, accion, entidad, entidad_id, antes, despues, ocurrido_en
FROM audit.evento
WHERE taller_id = $1 AND entidad = $2 AND entidad_id = $3
ORDER BY ocurrido_en, id;
