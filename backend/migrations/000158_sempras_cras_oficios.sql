-- +goose Up
-- SEMPRAS — Ofícios do CRAS: ofício recebido de um ente externo
-- (Conselho Tutelar, Ministério Público etc.) sobre uma família
-- acompanhada. Mesmo conceito das Notificações do CREAS
-- (sempras_creas_notificacoes), mas com os campos próprios do CRAS (nº
-- do ofício, data de emissão, nº da resposta) — por isso é uma tabela
-- própria, não uma extensão da notificação do CREAS.

CREATE TABLE sempras_cras_oficios (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cidadao_id              UUID NOT NULL REFERENCES sempras_cidadaos (id),
    unidade_id              UUID NOT NULL REFERENCES unidades (id),
    orgao_emissor           TEXT NOT NULL CHECK (char_length(orgao_emissor) BETWEEN 3 AND 200),
    numero_oficio           TEXT NOT NULL CHECK (char_length(numero_oficio) BETWEEN 1 AND 100),
    data_oficio             DATE,
    data_recebimento        DATE NOT NULL DEFAULT CURRENT_DATE,
    assunto                 TEXT NOT NULL DEFAULT '' CHECK (char_length(assunto) <= 2000),
    status                  TEXT NOT NULL DEFAULT 'pendente' CHECK (status IN ('pendente', 'respondido')),
    numero_resposta         TEXT NOT NULL DEFAULT '' CHECK (char_length(numero_resposta) <= 100),
    data_resposta           DATE,
    tecnico_responsavel_id  UUID REFERENCES users (id),
    registrado_por          UUID REFERENCES users (id),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sempras_cras_oficios_cidadao ON sempras_cras_oficios (cidadao_id, data_recebimento DESC);
CREATE INDEX idx_sempras_cras_oficios_unidade ON sempras_cras_oficios (unidade_id, status);

-- +goose Down
DROP TABLE IF EXISTS sempras_cras_oficios;
