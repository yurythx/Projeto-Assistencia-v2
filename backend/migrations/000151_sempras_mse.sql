-- +goose Up
-- SEMPRAS — Medidas Socioeducativas em meio aberto (CREAS, ECA art. 112,
-- III e VI: Liberdade Assistida e Prestação de Serviços à Comunidade).
-- Cumulação das duas medidas vira dois registros, cada um com seu
-- próprio acompanhamento de cumprimento (o legado de referência cumula
-- num único campo de texto e não dá pra acompanhar cada medida à parte).

CREATE TABLE sempras_mse_medidas (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    adolescente_id         UUID NOT NULL REFERENCES sempras_cidadaos (id),
    unidade_id             UUID NOT NULL REFERENCES unidades (id),
    numero_processo        TEXT NOT NULL DEFAULT '' CHECK (char_length(numero_processo) <= 100),
    vara_origem            TEXT NOT NULL DEFAULT '' CHECK (char_length(vara_origem) <= 200),
    ato_infracional        TEXT NOT NULL CHECK (char_length(ato_infracional) BETWEEN 3 AND 500),
    tipo_medida            TEXT NOT NULL CHECK (tipo_medida IN ('liberdade_assistida', 'prestacao_servico_comunidade')),
    instituicao_parceira   TEXT NOT NULL DEFAULT '' CHECK (char_length(instituicao_parceira) <= 200),
    status                 TEXT NOT NULL DEFAULT 'em_cumprimento' CHECK (status IN
                                ('em_cumprimento', 'cumprida', 'descumprida', 'extinta')),
    tecnico_referencia_id  UUID REFERENCES users (id),
    observacoes            TEXT NOT NULL DEFAULT '' CHECK (char_length(observacoes) <= 4000),
    registrado_por         UUID REFERENCES users (id),
    data_inicio            TIMESTAMPTZ NOT NULL DEFAULT now(),
    prazo_previsto         TIMESTAMPTZ,
    data_conclusao         TIMESTAMPTZ
);
CREATE INDEX idx_sempras_mse_adolescente ON sempras_mse_medidas (adolescente_id, data_inicio DESC);
CREATE INDEX idx_sempras_mse_unidade ON sempras_mse_medidas (unidade_id, status);

-- +goose Down
DROP TABLE IF EXISTS sempras_mse_medidas;
