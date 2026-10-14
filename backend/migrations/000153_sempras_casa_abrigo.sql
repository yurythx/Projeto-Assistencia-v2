-- +goose Up
-- SEMPRAS — Casa Abrigo: acolhimento institucional de criança/adolescente
-- sob medida protetiva. status é real (não inferido pela presença de
-- motivo_desligamento/data_desligamento, como faz o legado de
-- referência) e só muda pela ação dedicada (desligar), nunca pela
-- edição geral — é essa separação que impede contornar a regra de que
-- só a equipe técnica desliga (RN-14 do legado, lá contornável pelo PUT
-- geral).

CREATE TABLE sempras_casa_abrigo_acolhimentos (
    id                                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cidadao_id                        UUID NOT NULL REFERENCES sempras_cidadaos (id),
    unidade_id                        UUID NOT NULL REFERENCES unidades (id),
    sexo                              TEXT NOT NULL DEFAULT 'masculino' CHECK (sexo IN ('masculino', 'feminino')),
    responsavel_nome                  TEXT NOT NULL DEFAULT '' CHECK (char_length(responsavel_nome) <= 200),
    responsavel_nis                   TEXT NOT NULL DEFAULT '' CHECK (responsavel_nis = '' OR responsavel_nis ~ '^[0-9]{11}$'),
    responsavel_telefone              TEXT NOT NULL DEFAULT '' CHECK (char_length(responsavel_telefone) <= 30),
    responsavel_vinculo               TEXT NOT NULL DEFAULT '' CHECK (char_length(responsavel_vinculo) <= 100),
    responsavel_endereco              TEXT NOT NULL DEFAULT '' CHECK (char_length(responsavel_endereco) <= 300),
    motivos                           TEXT[] NOT NULL CHECK (motivos <@ ARRAY[
                                          'negligencia_abandono', 'violencia_fisica', 'violencia_psicologica', 'abuso_sexual',
                                          'orfandade_ausencia_responsavel', 'usuario_drogas_responsavel', 'situacao_rua',
                                          'trabalho_infantil', 'outro']::TEXT[]
                                          AND array_length(motivos, 1) > 0),
    processo_judicial                 TEXT NOT NULL DEFAULT '' CHECK (char_length(processo_judicial) <= 200),
    guia_acolhimento                  TEXT NOT NULL DEFAULT '' CHECK (char_length(guia_acolhimento) <= 2000),
    conselheiro_nome                  TEXT NOT NULL DEFAULT '' CHECK (char_length(conselheiro_nome) <= 200),
    conselheiro_telefone              TEXT NOT NULL DEFAULT '' CHECK (char_length(conselheiro_telefone) <= 30),
    situacao_juridica                 TEXT NOT NULL DEFAULT '' CHECK (char_length(situacao_juridica) <= 500),
    historico_acolhido_anteriormente  BOOLEAN NOT NULL DEFAULT false,
    historico_irmaos_outra_unidade    BOOLEAN NOT NULL DEFAULT false,
    observacoes_gerais                TEXT NOT NULL DEFAULT '' CHECK (char_length(observacoes_gerais) <= 4000),
    status                            TEXT NOT NULL DEFAULT 'acolhido' CHECK (status IN ('acolhido', 'desligado')),
    motivo_desligamento               TEXT NOT NULL DEFAULT '' CHECK (char_length(motivo_desligamento) <= 2000),
    registrado_por                    UUID REFERENCES users (id),
    desligado_por                     UUID REFERENCES users (id),
    data_entrada                      TIMESTAMPTZ NOT NULL DEFAULT now(),
    data_desligamento                 TIMESTAMPTZ
);
CREATE INDEX idx_sempras_casa_abrigo_cidadao ON sempras_casa_abrigo_acolhimentos (cidadao_id, data_entrada DESC);
CREATE INDEX idx_sempras_casa_abrigo_unidade ON sempras_casa_abrigo_acolhimentos (unidade_id, status);

-- Irmãos do acolhido (RN-07 do legado) — preserva o vínculo familiar
-- mesmo quando o irmão está em outra unidade ou não está acolhido.
CREATE TABLE sempras_casa_abrigo_irmaos (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    acolhimento_id    UUID NOT NULL REFERENCES sempras_casa_abrigo_acolhimentos (id) ON DELETE CASCADE,
    nome              TEXT NOT NULL CHECK (char_length(nome) BETWEEN 2 AND 200),
    data_nascimento   DATE,
    esta_acolhido     BOOLEAN NOT NULL DEFAULT false,
    unidade_descricao TEXT NOT NULL DEFAULT '' CHECK (char_length(unidade_descricao) <= 200),
    observacao        TEXT NOT NULL DEFAULT '' CHECK (char_length(observacao) <= 1000),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sempras_casa_abrigo_irmaos_acolhimento ON sempras_casa_abrigo_irmaos (acolhimento_id);

-- Diagnósticos de saúde/deficiência/transtorno do acolhido (RN-12 do
-- legado), normalizados numa lista — não em ~30 colunas booleanas fixas
-- na tabela principal, como faz o legado de referência.
CREATE TABLE sempras_casa_abrigo_diagnosticos (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    acolhimento_id UUID NOT NULL REFERENCES sempras_casa_abrigo_acolhimentos (id) ON DELETE CASCADE,
    categoria      TEXT NOT NULL CHECK (categoria IN ('doenca', 'deficiencia', 'transtorno', 'outro')),
    diagnostico    TEXT NOT NULL CHECK (char_length(diagnostico) BETWEEN 2 AND 255),
    descricao      TEXT NOT NULL DEFAULT '' CHECK (char_length(descricao) <= 2000),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sempras_casa_abrigo_diagnosticos_acolhimento ON sempras_casa_abrigo_diagnosticos (acolhimento_id);

-- +goose Down
DROP TABLE IF EXISTS sempras_casa_abrigo_diagnosticos;
DROP TABLE IF EXISTS sempras_casa_abrigo_irmaos;
DROP TABLE IF EXISTS sempras_casa_abrigo_acolhimentos;
