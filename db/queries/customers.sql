-- name: InsertCustomer :execrows
-- Si otro usuario registró el mismo celular al mismo tiempo, no inserta (0 filas) y el
-- caso de uso devuelve el existente: nunca hay dos clientes con el mismo celular (RF-04).
INSERT INTO customers.cliente (id, taller_id, celular, nombre, correo, autorizacion_estado, created_at, updated_at, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $7, $8)
ON CONFLICT ON CONSTRAINT cliente_celular_unico DO NOTHING;

-- name: GetCustomerByPhone :one
SELECT id, taller_id, celular, nombre, correo, autorizacion_estado, version, created_at
FROM customers.cliente
WHERE taller_id = $1 AND celular = $2;

-- name: GetCustomerByID :one
SELECT id, taller_id, celular, nombre, correo, autorizacion_estado, version, created_at
FROM customers.cliente
WHERE taller_id = $1 AND id = $2;

-- name: ListCustomersByIDs :many
SELECT id, taller_id, celular, nombre, correo, autorizacion_estado, version, created_at
FROM customers.cliente
WHERE taller_id = $1 AND id = ANY(sqlc.arg(ids)::uuid[]);
