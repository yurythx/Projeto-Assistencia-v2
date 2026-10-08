-- +goose Up
-- Público-alvo (ADR 014): Blog, Wiki e Agenda podem ser lidos só por uma
-- ou mais entidades e/ou unidades. Sem linhas = todos, como antes.
--
-- Chave estrangeira RESTRICT: excluir uma entidade ou unidade que é público
-- de algum conteúdo é recusado — com CASCADE o conteúdo ficaria visível a
-- todos em silêncio.

-- A unidade e as de cima (quem está numa unidade pertence ao público de
-- qualquer unidade acima dela). O resolvedor do IAM guarda no cache.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION nexus_unidades_acima(u UUID)
RETURNS UUID[]
LANGUAGE sql STABLE PARALLEL SAFE AS $$
    SELECT CASE WHEN u IS NULL THEN '{}'::UUID[] ELSE (
        WITH RECURSIVE acima(id, parent_id, nivel) AS (
            SELECT id, parent_id, 0 FROM unidades WHERE id = u
            UNION ALL
            SELECT x.id, x.parent_id, a.nivel + 1 FROM unidades x JOIN acima a ON x.id = a.parent_id WHERE a.nivel < 64
        )
        SELECT COALESCE(array_agg(id), '{}'::UUID[]) FROM acima)
    END
$$;
-- +goose StatementEnd

CREATE TABLE blog_post_publico (
    post_id     UUID NOT NULL REFERENCES blog_posts (id) ON DELETE CASCADE,
    entidade_id UUID REFERENCES entidades (id) ON DELETE RESTRICT,
    unidade_id  UUID REFERENCES unidades (id) ON DELETE RESTRICT,
    CONSTRAINT blog_post_publico_um_alvo CHECK (num_nonnulls(entidade_id, unidade_id) = 1),
    CONSTRAINT blog_post_publico_unico UNIQUE NULLS NOT DISTINCT (post_id, entidade_id, unidade_id)
);

CREATE TABLE wiki_page_publico (
    page_id     UUID NOT NULL REFERENCES wiki_pages (id) ON DELETE CASCADE,
    entidade_id UUID REFERENCES entidades (id) ON DELETE RESTRICT,
    unidade_id  UUID REFERENCES unidades (id) ON DELETE RESTRICT,
    CONSTRAINT wiki_page_publico_um_alvo CHECK (num_nonnulls(entidade_id, unidade_id) = 1),
    CONSTRAINT wiki_page_publico_unico UNIQUE NULLS NOT DISTINCT (page_id, entidade_id, unidade_id)
);

CREATE TABLE calendar_event_publico (
    event_id    UUID NOT NULL REFERENCES calendar_events (id) ON DELETE CASCADE,
    entidade_id UUID REFERENCES entidades (id) ON DELETE RESTRICT,
    unidade_id  UUID REFERENCES unidades (id) ON DELETE RESTRICT,
    CONSTRAINT calendar_event_publico_um_alvo CHECK (num_nonnulls(entidade_id, unidade_id) = 1),
    CONSTRAINT calendar_event_publico_unico UNIQUE NULLS NOT DISTINCT (event_id, entidade_id, unidade_id)
);

-- +goose Down
DROP TABLE IF EXISTS calendar_event_publico;
DROP TABLE IF EXISTS wiki_page_publico;
DROP TABLE IF EXISTS blog_post_publico;
DROP FUNCTION IF EXISTS nexus_unidades_acima(UUID);
