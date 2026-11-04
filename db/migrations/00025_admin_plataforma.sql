-- ADR-0030 / HU-74: el administrador del ERP ve el consumo de cada taller. Excepción controlada al aislamiento: el api
-- puede LISTAR los talleres (id y nombre) solo dentro de una transacción en la que el código fijó
-- app.admin_plataforma = 'on' después de comprobar que el usuario es administrador (PLATFORM_ADMINS). Las cifras de
-- cada taller se leen después fijando su app.taller_id, con el aislamiento normal. Sin SECURITY DEFINER ni BYPASSRLS.

-- +goose Up
CREATE POLICY taller_admin_lista ON tenancy.taller FOR SELECT TO sweveh_api
    USING (current_setting('app.admin_plataforma', true) = 'on');

-- +goose Down
DROP POLICY taller_admin_lista ON tenancy.taller;
