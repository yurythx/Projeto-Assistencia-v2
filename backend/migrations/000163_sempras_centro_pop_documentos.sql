-- +goose Up
-- SEMPRAS — Centro POP: documentos do munícipe atendido (ex.: laudo de
-- saúde, documento de identificação quando consegue obter um depois).
-- Mesmo padrão dos documentos da Casa Abrigo (migração 000155): o
-- conteúdo nunca atravessa a API, só o metadado fica aqui.

CREATE TABLE sempras_centro_pop_documentos (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cidadao_id     UUID NOT NULL REFERENCES sempras_cidadaos (id),
    -- Cidadao não tem unidade própria (cadastro é da secretaria toda) —
    -- a unidade de quem subiu o documento fica aqui, pro escopo de área
    -- (mesma convenção de sempras_beneficios/sempras_encaminhamentos_externos).
    unidade_id     UUID NOT NULL REFERENCES unidades (id),
    tipo           TEXT NOT NULL CHECK (tipo IN ('identificacao', 'laudo_saude', 'outro')),
    nome_original  TEXT NOT NULL CHECK (char_length(nome_original) BETWEEN 1 AND 255),
    object_key     TEXT NOT NULL UNIQUE,
    content_type   TEXT NOT NULL DEFAULT '',
    size_bytes     BIGINT NOT NULL DEFAULT 0,
    status         TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'ready')),
    registrado_por UUID REFERENCES users (id),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sempras_centro_pop_documentos_cidadao ON sempras_centro_pop_documentos (cidadao_id);
CREATE INDEX idx_sempras_centro_pop_documentos_unidade ON sempras_centro_pop_documentos (unidade_id);

-- +goose Down
DROP TABLE IF EXISTS sempras_centro_pop_documentos;
