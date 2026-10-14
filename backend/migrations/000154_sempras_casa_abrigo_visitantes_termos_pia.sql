-- +goose Up
-- SEMPRAS — Casa Abrigo: visitantes, termos (advertência/passeio/saída) e
-- PIA (Plano Individual de Atendimento). No legado de referência essas 3
-- partes tinham model e rota prontos mas NENHUMA tela jamais as usou —
-- aqui ganham tela real. hora_entrada/hora_saida são TIME de verdade
-- (o legado guarda como STRING(10) e compara como texto). O PIA do
-- legado é 5 tabelas (saúde, família, rotina, plano, intervenção) com
-- metas e objetivos no fundo iguais — aqui plano e intervenção
-- normalizam numa lista só (pia_metas), com "tipo" dizendo qual é qual.

CREATE TABLE sempras_casa_abrigo_visitantes (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    acolhimento_id       UUID NOT NULL REFERENCES sempras_casa_abrigo_acolhimentos (id) ON DELETE CASCADE,
    nome_visitante       TEXT NOT NULL CHECK (char_length(nome_visitante) BETWEEN 2 AND 200),
    documento_identidade TEXT NOT NULL DEFAULT '' CHECK (char_length(documento_identidade) <= 30),
    parentesco_vinculo   TEXT NOT NULL DEFAULT '' CHECK (char_length(parentesco_vinculo) <= 100),
    data_visita          DATE NOT NULL,
    hora_entrada         TIME,
    hora_saida           TIME,
    observacoes          TEXT NOT NULL DEFAULT '' CHECK (char_length(observacoes) <= 1000),
    registrado_por       UUID REFERENCES users (id),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sempras_casa_abrigo_visitantes_acolhimento ON sempras_casa_abrigo_visitantes (acolhimento_id, data_visita DESC);

CREATE TABLE sempras_casa_abrigo_termos (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    acolhimento_id         UUID NOT NULL REFERENCES sempras_casa_abrigo_acolhimentos (id) ON DELETE CASCADE,
    tipo_termo             TEXT NOT NULL CHECK (tipo_termo IN ('advertencia', 'passeio', 'saida', 'outro')),
    responsavel_retirada   TEXT NOT NULL DEFAULT '' CHECK (char_length(responsavel_retirada) <= 200),
    parentesco_responsavel TEXT NOT NULL DEFAULT '' CHECK (char_length(parentesco_responsavel) <= 100),
    motivo_observacao      TEXT NOT NULL DEFAULT '' CHECK (char_length(motivo_observacao) <= 2000),
    data_termo             DATE NOT NULL,
    registrado_por         UUID REFERENCES users (id),
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sempras_casa_abrigo_termos_acolhimento ON sempras_casa_abrigo_termos (acolhimento_id, data_termo DESC);

-- PIA: ficha 1:1 por acolhimento (saúde, família, rotina).
CREATE TABLE sempras_casa_abrigo_pia (
    acolhimento_id          UUID PRIMARY KEY REFERENCES sempras_casa_abrigo_acolhimentos (id) ON DELETE CASCADE,
    matricula_escolar       TEXT NOT NULL DEFAULT '' CHECK (char_length(matricula_escolar) <= 100),
    serie_escolar           TEXT NOT NULL DEFAULT '' CHECK (char_length(serie_escolar) <= 100),
    escola                  TEXT NOT NULL DEFAULT '' CHECK (char_length(escola) <= 200),
    medicamentos            TEXT NOT NULL DEFAULT '' CHECK (char_length(medicamentos) <= 2000),
    alergias                TEXT NOT NULL DEFAULT '' CHECK (char_length(alergias) <= 2000),
    vacinacao_atualizada    BOOLEAN NOT NULL DEFAULT false,
    cartao_sus              TEXT NOT NULL DEFAULT '' CHECK (char_length(cartao_sus) <= 30),
    contato_familiar        TEXT NOT NULL DEFAULT '' CHECK (char_length(contato_familiar) <= 2000),
    visitas_autorizadas     TEXT NOT NULL DEFAULT '' CHECK (char_length(visitas_autorizadas) <= 2000),
    historico_familiar      TEXT NOT NULL DEFAULT '' CHECK (char_length(historico_familiar) <= 4000),
    atividades_manha        TEXT NOT NULL DEFAULT '' CHECK (char_length(atividades_manha) <= 1000),
    atividades_tarde        TEXT NOT NULL DEFAULT '' CHECK (char_length(atividades_tarde) <= 1000),
    atividades_noite        TEXT NOT NULL DEFAULT '' CHECK (char_length(atividades_noite) <= 1000),
    atividades_fim_semana   TEXT NOT NULL DEFAULT '' CHECK (char_length(atividades_fim_semana) <= 1000),
    atualizado_por          UUID REFERENCES users (id),
    atualizado_em           TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- PIA: metas — plano (necessidade/providências) e intervenção
-- (objetivos/estratégias) normalizados numa lista só.
CREATE TABLE sempras_casa_abrigo_pia_metas (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    acolhimento_id UUID NOT NULL REFERENCES sempras_casa_abrigo_acolhimentos (id) ON DELETE CASCADE,
    tipo           TEXT NOT NULL CHECK (tipo IN ('plano', 'intervencao')),
    descricao      TEXT NOT NULL CHECK (char_length(descricao) BETWEEN 3 AND 1000),
    providencia    TEXT NOT NULL DEFAULT '' CHECK (char_length(providencia) <= 1000),
    responsavel    TEXT NOT NULL DEFAULT '' CHECK (char_length(responsavel) <= 200),
    prazo          DATE,
    concluida      BOOLEAN NOT NULL DEFAULT false,
    registrado_por UUID REFERENCES users (id),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sempras_casa_abrigo_pia_metas_acolhimento ON sempras_casa_abrigo_pia_metas (acolhimento_id, tipo);

-- +goose Down
DROP TABLE IF EXISTS sempras_casa_abrigo_pia_metas;
DROP TABLE IF EXISTS sempras_casa_abrigo_pia;
DROP TABLE IF EXISTS sempras_casa_abrigo_termos;
DROP TABLE IF EXISTS sempras_casa_abrigo_visitantes;
