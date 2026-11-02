-- name: InsertWorkshop :exec
INSERT INTO tenancy.taller (id, nombre, created_at, updated_at)
VALUES ($1, $2, $3, $3);

-- name: GetWorkshop :one
SELECT id, nombre, version FROM tenancy.taller WHERE id = $1;
