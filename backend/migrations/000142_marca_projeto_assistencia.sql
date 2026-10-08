-- +goose Up
-- Marca do Projeto-Assistencia-v2 (ADR 030): o nome padrão do branding passa
-- a ser o do produto. Só troca o valor que ainda é o padrão antigo — um nome
-- já personalizado pela tela de Branding é preservado.
ALTER TABLE branding_settings ALTER COLUMN app_name SET DEFAULT 'Projeto-Assistencia-v2';
UPDATE branding_settings SET app_name = 'Projeto-Assistencia-v2' WHERE app_name = 'Projeto Nexus';

-- +goose Down
ALTER TABLE branding_settings ALTER COLUMN app_name SET DEFAULT 'Projeto Nexus';
UPDATE branding_settings SET app_name = 'Projeto Nexus' WHERE app_name = 'Projeto-Assistencia-v2';
