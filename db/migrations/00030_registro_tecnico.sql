-- HU-100: el administrador del ERP busca por número qué entró y salió por el webhook. Los cuerpos crudos se guardan
-- antes de saber el taller (no tienen taller_id ni RLS); el api solo los lee desde el registro técnico de plataforma
-- (PLATFORM_ADMINS, ADR-0030). Los mensajes se siguen leyendo taller por taller, con el aislamiento normal.

-- +goose Up
GRANT SELECT ON messaging.webhook_evento TO sweveh_api;

-- +goose Down
REVOKE SELECT ON messaging.webhook_evento FROM sweveh_api;
