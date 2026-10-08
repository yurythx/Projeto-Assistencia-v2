-- +goose Up
-- Biblioteca de modelos de documento do Atlas (ADR 024): cada modelo tem
-- versões (o arquivo de cada uma fica no MinIO) e as peças dos fluxos
-- apontam para um modelo do catálogo — atualizar o modelo vale para todos
-- os fluxos que o usam. Modelo não é apagado: desativado, sai da escolha
-- de novas peças, mas continua disponível onde já está ligado.

CREATE TABLE atlas_modelos (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    nome        TEXT NOT NULL CHECK (char_length(nome) BETWEEN 1 AND 150),
    descricao   TEXT NOT NULL DEFAULT '' CHECK (char_length(descricao) <= 2000),
    ativo       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by  TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX idx_atlas_modelos_nome ON atlas_modelos (lower(nome));

CREATE TABLE atlas_modelo_versoes (
    modelo_id     UUID NOT NULL REFERENCES atlas_modelos (id) ON DELETE RESTRICT,
    versao        INT NOT NULL CHECK (versao >= 1),
    arquivo_nome  TEXT NOT NULL CHECK (char_length(arquivo_nome) BETWEEN 1 AND 255),
    content_type  TEXT NOT NULL,
    tamanho       BIGINT NOT NULL CHECK (tamanho > 0),
    sha256        TEXT NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    objeto        TEXT NOT NULL,
    nota          TEXT NOT NULL DEFAULT '' CHECK (char_length(nota) <= 500),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by    TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (modelo_id, versao)
);

ALTER TABLE atlas_etapa_documentos ADD COLUMN modelo_id UUID REFERENCES atlas_modelos (id) ON DELETE RESTRICT;
CREATE INDEX idx_atlas_etapa_documentos_modelo ON atlas_etapa_documentos (modelo_id) WHERE modelo_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_atlas_etapa_documentos_modelo;
ALTER TABLE atlas_etapa_documentos DROP COLUMN IF EXISTS modelo_id;
DROP TABLE IF EXISTS atlas_modelo_versoes;
DROP TABLE IF EXISTS atlas_modelos;
