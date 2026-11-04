-- El worker ejecuta los mismos casos de uso que el api cuando procesa WhatsApp (aprobar la cotización desde el
-- botón, HU-28; recibir con los parámetros del taller; descontar inventario, HU-33). En 00009 recibió permisos
-- sobre las tablas de ese momento y sin DELETE; las tablas nuevas (hallazgos, parámetros) no le dieron nada, así
-- que aprobar desde WhatsApp fallaba con "permission denied for table linea". Desde aquí el worker tiene en las
-- tablas de negocio exactamente los privilegios del api. El aislamiento sigue igual: las políticas RLS aplican a
-- los dos roles (tenancy.taller_actual()). La prueba TestWorkerHasAPIPrivileges evita que se repita.

-- +goose Up
-- +goose StatementBegin
DO $$
DECLARE g record;
BEGIN
    FOR g IN
        SELECT table_schema, table_name, privilege_type
        FROM information_schema.role_table_grants
        WHERE grantee = 'sweveh_api'
          AND table_schema IN ('orders', 'customers', 'vehicles', 'quotes', 'inventory', 'catalog', 'tenancy')
          AND NOT (table_schema = 'tenancy' AND table_name = 'taller')
          AND privilege_type IN ('SELECT', 'INSERT', 'UPDATE', 'DELETE')
    LOOP
        EXECUTE format('GRANT %s ON %I.%I TO sweveh_worker', g.privilege_type, g.table_schema, g.table_name);
    END LOOP;
END $$;
-- +goose StatementEnd

-- +goose Down
REVOKE DELETE ON quotes.linea, catalog.servicio, orders.hallazgo FROM sweveh_worker;
REVOKE ALL ON orders.hallazgo, tenancy.parametros FROM sweveh_worker;
