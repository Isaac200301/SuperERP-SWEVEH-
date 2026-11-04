-- Estados opcionales por taller (decisión de Pablo, 3-nov-2026; amplía Q-04): Esperando repuesto y Control de calidad.
-- Vienen apagados en tenancy.parametros, así el flujo de siempre no cambia.

-- +goose Up
ALTER TABLE orders.orden DROP CONSTRAINT orden_estado_check;
ALTER TABLE orders.orden ADD CONSTRAINT orden_estado_check CHECK (estado IN ('por_recibir', 'recibido', 'en_diagnostico',
    'esperando_aprobacion', 'en_reparacion', 'esperando_repuesto', 'control_calidad', 'listo', 'entregado', 'cancelada'));

-- +goose Down
UPDATE orders.orden SET estado = 'en_reparacion' WHERE estado IN ('esperando_repuesto', 'control_calidad');
ALTER TABLE orders.orden DROP CONSTRAINT orden_estado_check;
ALTER TABLE orders.orden ADD CONSTRAINT orden_estado_check CHECK (estado IN ('por_recibir', 'recibido', 'en_diagnostico',
    'esperando_aprobacion', 'en_reparacion', 'listo', 'entregado', 'cancelada'));
