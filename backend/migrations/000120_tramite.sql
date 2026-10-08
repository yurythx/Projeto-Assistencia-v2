-- +goose Up
-- Plugin Trâmite — processos administrativos numerados (NNNNNN/AAAA), com
-- controle de sigilo, documentos (redigidos no sistema ou anexados no
-- MinIO), tramitação entre unidades com histórico imutável e assinatura
-- delegada ao Signum.

CREATE TABLE tramite_tipos (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug       TEXT NOT NULL UNIQUE,
    nome       TEXT NOT NULL,
    descricao  TEXT NOT NULL DEFAULT '',
    ativo      BOOLEAN NOT NULL DEFAULT true
);
INSERT INTO tramite_tipos (slug, nome, descricao) VALUES
    ('requerimento', 'Requerimento', 'Pedido formal de um interessado.'),
    ('memorando', 'Memorando', 'Comunicação interna entre unidades.'),
    ('oficio', 'Ofício', 'Comunicação oficial com órgãos externos.'),
    ('contratacao', 'Contratação', 'Instrução de processo de contratação.'),
    ('outros', 'Outros', 'Demais processos administrativos.')
ON CONFLICT (slug) DO NOTHING;

-- Numeração sequencial por ano, sem lacunas (UPDATE ... RETURNING dentro
-- da transação de abertura).
CREATE TABLE tramite_numeracao (
    ano     INTEGER PRIMARY KEY,
    ultimo  INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE tramite_processos (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    numero             TEXT NOT NULL UNIQUE,
    tipo_id            UUID NOT NULL REFERENCES tramite_tipos (id),
    assunto            TEXT NOT NULL,
    interessado        TEXT NOT NULL DEFAULT '',
    descricao          TEXT NOT NULL DEFAULT '',
    sigilo             TEXT NOT NULL DEFAULT 'publico' CONSTRAINT tramite_processos_sigilo_check
                           CHECK (sigilo IN ('publico', 'restrito', 'sigiloso')),
    status             TEXT NOT NULL DEFAULT 'aberto' CONSTRAINT tramite_processos_status_check
                           CHECK (status IN ('aberto', 'em_tramitacao', 'concluido', 'arquivado')),
    unidade_origem_id  UUID NOT NULL REFERENCES unidades (id),
    unidade_atual_id   UUID NOT NULL REFERENCES unidades (id),
    created_by         UUID NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    concluido_at       TIMESTAMPTZ,
    search             TSVECTOR GENERATED ALWAYS AS (
        setweight(to_tsvector('simple', numero), 'A') ||
        setweight(to_tsvector('portuguese', nexus_unaccent(assunto)), 'A') ||
        setweight(to_tsvector('portuguese', nexus_unaccent(interessado || ' ' || descricao)), 'B')
    ) STORED
);
CREATE INDEX idx_tramite_processos_unidade_atual ON tramite_processos (unidade_atual_id, status);
CREATE INDEX idx_tramite_processos_created ON tramite_processos (created_at DESC);
CREATE INDEX idx_tramite_processos_search ON tramite_processos USING gin (search);

-- +goose StatementBegin
CREATE TRIGGER trg_tramite_processos_set_updated_at BEFORE UPDATE ON tramite_processos
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
-- +goose StatementEnd

-- Credenciais explícitas de acesso a processo restrito/sigiloso.
CREATE TABLE tramite_acessos (
    processo_id  UUID NOT NULL REFERENCES tramite_processos (id) ON DELETE CASCADE,
    user_id      UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    granted_by   UUID NOT NULL,
    granted_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (processo_id, user_id)
);

CREATE TABLE tramite_documentos (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    processo_id   UUID NOT NULL REFERENCES tramite_processos (id) ON DELETE CASCADE,
    tipo          TEXT NOT NULL DEFAULT 'despacho',
    titulo        TEXT NOT NULL,
    origem        TEXT NOT NULL CONSTRAINT tramite_documentos_origem_check CHECK (origem IN ('redigido', 'anexo')),
    conteudo      TEXT NOT NULL DEFAULT '',
    object_key    TEXT NOT NULL DEFAULT '',
    content_type  TEXT NOT NULL DEFAULT '',
    size_bytes    BIGINT NOT NULL DEFAULT 0,
    sha256        CHAR(64),
    status        TEXT NOT NULL DEFAULT 'rascunho' CONSTRAINT tramite_documentos_status_check
                      CHECK (status IN ('rascunho', 'aguardando_assinatura', 'assinado', 'cancelado')),
    envelope_id   UUID,
    created_by    UUID NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_tramite_documentos_processo ON tramite_documentos (processo_id, created_at);
CREATE UNIQUE INDEX uq_tramite_documentos_envelope ON tramite_documentos (envelope_id) WHERE envelope_id IS NOT NULL;

-- +goose StatementBegin
CREATE TRIGGER trg_tramite_documentos_set_updated_at BEFORE UPDATE ON tramite_documentos
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
-- +goose StatementEnd

-- Histórico de movimentação: append-only (UPDATE/DELETE bloqueados).
CREATE TABLE tramite_movimentos (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    processo_id      UUID NOT NULL REFERENCES tramite_processos (id) ON DELETE CASCADE,
    acao             TEXT NOT NULL CONSTRAINT tramite_movimentos_acao_check
                         CHECK (acao IN ('abertura', 'tramitacao', 'conclusao', 'arquivamento', 'reabertura',
                                         'documento', 'assinatura_solicitada', 'assinatura_concluida', 'acesso_concedido')),
    de_unidade_id    UUID REFERENCES unidades (id),
    para_unidade_id  UUID REFERENCES unidades (id),
    despacho         TEXT NOT NULL DEFAULT '',
    actor_id         UUID,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_tramite_movimentos_processo ON tramite_movimentos (processo_id, created_at);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION prevent_tramite_movimentos_mutation()
RETURNS TRIGGER AS $$
BEGIN
    -- A exclusão em cascata do processo (ON DELETE CASCADE) é a única
    -- remoção admitida; o Nexus nunca apaga processo pela API.
    IF TG_OP = 'DELETE' AND NOT EXISTS (SELECT 1 FROM tramite_processos WHERE id = OLD.processo_id) THEN
        RETURN OLD;
    END IF;
    RAISE EXCEPTION 'tramite_movimentos é append-only: % não é permitido', TG_OP;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER trg_tramite_movimentos_immutable
    BEFORE UPDATE OR DELETE ON tramite_movimentos
    FOR EACH ROW EXECUTE FUNCTION prevent_tramite_movimentos_mutation();
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER IF EXISTS trg_tramite_movimentos_immutable ON tramite_movimentos;
DROP FUNCTION IF EXISTS prevent_tramite_movimentos_mutation();
DROP TABLE IF EXISTS tramite_movimentos;
DROP TABLE IF EXISTS tramite_documentos;
DROP TABLE IF EXISTS tramite_acessos;
DROP TABLE IF EXISTS tramite_processos;
DROP TABLE IF EXISTS tramite_numeracao;
DROP TABLE IF EXISTS tramite_tipos;
