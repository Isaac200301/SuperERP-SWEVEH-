-- name: InsertCustomer :execrows
-- Si otro usuario registró el mismo celular al mismo tiempo, no inserta (0 filas) y el
-- caso de uso devuelve el existente: nunca hay dos clientes con el mismo celular (RF-04).
INSERT INTO customers.cliente (id, taller_id, celular, nombre, correo, autorizacion_estado, created_at, updated_at, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $7, $8)
ON CONFLICT ON CONSTRAINT cliente_celular_unico DO NOTHING;

-- name: GetCustomerByPhone :one
-- Si el celular es de un cliente que se unió a otro (HU-15), devuelve el que se conservó.
SELECT c.id, c.taller_id, c.celular, c.nombre, c.correo, c.autorizacion_estado, c.version, c.created_at
FROM customers.cliente c
WHERE c.taller_id = sqlc.arg(taller_id) AND c.id = (SELECT COALESCE(x.fusionado_con, x.id) FROM customers.cliente x
    WHERE x.taller_id = sqlc.arg(taller_id) AND x.celular = sqlc.arg(celular));

-- name: GetCustomerByID :one
SELECT id, taller_id, celular, nombre, correo, autorizacion_estado, version, created_at
FROM customers.cliente
WHERE taller_id = $1 AND id = $2;

-- name: ListCustomersByIDs :many
SELECT id, taller_id, celular, nombre, correo, autorizacion_estado, version, created_at
FROM customers.cliente
WHERE taller_id = $1 AND id = ANY(sqlc.arg(ids)::uuid[]);
