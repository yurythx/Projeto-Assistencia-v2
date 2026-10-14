-- +goose Up
-- SEMPRAS — Casa Abrigo: documentos (PDF) do acolhido ou do responsável
-- (RN-15 do legado de referência). O conteúdo nunca atravessa a API — o
-- navegador envia direto ao MinIO por URL pré-assinada (mesmo padrão de
-- `files`/`tramite`): esta tabela só guarda o metadado.

CREATE TABLE sempras_casa_abrigo_documentos (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    acolhimento_id UUID NOT NULL REFERENCES sempras_casa_abrigo_acolhimentos (id) ON DELETE CASCADE,
    tipo           TEXT NOT NULL CHECK (tipo IN ('acolhido', 'responsavel')),
    nome_original  TEXT NOT NULL CHECK (char_length(nome_original) BETWEEN 1 AND 255),
    object_key     TEXT NOT NULL UNIQUE,
    content_type   TEXT NOT NULL DEFAULT '',
    size_bytes     BIGINT NOT NULL DEFAULT 0,
    status         TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'ready')),
    registrado_por UUID REFERENCES users (id),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sempras_casa_abrigo_documentos_acolhimento ON sempras_casa_abrigo_documentos (acolhimento_id);

-- +goose Down
DROP TABLE IF EXISTS sempras_casa_abrigo_documentos;
