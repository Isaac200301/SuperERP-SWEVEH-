-- name: InsertVehicle :execrows
-- Igual que con el cliente: placa repetida en el mismo taller = 0 filas, se devuelve el existente (HU-10).
INSERT INTO vehicles.vehiculo (id, taller_id, cliente_id, tipo, placa, formato_placa, marca, linea,
                               modelo_anio, kilometraje, cilindraje_cc, turbo, created_at, updated_at, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $13, $14)
ON CONFLICT ON CONSTRAINT vehiculo_placa_unica DO NOTHING;

-- name: GetVehicleByPlate :one
SELECT id, taller_id, cliente_id, tipo, placa, formato_placa, marca, linea, modelo_anio, kilometraje,
       cilindraje_cc, turbo, version, created_at
FROM vehicles.vehiculo
WHERE taller_id = $1 AND placa = $2;

-- name: GetVehicleByID :one
SELECT id, taller_id, cliente_id, tipo, placa, formato_placa, marca, linea, modelo_anio, kilometraje,
       cilindraje_cc, turbo, version, created_at
FROM vehicles.vehiculo
WHERE taller_id = $1 AND id = $2;

-- name: ListVehiclesByIDs :many
SELECT id, taller_id, cliente_id, tipo, placa, formato_placa, marca, linea, modelo_anio, kilometraje,
       cilindraje_cc, turbo, version, created_at
FROM vehicles.vehiculo
WHERE taller_id = $1 AND id = ANY(sqlc.arg(ids)::uuid[]);

-- name: UpdateVehicleEngine :execrows
-- Concurrencia optimista (R-07): solo actualiza si nadie cambió la versión.
UPDATE vehicles.vehiculo
SET cilindraje_cc = $3, turbo = $4, version = version + 1, updated_at = $5
WHERE taller_id = $1 AND id = $2 AND version = $6;
