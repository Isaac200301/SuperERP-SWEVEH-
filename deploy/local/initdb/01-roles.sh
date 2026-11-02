#!/bin/sh
# Crea los roles y la base local de SWEVEH la primera vez que arranca el contenedor.
# Mismo contenido que db/bootstrap/roles.sql, con la contraseña de desarrollo del compose.
set -eu

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres \
  -v migrator_pw="$SWEVEH_ROLE_PASSWORD" -v api_pw="$SWEVEH_ROLE_PASSWORD" -v worker_pw="$SWEVEH_ROLE_PASSWORD" \
  -f /docker-entrypoint-initdb.d/roles.sql.inc

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres <<-SQL
	CREATE DATABASE sweveh OWNER sweveh_migrator;
	REVOKE ALL ON DATABASE sweveh FROM PUBLIC;
	GRANT CONNECT ON DATABASE sweveh TO sweveh_api, sweveh_worker;
SQL
