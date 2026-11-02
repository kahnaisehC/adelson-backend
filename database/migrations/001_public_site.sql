-- Migración para el sitio público.
-- Aplicar sobre bases existentes creadas con database/setup.sql.

ALTER TYPE tipo_inmueble_enum ADD VALUE IF NOT EXISTS 'Campo';

ALTER TABLE consultas ALTER COLUMN inmueble_id DROP NOT NULL;
