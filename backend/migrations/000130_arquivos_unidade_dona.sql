-- +goose Up
-- Arquivos com dono organizacional (ADR 013, fase 3): a pasta pode ter uma
-- unidade dona, herdada pelas subpastas (a cadeia até a raiz). files:manage
-- com escopo tem acesso total às pastas cuja dona ele cobre. Pastas sem
-- dona (todas as que já existem) seguem só com o dono pessoal e as ACLs —
-- e a gestão global.
--
-- ON DELETE SET NULL: excluir a unidade não apaga nem trava as pastas.
ALTER TABLE files_folders ADD COLUMN unidade_id UUID REFERENCES unidades (id) ON DELETE SET NULL;
CREATE INDEX idx_files_folders_unidade ON files_folders (unidade_id) WHERE unidade_id IS NOT NULL;

-- +goose Down
ALTER TABLE files_folders DROP COLUMN IF EXISTS unidade_id;
