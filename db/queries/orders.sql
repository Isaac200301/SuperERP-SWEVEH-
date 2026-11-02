-- name: NextOrderNumber :one
-- Contador por taller (R-07). El UPSERT bloquea la fila del taller hasta el fin de la
-- transacción, así dos órdenes simultáneas nunca reciben el mismo número.
INSERT INTO orders.contador (taller_id, ultimo) VALUES ($1, 1)
ON CONFLICT (taller_id) DO UPDATE SET ultimo = orders.contador.ultimo + 1
RETURNING ultimo;

-- name: InsertOrder :exec
INSERT INTO orders.orden (id, taller_id, numero, vehiculo_id, cliente_id, placa, tipo_vehiculo, estado,
                          sintoma, created_at, updated_at, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10, $11);

-- name: InsertStateHistory :exec
INSERT INTO orders.estado_historial (id, taller_id, orden_id, desde, hacia, usuario_id, ocurrido_en)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: CountOpenOrdersByVehicle :one
SELECT count(*) FROM orders.orden
WHERE taller_id = $1 AND vehiculo_id = $2 AND estado NOT IN ('entregado', 'cancelada');

-- name: GetOrder :one
SELECT id, taller_id, numero, vehiculo_id, cliente_id, placa, tipo_vehiculo, estado, sintoma,
       version, created_at, created_by
FROM orders.orden
WHERE taller_id = $1 AND id = $2;

-- name: ListRecentOrders :many
-- Paginación por cursor (created_at, id), no por OFFSET (docs/08 §4).
SELECT id, taller_id, numero, vehiculo_id, cliente_id, placa, tipo_vehiculo, estado, sintoma,
       version, created_at, created_by
FROM orders.orden
WHERE taller_id = $1
  AND (sqlc.narg(cursor_at)::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg(cursor_at)::timestamptz, sqlc.narg(cursor_id)::uuid))
ORDER BY created_at DESC, id DESC
LIMIT $2;

-- name: ListStateHistory :many
SELECT desde, hacia, usuario_id, ocurrido_en FROM orders.estado_historial
WHERE taller_id = $1 AND orden_id = $2
ORDER BY ocurrido_en, id;
