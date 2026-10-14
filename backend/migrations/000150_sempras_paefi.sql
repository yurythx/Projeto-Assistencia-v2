-- +goose Up
-- SEMPRAS — PAEFI (CREAS): acompanhamento de família/indivíduo em
-- situação de violação de direitos (Tipificação Nacional de Serviços
-- Socioassistenciais, Resolução CNAS nº 109/2009). violacoes é um
-- array validado por contenção (<@) — cada elemento tem que estar na
-- lista permitida, sem precisar de um tipo enum à parte para array.

CREATE TABLE sempras_paefi_casos (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    caso_codigo            TEXT NOT NULL UNIQUE,
    cidadao_id             UUID NOT NULL REFERENCES sempras_cidadaos (id),
    unidade_id             UUID NOT NULL REFERENCES unidades (id),
    violacoes              TEXT[] NOT NULL CHECK (violacoes <@ ARRAY[
                                'violencia_fisica', 'violencia_psicologica', 'violencia_sexual', 'negligencia',
                                'abandono', 'trabalho_infantil', 'discriminacao', 'medida_socioeducativa_meio_aberto']::TEXT[]
                                AND array_length(violacoes, 1) > 0),
    membros_vulneraveis    TEXT NOT NULL DEFAULT '' CHECK (char_length(membros_vulneraveis) <= 2000),
    pia_elaborado          BOOLEAN NOT NULL DEFAULT false,
    status                 TEXT NOT NULL DEFAULT 'em_acompanhamento' CHECK (status IN ('em_acompanhamento', 'desligado')),
    motivo_encerramento    TEXT NOT NULL DEFAULT '' CHECK (motivo_encerramento IN
                                ('', 'objetivos_alcancados', 'mudanca_territorio', 'nao_localizado', 'a_pedido_da_familia', 'outro')),
    tecnico_referencia_id  UUID REFERENCES users (id),
    observacoes            TEXT NOT NULL DEFAULT '' CHECK (char_length(observacoes) <= 4000),
    registrado_por         UUID REFERENCES users (id),
    data_abertura          TIMESTAMPTZ NOT NULL DEFAULT now(),
    data_encerramento      TIMESTAMPTZ
);
CREATE INDEX idx_sempras_paefi_cidadao ON sempras_paefi_casos (cidadao_id, data_abertura DESC);
CREATE INDEX idx_sempras_paefi_unidade ON sempras_paefi_casos (unidade_id, status);

-- +goose Down
DROP TABLE IF EXISTS sempras_paefi_casos;
