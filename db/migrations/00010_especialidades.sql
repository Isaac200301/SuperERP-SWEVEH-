-- Especialidades del paso 2 (HU-03): "electrico" se separa en sistema eléctrico y alto voltaje,
-- y "diagnostico_escaner" pasa a diagnóstico electrónico. Ver tenancy/domain.Specialties.

-- +goose Up
UPDATE tenancy.taller
SET especialidades = array_replace(array_replace(especialidades, 'electrico', 'sistema_electrico'),
                                   'diagnostico_escaner', 'diagnostico_electronico')
WHERE especialidades && ARRAY['electrico', 'diagnostico_escaner'];

-- +goose Down
UPDATE tenancy.taller
SET especialidades = array_replace(array_replace(especialidades, 'sistema_electrico', 'electrico'),
                                   'diagnostico_electronico', 'diagnostico_escaner')
WHERE especialidades && ARRAY['sistema_electrico', 'diagnostico_electronico'];
