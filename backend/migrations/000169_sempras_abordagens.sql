-- +goose Up
-- SEMPRAS — abordagem social do Centro POP (busca ativa da equipe na rua,
-- Tipificação Nacional, Serviço Especializado em Abordagem Social). Cada
-- linha é uma abordagem: onde, quando, a quem (o munícipe cadastrado ou só
-- uma descrição, quando a pessoa não se identifica) e o resultado. O legado
-- (SocialAbordagem) tinha o model, mas nenhuma tela de registro.

CREATE TABLE sempras_abordagens (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    unidade_id           UUID NOT NULL REFERENCES unidades (id),
    cidadao_id           UUID REFERENCES sempras_cidadaos (id),
    descricao_pessoa     TEXT NOT NULL DEFAULT '' CHECK (char_length(descricao_pessoa) <= 300),
    local                TEXT NOT NULL CHECK (char_length(local) BETWEEN 3 AND 300),
    data_abordagem       TIMESTAMPTZ NOT NULL,
    resultado            TEXT NOT NULL CHECK (resultado IN ('aceitou_acolhimento', 'aceitou_encaminhamento', 'orientado', 'recusou')),
    uso_drogas_aparente  BOOLEAN NOT NULL DEFAULT false,
    demanda_saude        BOOLEAN NOT NULL DEFAULT false,
    observacoes          TEXT NOT NULL DEFAULT '' CHECK (char_length(observacoes) <= 2000),
    registrado_por       UUID REFERENCES users (id),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Identificada pelo cadastro ou, ao menos, descrita.
    CONSTRAINT sempras_abordagens_pessoa_check CHECK (cidadao_id IS NOT NULL OR descricao_pessoa <> '')
);
CREATE INDEX idx_sempras_abordagens_unidade ON sempras_abordagens (unidade_id, data_abordagem DESC);
CREATE INDEX idx_sempras_abordagens_cidadao ON sempras_abordagens (cidadao_id, data_abordagem DESC) WHERE cidadao_id IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS sempras_abordagens;
