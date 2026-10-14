-- +goose Up
-- SEMPRAS — Benefícios Eventuais (LOAS art. 22; Decreto 6.307/2007):
-- natalidade, funeral, cesta básica e passagem. Quem pede pode ser o
-- próprio beneficiário ou outra pessoa da família (solicitante_id) — por
-- isso o tipo é um enum real (no legado de referência era texto livre e
-- quebrava silenciosamente os relatórios).

CREATE TABLE sempras_beneficios (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    beneficiario_id     UUID NOT NULL REFERENCES sempras_cidadaos (id),
    solicitante_id      UUID REFERENCES sempras_cidadaos (id),
    unidade_id          UUID NOT NULL REFERENCES unidades (id),
    tipo                TEXT NOT NULL CHECK (tipo IN ('natalidade', 'funeral', 'cesta_basica', 'passagem')),
    quantidade          INT NOT NULL DEFAULT 1 CHECK (quantidade BETWEEN 1 AND 100),
    status              TEXT NOT NULL DEFAULT 'solicitado' CHECK (status IN ('solicitado', 'concedido', 'negado', 'suspenso')),
    observacao          TEXT NOT NULL DEFAULT '' CHECK (char_length(observacao) <= 2000),
    motivo_decisao      TEXT NOT NULL DEFAULT '' CHECK (char_length(motivo_decisao) <= 1000),
    tecnico_decisao_id  UUID REFERENCES users (id),
    solicitado_por      UUID REFERENCES users (id),
    data_solicitacao    TIMESTAMPTZ NOT NULL DEFAULT now(),
    data_decisao        TIMESTAMPTZ
);
CREATE INDEX idx_sempras_beneficios_beneficiario ON sempras_beneficios (beneficiario_id, data_solicitacao DESC);
CREATE INDEX idx_sempras_beneficios_unidade ON sempras_beneficios (unidade_id, data_solicitacao DESC);

-- +goose Down
DROP TABLE IF EXISTS sempras_beneficios;
