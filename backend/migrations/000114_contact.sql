-- +goose Up
-- Plugin Contato — formulário público institucional. Guarda PII de
-- visitante externo (nome, e-mail, telefone): leitura só com contact:read,
-- e o evento contact.message.submitted NÃO carrega PII no payload.

CREATE SEQUENCE contact_protocol_seq;

CREATE TABLE contact_messages (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    protocol     TEXT NOT NULL UNIQUE,
    name         TEXT NOT NULL,
    email        TEXT NOT NULL,
    phone        TEXT NOT NULL DEFAULT '',
    subject      TEXT NOT NULL,
    category     TEXT NOT NULL DEFAULT 'duvida' CONSTRAINT contact_messages_category_check
                     CHECK (category IN ('duvida', 'sugestao', 'reclamacao', 'elogio', 'outro')),
    message      TEXT NOT NULL,
    consent_at   TIMESTAMPTZ NOT NULL,
    ip_address   INET,
    user_agent   TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'new' CONSTRAINT contact_messages_status_check
                     CHECK (status IN ('new', 'in_progress', 'answered', 'archived')),
    assigned_to  UUID,
    notes        TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_contact_messages_status_created ON contact_messages (status, created_at DESC);

-- +goose StatementBegin
CREATE TRIGGER trg_contact_messages_set_updated_at BEFORE UPDATE ON contact_messages
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS contact_messages;
DROP SEQUENCE IF EXISTS contact_protocol_seq;
