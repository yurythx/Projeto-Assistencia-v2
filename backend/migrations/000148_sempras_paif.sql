-- +goose Up
-- SEMPRAS — PAIF (Serviço de Proteção e Atendimento Integral à Família,
-- CRAS): vínculo da família ao CRAS de referência. status é um campo
-- real — no legado de referência "ativo" era inferido pela ausência de
-- data de exclusão, o que falha silenciosamente com dado incompleto.
-- extrema_pobreza/bolsa_familia/bpc são os indicadores de público
-- prioritário do RMA/MDS, como flags reais (não texto livre casado por
-- substring, que o legado usa e que silenciosamente desconta do relatório
-- qualquer erro de digitação).

CREATE TABLE sempras_paif_vinculos (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cidadao_id          UUID NOT NULL REFERENCES sempras_cidadaos (id),
    unidade_id          UUID NOT NULL REFERENCES unidades (id),
    status              TEXT NOT NULL DEFAULT 'ativo' CHECK (status IN ('ativo', 'excluido')),
    extrema_pobreza     BOOLEAN NOT NULL DEFAULT false,
    bolsa_familia       BOOLEAN NOT NULL DEFAULT false,
    bpc                 BOOLEAN NOT NULL DEFAULT false,
    scfv_participantes  INT NOT NULL DEFAULT 0 CHECK (scfv_participantes BETWEEN 0 AND 50),
    motivo_exclusao     TEXT NOT NULL DEFAULT '' CHECK (char_length(motivo_exclusao) <= 1000),
    incluido_por        UUID REFERENCES users (id),
    excluido_por        UUID REFERENCES users (id),
    data_inclusao       TIMESTAMPTZ NOT NULL DEFAULT now(),
    data_exclusao       TIMESTAMPTZ
);
-- Uma família só tem UM vínculo ativo por CRAS por vez — pode ser
-- reincluída (novo registro) depois de excluída, mas não duas vezes ativa.
CREATE UNIQUE INDEX uq_sempras_paif_ativo ON sempras_paif_vinculos (cidadao_id, unidade_id) WHERE status = 'ativo';
CREATE INDEX idx_sempras_paif_cidadao ON sempras_paif_vinculos (cidadao_id, data_inclusao DESC);
CREATE INDEX idx_sempras_paif_unidade ON sempras_paif_vinculos (unidade_id, status);

-- +goose Down
DROP TABLE IF EXISTS sempras_paif_vinculos;
