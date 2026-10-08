-- +goose Up
-- Validação dos procedimentos (ADR 028): um fluxo nasce RASCUNHO, passa a
-- EM_VALIDACAO com as entrevistas nos departamentos e só é publicado quando
-- HOMOLOGADO. A regra "só homologado fica ativo" vale no banco: rascunho
-- nunca aparece na consulta pública, na busca nem no assistente.

ALTER TABLE atlas_workflows ADD COLUMN situacao TEXT NOT NULL DEFAULT 'HOMOLOGADO'
    CONSTRAINT atlas_workflows_situacao_check CHECK (situacao IN ('RASCUNHO', 'EM_VALIDACAO', 'HOMOLOGADO'));
ALTER TABLE atlas_workflows ADD CONSTRAINT atlas_workflows_ativo_homologado_check
    CHECK (NOT ativo OR situacao = 'HOMOLOGADO');
CREATE INDEX idx_atlas_workflows_situacao ON atlas_workflows (situacao) WHERE situacao <> 'HOMOLOGADO';

-- Registro das entrevistas/reuniões de validação, pelo código processual
-- (vale para as versões seguintes). Imutável: correções viram novo registro.
CREATE TABLE atlas_validacoes (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    codigo_processual  TEXT NOT NULL,
    realizada_em       DATE NOT NULL,
    unidade            TEXT NOT NULL CHECK (char_length(unidade) BETWEEN 1 AND 200),
    participantes      TEXT NOT NULL DEFAULT '' CHECK (char_length(participantes) <= 1000),
    registro           TEXT NOT NULL CHECK (char_length(registro) BETWEEN 1 AND 10000),
    pendencias         TEXT NOT NULL DEFAULT '' CHECK (char_length(pendencias) <= 5000),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by         TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_atlas_validacoes_codigo ON atlas_validacoes (codigo_processual, realizada_em DESC);

-- +goose Down
DROP TABLE IF EXISTS atlas_validacoes;
DROP INDEX IF EXISTS idx_atlas_workflows_situacao;
ALTER TABLE atlas_workflows DROP CONSTRAINT IF EXISTS atlas_workflows_ativo_homologado_check;
ALTER TABLE atlas_workflows DROP COLUMN IF EXISTS situacao;
