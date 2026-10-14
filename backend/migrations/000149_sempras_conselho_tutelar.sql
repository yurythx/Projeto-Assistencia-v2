-- +goose Up
-- SEMPRAS — Conselho Tutelar: atendimento à criança/adolescente com
-- medida protetiva do Art. 101 do ECA. A criança é um munícipe (mesmo
-- cadastro do módulo); o responsável fica em campos descritivos (não
-- ganha ficha própria — ninguém pede atendimento/benefício em nome dele
-- aqui). medida_protetiva fica vazia até a decisão (aplicar_medida) —
-- no intake ainda não se sabe qual medida será aplicada.

CREATE TABLE sempras_conselho_atendimentos (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    protocolo              TEXT NOT NULL UNIQUE,
    crianca_id             UUID NOT NULL REFERENCES sempras_cidadaos (id),
    unidade_id             UUID NOT NULL REFERENCES unidades (id),
    responsavel_nome       TEXT NOT NULL DEFAULT '' CHECK (char_length(responsavel_nome) <= 200),
    responsavel_cpf        TEXT NOT NULL DEFAULT '' CHECK (responsavel_cpf = '' OR responsavel_cpf ~ '^[0-9]{11}$'),
    responsavel_telefone   TEXT NOT NULL DEFAULT '' CHECK (char_length(responsavel_telefone) <= 30),
    origem_demanda         TEXT NOT NULL CHECK (origem_demanda IN
                                ('disque_100', 'escola', 'unidade_saude', 'plantao_24h', 'demanda_espontanea', 'policia')),
    motivo_violacao        TEXT NOT NULL CHECK (motivo_violacao IN
                                ('negligencia_abandono', 'violencia_fisica', 'abuso_exploracao_sexual', 'evasao_escolar', 'trabalho_infantil', 'conflito_familiar')),
    -- Art. 101 do ECA.
    medida_protetiva       TEXT NOT NULL DEFAULT '' CHECK (medida_protetiva IN
                                ('', 'termo_responsabilidade', 'inclusao_programa_social', 'matricula_escolar_obrigatoria', 'tratamento_medico_psicologico', 'acolhimento_emergencial')),
    status                 TEXT NOT NULL DEFAULT 'em_andamento' CHECK (status IN
                                ('em_andamento', 'medida_aplicada', 'encaminhado_mp', 'arquivado')),
    observacoes            TEXT NOT NULL DEFAULT '' CHECK (char_length(observacoes) <= 4000),
    registrado_por         UUID REFERENCES users (id),
    conselheiro_id         UUID REFERENCES users (id),
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    atualizado_em          TIMESTAMPTZ
);
CREATE INDEX idx_sempras_conselho_crianca ON sempras_conselho_atendimentos (crianca_id, created_at DESC);
CREATE INDEX idx_sempras_conselho_unidade ON sempras_conselho_atendimentos (unidade_id, status);

-- +goose Down
DROP TABLE IF EXISTS sempras_conselho_atendimentos;
