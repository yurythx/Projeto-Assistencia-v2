-- +goose Up
-- Notificações persistentes da plataforma (ADR 027): a caixa de cada
-- usuário (o sino), com leitura, preferência por módulo (opt-out) e chave de
-- idempotência — o mesmo aviso nunca chega duas vezes à mesma pessoa.

CREATE TABLE notificacoes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    modulo      TEXT NOT NULL CHECK (char_length(modulo) BETWEEN 1 AND 40),
    titulo      TEXT NOT NULL CHECK (char_length(titulo) BETWEEN 1 AND 200),
    mensagem    TEXT NOT NULL DEFAULT '' CHECK (char_length(mensagem) <= 1000),
    -- Só caminho interno ("/atlas/…"): nada de redirecionamento externo.
    link        TEXT NOT NULL DEFAULT '' CHECK (link = '' OR link ~ '^/[^/\\]'),
    chave       TEXT NOT NULL CHECK (char_length(chave) BETWEEN 1 AND 200),
    lida_em     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_notificacoes_chave UNIQUE (user_id, chave)
);
CREATE INDEX idx_notificacoes_caixa ON notificacoes (user_id, created_at DESC);
CREATE INDEX idx_notificacoes_nao_lidas ON notificacoes (user_id) WHERE lida_em IS NULL;

-- Opt-out por módulo: sem linha, o usuário recebe.
CREATE TABLE notificacoes_preferencias (
    user_id     UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    modulo      TEXT NOT NULL,
    ativo       BOOLEAN NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, modulo)
);

-- Quem segue um procedimento do Atlas (pelo código: vale para as versões
-- seguintes).
CREATE TABLE atlas_seguidores (
    user_id            UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    codigo_processual  TEXT NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, codigo_processual)
);
CREATE INDEX idx_atlas_seguidores_codigo ON atlas_seguidores (codigo_processual);

-- +goose Down
DROP TABLE IF EXISTS atlas_seguidores;
DROP TABLE IF EXISTS notificacoes_preferencias;
DROP TABLE IF EXISTS notificacoes;
