-- +goose Up
-- anonymous_consents — aceite dos Termos / Política de Privacidade por um
-- VISITANTE ainda não autenticado (gap G-11). Antes, numa página pública
-- o POST /lgpd/accept respondia 401 e só o localStorage do navegador
-- guardava o aceite — sem registro do lado do servidor.
--
-- Nenhuma PII: device_hash é um identificador opaco gerado no cliente
-- (UUID aleatório persistido em localStorage), NÃO derivado de dado
-- pessoal. Serve só para não duplicar o registro do mesmo navegador e
-- para o mesmo visitante, ao autenticar depois, ter seu consentimento
-- reconhecido.
CREATE TABLE anonymous_consents (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_hash   TEXT NOT NULL,
    term_version  TEXT NOT NULL,
    ip_address    INET,
    user_agent    TEXT,
    accepted_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT anonymous_consents_unique UNIQUE (device_hash, term_version)
);

CREATE INDEX idx_anonymous_consents_accepted_at ON anonymous_consents (accepted_at DESC);

-- Toda aceitação anônima também entra na trilha imutável (user_id NULL —
-- a coluna já é nullable). resource_id = device_hash para poder cruzar
-- depois com o user_consents do mesmo navegador, se o visitante logar.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION log_anonymous_consent_audit()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO audit_logs (user_id, action, resource_type, resource_id, metadata, ip_address)
    VALUES (
        NULL,
        'lgpd_consent_given_anon',
        'anonymous_consents',
        NEW.device_hash,
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

CREATE TRIGGER trg_anonymous_consents_audit
    AFTER INSERT ON anonymous_consents
    FOR EACH ROW
    EXECUTE FUNCTION log_anonymous_consent_audit();

-- +goose Down
DROP TRIGGER IF EXISTS trg_anonymous_consents_audit ON anonymous_consents;
DROP FUNCTION IF EXISTS log_anonymous_consent_audit();
DROP TABLE IF EXISTS anonymous_consents;
