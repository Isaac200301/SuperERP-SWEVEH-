-- Ficha del vehículo (decisión de Pablo, 3-nov-2026; reemplaza "motor = cilindraje + turbo"): energía,
-- aspiración, caja, tracción (carro), ciclo y refrigeración (moto), color, carrocería y llanta. Los valores
-- por tipo los valida vehicles/domain; aquí se limita al catálogo completo. `turbo` queda derivado de la
-- aspiración para no romper avisos ni la IA. La tabla ya tiene RLS forzado (00006).

-- +goose Up
ALTER TABLE vehicles.vehiculo
    ADD COLUMN energia       text CHECK (energia IN ('gasolina', 'diesel', 'gas', 'hibrido', 'hibrido_enchufable', 'electrico')),
    ADD COLUMN aspiracion    text CHECK (aspiracion IN ('atmosferico', 'turbo', 'supercargado')),
    ADD COLUMN transmision   text CHECK (transmision IN ('manual', 'automatica', 'cvt', 'doble_embrague', 'amt', 'semiautomatica')),
    ADD COLUMN traccion      text CHECK (traccion IN ('delantera', 'trasera', 'awd', '4x4')),
    ADD COLUMN ciclo         text CHECK (ciclo IN ('2t', '4t')),
    ADD COLUMN refrigeracion text CHECK (refrigeracion IN ('aire', 'liquido')),
    ADD COLUMN color         text CHECK (color IS NULL OR length(color) <= 60),
    ADD COLUMN carroceria    text CHECK (carroceria IN ('sedan', 'hatchback', 'suv', 'pickup', 'van', 'otra')),
    ADD COLUMN llanta        text CHECK (llanta IS NULL OR length(llanta) <= 20),
    ADD CONSTRAINT vehiculo_ficha_por_tipo CHECK (
        (tipo = 'carro' AND ciclo IS NULL AND refrigeracion IS NULL) OR
        (tipo = 'moto' AND traccion IS NULL AND carroceria IS NULL));

-- Lo ya registrado: el turbo da la aspiración; con cilindraje se asume gasolina (editable en la ficha).
UPDATE vehicles.vehiculo SET aspiracion = CASE WHEN turbo THEN 'turbo' ELSE 'atmosferico' END WHERE turbo IS NOT NULL;
UPDATE vehicles.vehiculo SET energia = 'gasolina' WHERE cilindraje_cc IS NOT NULL;

-- +goose Down
ALTER TABLE vehicles.vehiculo
    DROP CONSTRAINT vehiculo_ficha_por_tipo,
    DROP COLUMN energia, DROP COLUMN aspiracion, DROP COLUMN transmision, DROP COLUMN traccion,
    DROP COLUMN ciclo, DROP COLUMN refrigeracion, DROP COLUMN color, DROP COLUMN carroceria, DROP COLUMN llanta;
