-- Cotización real (parámetros del taller): IVA y si los precios lo incluyen (copiados al crear), vigencia, desglose
-- guardado (subtotal, descuento, IVA y total), descuento por línea y mano de obra por horas. Las cotizaciones que ya
-- existían quedan sin IVA (0 %) y con su total de siempre.

-- +goose Up
ALTER TABLE quotes.cotizacion
    ADD COLUMN iva_pct         integer NOT NULL DEFAULT 0 CHECK (iva_pct BETWEEN 0 AND 30),
    ADD COLUMN precios_con_iva boolean NOT NULL DEFAULT true,
    ADD COLUMN vence_en        timestamptz,
    ADD COLUMN subtotal        bigint NOT NULL DEFAULT 0 CHECK (subtotal >= 0),
    ADD COLUMN descuento       bigint NOT NULL DEFAULT 0 CHECK (descuento >= 0),
    ADD COLUMN iva             bigint NOT NULL DEFAULT 0 CHECK (iva >= 0);
UPDATE quotes.cotizacion SET subtotal = total;

ALTER TABLE quotes.linea
    ADD COLUMN descuento_pct integer NOT NULL DEFAULT 0 CHECK (descuento_pct BETWEEN 0 AND 100),
    ADD COLUMN horas_cent    integer CHECK (horas_cent IS NULL OR horas_cent BETWEEN 25 AND 20000);

-- +goose Down
ALTER TABLE quotes.linea DROP COLUMN descuento_pct, DROP COLUMN horas_cent;
ALTER TABLE quotes.cotizacion DROP COLUMN iva_pct, DROP COLUMN precios_con_iva, DROP COLUMN vence_en,
    DROP COLUMN subtotal, DROP COLUMN descuento, DROP COLUMN iva;
