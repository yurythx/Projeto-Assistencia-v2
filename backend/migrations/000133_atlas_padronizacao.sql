-- +goose Up
-- Atlas alinhado ao padrão dos demais plugins:
--   * sem tenant_id — a plataforma é de uma organização só (a divisão é
--     entidade > unidade); o valor vinha da query string e deixava quem
--     chamava escolher o "tenant";
--   * created_by (autoria) e updated_at mantido pelo gatilho comum;
--   * busca full-text nativa (tsvector + GIN, nexus_unaccent) no lugar do
--     Typesense — mesma infraestrutura da Busca Global e do Catálogo;
--   * CHECKs que o domínio já garante, como defesa em profundidade.

ALTER TABLE atlas_workflows DROP CONSTRAINT IF EXISTS uq_atlas_workflow_versao;
DROP INDEX IF EXISTS idx_atlas_wf_tenant;
ALTER TABLE atlas_workflows DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE atlas_workflows ADD CONSTRAINT uq_atlas_workflow_versao UNIQUE (codigo_processual, versao);
ALTER TABLE atlas_workflows ADD COLUMN created_by UUID REFERENCES users (id) ON DELETE SET NULL;
ALTER TABLE atlas_workflows ADD CONSTRAINT atlas_workflows_versao_check CHECK (versao >= 1);
ALTER TABLE atlas_workflows ADD COLUMN search TSVECTOR GENERATED ALWAYS AS (
    setweight(to_tsvector('portuguese', nexus_unaccent(codigo_processual || ' ' || titulo)), 'A') ||
    setweight(to_tsvector('portuguese', nexus_unaccent(objetivo)), 'B') ||
    setweight(to_tsvector('portuguese', nexus_unaccent(publico_alvo || ' ' || codigo_ttdd)), 'C')
) STORED;
CREATE INDEX idx_atlas_workflows_search ON atlas_workflows USING gin (search);
CREATE INDEX idx_atlas_workflows_ativo ON atlas_workflows (ativo, codigo_processual);

ALTER TABLE atlas_etapas ADD CONSTRAINT atlas_etapas_ordem_check CHECK (ordem >= 1);
ALTER TABLE atlas_etapas ADD CONSTRAINT atlas_etapas_prazo_check CHECK (prazo_sla_em_dias BETWEEN 0 AND 3650);
ALTER TABLE atlas_etapas ADD COLUMN search TSVECTOR GENERATED ALWAYS AS (
    to_tsvector('portuguese', nexus_unaccent(nome_setor || ' ' || unidade_administrativa || ' ' || atribuicoes_setor))
) STORED;
CREATE INDEX idx_atlas_etapas_search ON atlas_etapas USING gin (search);

ALTER TABLE atlas_etapa_documentos ADD COLUMN search TSVECTOR GENERATED ALWAYS AS (
    to_tsvector('portuguese', nexus_unaccent(nome_documento))
) STORED;
CREATE INDEX idx_atlas_etapa_documentos_search ON atlas_etapa_documentos USING gin (search);

CREATE INDEX IF NOT EXISTS idx_atlas_transicoes_origem ON atlas_etapa_transicoes (origem_etapa_id);
CREATE INDEX IF NOT EXISTS idx_atlas_transicoes_destino ON atlas_etapa_transicoes (destino_etapa_id);

ALTER TABLE atlas_classificacao_ttdd ADD CONSTRAINT atlas_ttdd_fases_check
    CHECK (fase_corrente_anos >= 0 AND fase_interm_anos >= 0);

-- +goose StatementBegin
CREATE TRIGGER trg_atlas_workflows_set_updated_at BEFORE UPDATE ON atlas_workflows
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER IF EXISTS trg_atlas_workflows_set_updated_at ON atlas_workflows;
ALTER TABLE atlas_classificacao_ttdd DROP CONSTRAINT IF EXISTS atlas_ttdd_fases_check;
DROP INDEX IF EXISTS idx_atlas_transicoes_destino;
DROP INDEX IF EXISTS idx_atlas_transicoes_origem;
ALTER TABLE atlas_etapa_documentos DROP COLUMN IF EXISTS search;
ALTER TABLE atlas_etapas DROP COLUMN IF EXISTS search;
ALTER TABLE atlas_etapas DROP CONSTRAINT IF EXISTS atlas_etapas_prazo_check;
ALTER TABLE atlas_etapas DROP CONSTRAINT IF EXISTS atlas_etapas_ordem_check;
DROP INDEX IF EXISTS idx_atlas_workflows_ativo;
ALTER TABLE atlas_workflows DROP COLUMN IF EXISTS search;
ALTER TABLE atlas_workflows DROP CONSTRAINT IF EXISTS atlas_workflows_versao_check;
ALTER TABLE atlas_workflows DROP COLUMN IF EXISTS created_by;
ALTER TABLE atlas_workflows DROP CONSTRAINT IF EXISTS uq_atlas_workflow_versao;
ALTER TABLE atlas_workflows ADD COLUMN tenant_id VARCHAR(64) NOT NULL DEFAULT 'nexus';
ALTER TABLE atlas_workflows ADD CONSTRAINT uq_atlas_workflow_versao UNIQUE (tenant_id, codigo_processual, versao);
CREATE INDEX IF NOT EXISTS idx_atlas_wf_tenant ON atlas_workflows (tenant_id, ativo);
