-- +goose Up
-- Dono organizacional do conteúdo (ADR 013, fase 2): blog:manage,
-- wiki:manage e calendar:manage passam a valer só sobre o que está na
-- unidade coberta pela concessão. Conteúdo sem dono (NULL) é
-- institucional — só a gestão global o gerencia; é o que acontece com
-- tudo o que já existe (decisão do ADR 013).
--
-- ON DELETE SET NULL: excluir a unidade devolve o conteúdo à gestão
-- global, sem apagá-lo nem bloquear a exclusão da unidade.
ALTER TABLE blog_posts ADD COLUMN unidade_id UUID REFERENCES unidades (id) ON DELETE SET NULL;
ALTER TABLE wiki_pages ADD COLUMN unidade_id UUID REFERENCES unidades (id) ON DELETE SET NULL;
ALTER TABLE calendar_rooms ADD COLUMN unidade_id UUID REFERENCES unidades (id) ON DELETE SET NULL;
CREATE INDEX idx_blog_posts_unidade ON blog_posts (unidade_id) WHERE unidade_id IS NOT NULL;
CREATE INDEX idx_wiki_pages_unidade ON wiki_pages (unidade_id) WHERE unidade_id IS NOT NULL;
CREATE INDEX idx_calendar_rooms_unidade ON calendar_rooms (unidade_id) WHERE unidade_id IS NOT NULL;

-- +goose Down
ALTER TABLE calendar_rooms DROP COLUMN IF EXISTS unidade_id;
ALTER TABLE wiki_pages DROP COLUMN IF EXISTS unidade_id;
ALTER TABLE blog_posts DROP COLUMN IF EXISTS unidade_id;
