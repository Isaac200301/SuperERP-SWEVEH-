-- HU-60: la tarjeta de "Por revisar" muestra el audio junto a la transcripción cuando el cliente mandó una nota de voz.

-- +goose Up
ALTER TABLE assistant.borrador ADD COLUMN nota_de_voz boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE assistant.borrador DROP COLUMN nota_de_voz;
