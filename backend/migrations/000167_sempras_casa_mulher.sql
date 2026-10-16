-- +goose Up
-- SEMPRAS — Casa da Mulher (ADR 031): o caso de acompanhamento da mulher em
-- situação de violência doméstica e familiar (Lei 11.340/2006 — Maria da
-- Penha). O atendimento do dia já é a fila e o prontuário; o caso guarda o
-- que é próprio do serviço: as formas de violência (art. 7º), o vínculo com
-- o agressor, o risco, a medida protetiva de urgência, a Patrulha Maria da
-- Penha e o abrigamento sigiloso. Só a Casa da Mulher registra (o serviço
-- "casa_mulher" do catálogo) e o caso não aparece no resumo das outras
-- unidades — revelaria a violência a quem não atende a mulher.

CREATE TABLE sempras_casa_mulher_casos (
    id                        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    caso_codigo               TEXT NOT NULL UNIQUE,
    cidadao_id                UUID NOT NULL REFERENCES sempras_cidadaos (id),
    unidade_id                UUID NOT NULL REFERENCES unidades (id),
    violencias                TEXT[] NOT NULL CHECK (violencias <@ ARRAY[
                                   'fisica', 'psicologica', 'sexual', 'patrimonial', 'moral']::TEXT[]
                                   AND array_length(violencias, 1) > 0),
    relacao_agressor          TEXT NOT NULL CHECK (relacao_agressor IN
                                   ('conjuge_companheiro', 'ex_conjuge_companheiro', 'namorado', 'ex_namorado', 'familiar', 'outro')),
    nivel_risco               TEXT NOT NULL CHECK (nivel_risco IN ('baixo', 'medio', 'alto', 'extremo')),
    medida_protetiva_status   TEXT NOT NULL DEFAULT 'nao_solicitada' CHECK (medida_protetiva_status IN
                                   ('nao_solicitada', 'em_analise', 'deferida', 'indeferida', 'descumprida')),
    medida_protetiva_processo TEXT NOT NULL DEFAULT '' CHECK (char_length(medida_protetiva_processo) <= 60),
    patrulha_maria_da_penha   BOOLEAN NOT NULL DEFAULT false,
    filhos                    INTEGER NOT NULL DEFAULT 0 CHECK (filhos BETWEEN 0 AND 30),
    abrigada                  BOOLEAN NOT NULL DEFAULT false,
    data_abrigamento          TIMESTAMPTZ,
    status                    TEXT NOT NULL DEFAULT 'ativo' CHECK (status IN ('ativo', 'encerrado')),
    motivo_encerramento       TEXT NOT NULL DEFAULT '' CHECK (motivo_encerramento IN
                                   ('', 'superacao_violencia', 'mudanca_municipio', 'transferencia_rede', 'desistencia', 'outro')),
    tecnico_referencia_id     UUID REFERENCES users (id),
    observacoes               TEXT NOT NULL DEFAULT '' CHECK (char_length(observacoes) <= 4000),
    registrado_por            UUID REFERENCES users (id),
    data_abertura             TIMESTAMPTZ NOT NULL DEFAULT now(),
    atualizado_em             TIMESTAMPTZ,
    data_encerramento         TIMESTAMPTZ,
    CONSTRAINT sempras_casa_mulher_abrigamento_check CHECK (abrigada = (data_abrigamento IS NOT NULL))
);
CREATE INDEX idx_sempras_casa_mulher_cidadao ON sempras_casa_mulher_casos (cidadao_id, data_abertura DESC);
CREATE INDEX idx_sempras_casa_mulher_unidade ON sempras_casa_mulher_casos (unidade_id, status);
-- Um caso ativo por mulher em cada unidade.
CREATE UNIQUE INDEX uq_sempras_casa_mulher_ativo ON sempras_casa_mulher_casos (cidadao_id, unidade_id) WHERE status = 'ativo';

-- +goose Down
DROP TABLE IF EXISTS sempras_casa_mulher_casos;
