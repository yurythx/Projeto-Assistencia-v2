-- +goose Up
-- data_subject_requests — solicitações de exercício dos direitos do
-- titular previstos na LGPD (Lei 13.709/2018, art. 18): acesso e
-- portabilidade (kind = 'export') e eliminação/anonimização
-- (kind = 'erasure'). Fase 3 do roadmap de conformidade (ver
-- docs/adr/005 e docs/ROADMAP_CONFORMIDADE.md).
--
-- O processamento de cada solicitação (montar o pacote de dados do
-- titular, ou anonimizar suas linhas mantendo o que a lei obriga reter)
-- roda no worker; esta tabela é a fila + a trilha do SLA legal de
-- resposta. A eliminação NUNCA é um DELETE que quebre integridade
-- referencial ou a trilha imutável de audit_logs — é anonimização das
-- colunas de PII + marca do evento.
--
-- Só o próprio titular abre uma solicitação para si (o handler confere
-- identity.Subject == user_id); um nexus-admin pode consultar o
-- andamento para cumprir o prazo do art. 19.
CREATE TABLE data_subject_requests (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    kind          TEXT NOT NULL
                      CONSTRAINT data_subject_requests_kind_check
                      CHECK (kind IN ('export', 'erasure')),
    status        TEXT NOT NULL DEFAULT 'pending'
                      CONSTRAINT data_subject_requests_status_check
                      CHECK (status IN ('pending', 'processing', 'completed', 'failed', 'rejected')),
    -- Onde o pacote de exportação ficou disponível (chave no MinIO), para
    -- 'export' concluído. NULL para 'erasure'.
    result_object_key TEXT,
    -- Motivo de rejeição/falha, seguro de mostrar ao titular.
    detail        TEXT NOT NULL DEFAULT '',
    requested_ip  INET,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at  TIMESTAMPTZ
);

CREATE INDEX idx_data_subject_requests_user_id ON data_subject_requests (user_id);
CREATE INDEX idx_data_subject_requests_status ON data_subject_requests (status)
    WHERE status IN ('pending', 'processing');
CREATE INDEX idx_data_subject_requests_created_at ON data_subject_requests (created_at DESC);

-- Mantém updated_at coerente (mesma função compartilhada da 000001).
CREATE TRIGGER trg_data_subject_requests_set_updated_at
    BEFORE UPDATE ON data_subject_requests
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Toda abertura de solicitação entra na trilha imutável — a existência
-- do pedido é, ela mesma, um fato auditável (art. 18 §1º).
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION log_data_subject_request_audit()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO audit_logs (user_id, action, resource_type, resource_id, metadata, ip_address)
    VALUES (
        NEW.user_id,
        'lgpd.data_subject_request.' || NEW.kind,
        'data_subject_request',
        NEW.id::text,
        jsonb_build_object('kind', NEW.kind, 'status', NEW.status),
        NEW.requested_ip
    );
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trg_data_subject_requests_audit
    AFTER INSERT ON data_subject_requests
    FOR EACH ROW
    EXECUTE FUNCTION log_data_subject_request_audit();

-- +goose Down
DROP TRIGGER IF EXISTS trg_data_subject_requests_audit ON data_subject_requests;
DROP FUNCTION IF EXISTS log_data_subject_request_audit();
DROP TRIGGER IF EXISTS trg_data_subject_requests_set_updated_at ON data_subject_requests;
DROP TABLE IF EXISTS data_subject_requests;
