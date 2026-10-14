-- +goose Up
-- SEMPRAS — Notificações do CREAS: ofícios/requisições recebidos de um
-- ente externo (MP, Judiciário, Conselho Tutelar, Disque 100/180,
-- Delegacia) sobre uma vítima de violação de direitos. A vítima tem que
-- já estar cadastrada (vitima_id com FK real) — o legado de referência
-- só guarda nome/CPF em texto livre e não cruza com nenhum outro caso.

CREATE TABLE sempras_creas_notificacoes (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    protocolo              TEXT NOT NULL UNIQUE,
    vitima_id              UUID NOT NULL REFERENCES sempras_cidadaos (id),
    unidade_id             UUID NOT NULL REFERENCES unidades (id),
    origem_entidade        TEXT NOT NULL CHECK (origem_entidade IN
                                ('ministerio_publico', 'poder_judiciario', 'conselho_tutelar', 'disque_100_180', 'delegacia', 'outro')),
    assunto                TEXT NOT NULL CHECK (char_length(assunto) BETWEEN 3 AND 2000),
    status                 TEXT NOT NULL DEFAULT 'recebida' CHECK (status IN
                                ('recebida', 'em_diligencia', 'respondida', 'arquivada')),
    resposta               TEXT NOT NULL DEFAULT '' CHECK (char_length(resposta) <= 4000),
    tecnico_referencia_id  UUID REFERENCES users (id),
    registrado_por         UUID REFERENCES users (id),
    data_entrada           TIMESTAMPTZ NOT NULL DEFAULT now(),
    prazo_resposta         TIMESTAMPTZ,
    data_resposta          TIMESTAMPTZ
);
CREATE INDEX idx_sempras_creas_notif_vitima ON sempras_creas_notificacoes (vitima_id, data_entrada DESC);
CREATE INDEX idx_sempras_creas_notif_unidade ON sempras_creas_notificacoes (unidade_id, status);

-- +goose Down
DROP TABLE IF EXISTS sempras_creas_notificacoes;
