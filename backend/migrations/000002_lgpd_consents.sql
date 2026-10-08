-- +goose Up
-- Tabela de Consentimento LGPD (Lei 13.709/2018 - Art. 7º, I)
-- Registra formalmente a aceitação dos Termos de Uso e Política de Privacidade.

CREATE TABLE IF NOT EXISTS user_consents (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    term_version  TEXT NOT NULL,
    ip_address    INET,
    user_agent    TEXT,
    accepted_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT user_consents_version_unique UNIQUE (user_id, term_version)
);

CREATE INDEX IF NOT EXISTS idx_user_consents_user_id ON user_consents (user_id);
CREATE INDEX IF NOT EXISTS idx_user_consents_accepted_at ON user_consents (accepted_at DESC);

-- Trigger para registrar a aceitação no audit_logs imutável
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION log_lgpd_consent_audit()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO audit_logs (user_id, action, resource_type, resource_id, metadata, ip_address)
    VALUES (
        NEW.user_id,
        'lgpd_consent_given',
        'user_consents',
        NEW.id::text,
        jsonb_build_object(
            'term_version', NEW.term_version,
            'user_agent', COALESCE(NEW.user_agent, 'unknown')
        ),
        NEW.ip_address
    );
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trg_user_consents_audit
    AFTER INSERT ON user_consents
    FOR EACH ROW
    EXECUTE FUNCTION log_lgpd_consent_audit();

-- +goose Down
DROP TRIGGER IF EXISTS trg_user_consents_audit ON user_consents;
DROP FUNCTION IF EXISTS log_lgpd_consent_audit();
DROP TABLE IF EXISTS user_consents;
