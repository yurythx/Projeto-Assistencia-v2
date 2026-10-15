-- +goose Up
-- SEMPRAS — Centro POP: perfil de rua, extensão 1:1 de sempras_cidadaos.
-- Fica fora da tabela principal porque não faz sentido pra uma família
-- do CRAS — só quem é atendido no Centro POP tem esse perfil. Saúde e
-- drogas são sigilosos como o registro psicológico (RN já estabelecida
-- no módulo): o Prontuario só inclui isso pra quem tem
-- sempras:atendimento, nunca pra recepção.

CREATE TABLE sempras_centro_pop_perfis (
    cidadao_id              UUID PRIMARY KEY REFERENCES sempras_cidadaos (id),
    situacao                TEXT NOT NULL DEFAULT 'situacao_rua' CHECK (situacao IN
                                ('situacao_rua', 'abrigado_institucionalmente', 'egresso_sistema_prisional', 'trecheiro', 'albergado', 'outro')),
    tempo_rua_anos          NUMERIC(5,2) CHECK (tempo_rua_anos IS NULL OR tempo_rua_anos >= 0),
    motivo_vulnerabilidade  TEXT NOT NULL DEFAULT 'outro' CHECK (motivo_vulnerabilidade IN
                                ('conflito_familiar', 'uso_de_drogas', 'alcoolismo', 'desemprego', 'perda_de_moradia',
                                 'egresso_sistema_prisional', 'migracao', 'escolha_propria', 'outro')),
    possui_doenca           BOOLEAN NOT NULL DEFAULT false,
    doenca_descricao        TEXT NOT NULL DEFAULT '' CHECK (char_length(doenca_descricao) <= 1000),
    usa_medicacao           BOOLEAN NOT NULL DEFAULT false,
    medicacao_descricao     TEXT NOT NULL DEFAULT '' CHECK (char_length(medicacao_descricao) <= 1000),
    uso_drogas              BOOLEAN NOT NULL DEFAULT false,
    drogas_descricao        TEXT NOT NULL DEFAULT '' CHECK (char_length(drogas_descricao) <= 1000),
    atualizado_por          UUID REFERENCES users (id),
    atualizado_em           TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS sempras_centro_pop_perfis;
