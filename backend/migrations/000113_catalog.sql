-- +goose Up
-- Plugin Catálogo — central pública de serviços institucionais. Leitura
-- pública (somente publicados); edição exige catalog:manage.

CREATE TABLE catalog_services (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug                    TEXT NOT NULL UNIQUE,
    title                   TEXT NOT NULL,
    summary                 TEXT NOT NULL DEFAULT '',
    description             TEXT NOT NULL DEFAULT '',
    category                TEXT NOT NULL DEFAULT 'Geral',
    audience                TEXT NOT NULL DEFAULT '',
    requirements            TEXT[] NOT NULL DEFAULT '{}',
    steps                   TEXT[] NOT NULL DEFAULT '{}',
    -- [{ "type": "online"|"presencial"|"telefone"|"email", "label": "...", "value": "..." }]
    channels                JSONB NOT NULL DEFAULT '[]'::jsonb,
    sla                     TEXT NOT NULL DEFAULT '',
    cost                    TEXT NOT NULL DEFAULT 'Gratuito',
    icon                    TEXT NOT NULL DEFAULT 'file-text',
    responsible_unidade_id  UUID REFERENCES unidades (id) ON DELETE SET NULL,
    status                  TEXT NOT NULL DEFAULT 'draft' CONSTRAINT catalog_services_status_check
                                CHECK (status IN ('draft', 'published', 'archived')),
    position                INTEGER NOT NULL DEFAULT 0,
    published_at            TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    search                  TSVECTOR GENERATED ALWAYS AS (
        setweight(to_tsvector('portuguese', nexus_unaccent(title)), 'A') ||
        setweight(to_tsvector('portuguese', nexus_unaccent(category || ' ' || summary)), 'B') ||
        setweight(to_tsvector('portuguese', nexus_unaccent(description)), 'C')
    ) STORED
);
CREATE INDEX idx_catalog_services_status ON catalog_services (status, category, position);
CREATE INDEX idx_catalog_services_search ON catalog_services USING gin (search);

-- +goose StatementBegin
CREATE TRIGGER trg_catalog_services_set_updated_at BEFORE UPDATE ON catalog_services
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS catalog_services;
