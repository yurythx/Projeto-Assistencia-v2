-- +goose Up
-- Plugin Blog — publicações internas e comunicados (rascunho/publicação).
-- Publicar emite blog.post.published no Outbox, na mesma transação.

CREATE TABLE blog_posts (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug              TEXT NOT NULL UNIQUE,
    title             TEXT NOT NULL,
    summary           TEXT NOT NULL DEFAULT '',
    body              TEXT NOT NULL DEFAULT '',
    cover_object_key  TEXT NOT NULL DEFAULT '',
    kind              TEXT NOT NULL DEFAULT 'noticia' CONSTRAINT blog_posts_kind_check
                          CHECK (kind IN ('noticia', 'comunicado')),
    status            TEXT NOT NULL DEFAULT 'draft' CONSTRAINT blog_posts_status_check
                          CHECK (status IN ('draft', 'published', 'archived')),
    pinned            BOOLEAN NOT NULL DEFAULT false,
    author_id         UUID,
    published_at      TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    search            TSVECTOR GENERATED ALWAYS AS (
        setweight(to_tsvector('portuguese', nexus_unaccent(title)), 'A') ||
        setweight(to_tsvector('portuguese', nexus_unaccent(summary)), 'B') ||
        setweight(to_tsvector('portuguese', nexus_unaccent(body)), 'C')
    ) STORED,
    CONSTRAINT blog_posts_published_has_date CHECK (status <> 'published' OR published_at IS NOT NULL)
);
CREATE INDEX idx_blog_posts_status_published ON blog_posts (status, published_at DESC);
CREATE INDEX idx_blog_posts_search ON blog_posts USING gin (search);

-- +goose StatementBegin
CREATE TRIGGER trg_blog_posts_set_updated_at BEFORE UPDATE ON blog_posts
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS blog_posts;
