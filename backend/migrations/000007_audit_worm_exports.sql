-- +goose Up
-- audit_worm_exports — índice da cópia WORM (Write Once Read Many) diária
-- da trilha de auditoria (F2.6 do roadmap de conformidade).
--
-- Motivo: o comentário da migration 000001 reconhece que os gatilhos que
-- tornam audit_logs append-only podem ser removidos por um superusuário
-- do Postgres. A defesa complementar é uma cópia externa imutável: o
-- worker (audit.WORMExporter) serializa cada dia COMPLETO de audit_logs,
-- calcula o SHA-256 do conteúdo encadeado ao SHA-256 do dia anterior
-- (cadeia à prova de adulteração) e sobe o arquivo + o digest para o
-- object storage. Esta tabela guarda a cabeça da cadeia por dia.
--
-- O bucket do object storage DEVE ser configurado com object-lock /
-- retenção no nível de infraestrutura para a garantia WORM completa; a
-- cadeia de hash aqui já dá evidência de adulteração mesmo sem isso.
CREATE TABLE audit_worm_exports (
    day          DATE PRIMARY KEY,
    row_count    INTEGER NOT NULL,
    sha256       TEXT NOT NULL,
    prev_sha256  TEXT NOT NULL DEFAULT '',
    object_key   TEXT NOT NULL,
    exported_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_worm_exports_exported_at ON audit_worm_exports (exported_at DESC);

-- A própria cabeça da cadeia é append-only — reescrever uma linha aqui
-- quebraria a verificação da cadeia sem deixar rastro.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION prevent_audit_worm_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'audit_worm_exports is append-only: % não é permitido', TG_OP;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trg_audit_worm_exports_immutable
    BEFORE UPDATE OR DELETE ON audit_worm_exports
    FOR EACH ROW
    EXECUTE FUNCTION prevent_audit_worm_mutation();

-- +goose Down
DROP TRIGGER IF EXISTS trg_audit_worm_exports_immutable ON audit_worm_exports;
DROP FUNCTION IF EXISTS prevent_audit_worm_mutation();
DROP TABLE IF EXISTS audit_worm_exports;
