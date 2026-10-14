-- +goose Up
-- SEMPRAS — encaminhamento entre unidades (ex.: CRAS → CREAS, proteção
-- básica para média/alta complexidade). Quem recebe decide: aceita (passa
-- a acompanhar o caso) ou recusa (com observação).

CREATE TABLE sempras_encaminhamentos (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cidadao_id          UUID NOT NULL REFERENCES sempras_cidadaos (id),
    unidade_origem_id   UUID NOT NULL REFERENCES unidades (id),
    unidade_destino_id  UUID NOT NULL REFERENCES unidades (id),
    motivo              TEXT NOT NULL CHECK (char_length(motivo) BETWEEN 3 AND 2000),
    status              TEXT NOT NULL DEFAULT 'pendente' CHECK (status IN ('pendente', 'aceito', 'recusado')),
    observacao_decisao  TEXT NOT NULL DEFAULT '' CHECK (char_length(observacao_decisao) <= 1000),
    encaminhado_por     UUID REFERENCES users (id),
    decidido_por        UUID REFERENCES users (id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    decidido_em         TIMESTAMPTZ,
    CONSTRAINT sempras_encaminhamentos_destino_diferente CHECK (unidade_origem_id <> unidade_destino_id)
);
CREATE INDEX idx_sempras_encaminhamentos_cidadao ON sempras_encaminhamentos (cidadao_id, created_at DESC);
CREATE INDEX idx_sempras_encaminhamentos_destino ON sempras_encaminhamentos (unidade_destino_id, status, created_at);

-- +goose Down
DROP TABLE IF EXISTS sempras_encaminhamentos;
