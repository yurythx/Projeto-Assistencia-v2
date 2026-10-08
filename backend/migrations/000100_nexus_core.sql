-- +goose Up
-- Projeto Nexus — núcleo do Microkernel.
--
-- Aditiva sobre o baseline (000001–000007): nada aqui apaga dado existente.
--   * system_modules  — estado de ativação de cada plugin registrado no
--                       Kernel (RegisterModule). Alterado em runtime, sem
--                       recompilar/reiniciar; o NOTIFY acorda todas as
--                       réplicas de API e worker.
--   * processed_events — deduplicação de consumidores (A08): um evento
--                       reentregue pelo RabbitMQ nunca é processado duas
--                       vezes pelo mesmo consumidor.
--   * branding_settings — white-label (DSGov / design tokens) por instalação.
--   * users.groups     — grupos do AD recebidos no último login federado.

-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS pg_trgm;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS btree_gist;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS unaccent;
-- +goose StatementEnd

-- unaccent() não é IMMUTABLE (depende do dicionário), então não pode ser
-- usada direto em coluna gerada/índice. Este wrapper fixa o dicionário e
-- é o que toda coluna tsvector de busca usa.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION nexus_unaccent(txt TEXT)
RETURNS TEXT
LANGUAGE sql IMMUTABLE PARALLEL SAFE STRICT AS $$
    SELECT public.unaccent('public.unaccent'::regdictionary, txt)
$$;
-- +goose StatementEnd

ALTER TABLE users ADD COLUMN IF NOT EXISTS groups TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE users ADD COLUMN IF NOT EXISTS ad_synced_at TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS idx_users_groups ON users USING gin (groups);
CREATE INDEX IF NOT EXISTS idx_users_search ON users
    USING gin ((nexus_unaccent(lower(display_name || ' ' || username || ' ' || email))) gin_trgm_ops);

CREATE TABLE system_modules (
    key         TEXT PRIMARY KEY,
    enabled     BOOLEAN NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by  TEXT NOT NULL DEFAULT 'system'
);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION notify_system_modules_changed()
RETURNS TRIGGER AS $$
BEGIN
    PERFORM pg_notify('nexus_modules_channel', COALESCE(NEW.key, OLD.key));
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER trg_system_modules_notify
    AFTER INSERT OR UPDATE OR DELETE ON system_modules
    FOR EACH ROW
    EXECUTE FUNCTION notify_system_modules_changed();
-- +goose StatementEnd

CREATE TABLE processed_events (
    consumer     TEXT NOT NULL,
    event_id     UUID NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (consumer, event_id)
);
CREATE INDEX idx_processed_events_processed_at ON processed_events (processed_at);

CREATE TABLE branding_settings (
    id              TEXT PRIMARY KEY DEFAULT 'default',
    app_name        TEXT NOT NULL DEFAULT 'Projeto Nexus',
    app_description TEXT NOT NULL DEFAULT 'Plataforma corporativa modular',
    org_name        TEXT NOT NULL DEFAULT 'Organização',
    logo_url        TEXT NOT NULL DEFAULT '',
    favicon_url     TEXT NOT NULL DEFAULT '',
    support_email   TEXT NOT NULL DEFAULT '',
    support_phone   TEXT NOT NULL DEFAULT '',
    support_hours   TEXT NOT NULL DEFAULT '',
    -- design tokens white-label (cores em hex); o frontend os aplica como
    -- CSS custom properties sobre a paleta DSGov padrão.
    tokens          JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by      TEXT NOT NULL DEFAULT '',
    CONSTRAINT branding_settings_singleton CHECK (id = 'default')
);
INSERT INTO branding_settings (id) VALUES ('default') ON CONFLICT DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS branding_settings;
DROP TABLE IF EXISTS processed_events;
DROP TRIGGER IF EXISTS trg_system_modules_notify ON system_modules;
DROP FUNCTION IF EXISTS notify_system_modules_changed();
DROP TABLE IF EXISTS system_modules;
DROP INDEX IF EXISTS idx_users_search;
DROP INDEX IF EXISTS idx_users_groups;
ALTER TABLE users DROP COLUMN IF EXISTS ad_synced_at;
ALTER TABLE users DROP COLUMN IF EXISTS groups;
DROP FUNCTION IF EXISTS nexus_unaccent(TEXT);
