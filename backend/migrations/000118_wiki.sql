-- +goose Up
-- Plugin Wiki — base de conhecimento em árvore Markdown colaborativa.
-- Qualquer autenticado cria e edita; excluir exige wiki:manage. Toda
-- edição gera uma revisão (histórico completo) e usa concorrência
-- otimista pela coluna version.

CREATE TABLE wiki_pages (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parent_id   UUID REFERENCES wiki_pages (id) ON DELETE RESTRICT,
    slug        TEXT NOT NULL UNIQUE,
    title       TEXT NOT NULL,
    body        TEXT NOT NULL DEFAULT '',
    position    INTEGER NOT NULL DEFAULT 0,
    version     INTEGER NOT NULL DEFAULT 1,
    created_by  UUID NOT NULL,
    updated_by  UUID NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    search      TSVECTOR GENERATED ALWAYS AS (
        setweight(to_tsvector('portuguese', nexus_unaccent(title)), 'A') ||
        setweight(to_tsvector('portuguese', nexus_unaccent(body)), 'C')
    ) STORED,
    CONSTRAINT wiki_pages_parent_not_self CHECK (parent_id IS NULL OR parent_id <> id)
);
CREATE INDEX idx_wiki_pages_parent ON wiki_pages (parent_id, position, title);
CREATE INDEX idx_wiki_pages_search ON wiki_pages USING gin (search);

CREATE TABLE wiki_revisions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    page_id     UUID NOT NULL REFERENCES wiki_pages (id) ON DELETE CASCADE,
    version     INTEGER NOT NULL,
    title       TEXT NOT NULL,
    body        TEXT NOT NULL,
    summary     TEXT NOT NULL DEFAULT '',
    edited_by   UUID NOT NULL,
    edited_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_wiki_revisions_page_version UNIQUE (page_id, version)
);

-- +goose Down
DROP TABLE IF EXISTS wiki_revisions;
DROP TABLE IF EXISTS wiki_pages;
