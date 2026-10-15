-- +goose Up
-- SEMPRAS — Centro POP: encaminhamento pra fora do sistema (CAPS AD,
-- albergue, saúde, polícia). Diferente de sempras_encaminhamentos (entre
-- unidades nossas, com aceite/recusa da unidade de destino): aqui não
-- há ninguém do nosso lado para decidir — é um registro de que a pessoa
-- foi encaminhada, com status gerenciado por quem registrou (mesma
-- lógica de log imutável dos Atendimentos Coletivos, não um fluxo de
-- 2 pontas).

CREATE TABLE sempras_encaminhamentos_externos (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cidadao_id      UUID NOT NULL REFERENCES sempras_cidadaos (id),
    unidade_id      UUID NOT NULL REFERENCES unidades (id),
    destino         TEXT NOT NULL CHECK (destino IN
                        ('caps_ad', 'caps_iii', 'albergue_noturno', 'abrigo_municipal', 'saude_upa_hospital',
                         'sine_qualificacao', 'conselho_tutelar', 'policia_civil_deam', 'outro')),
    prioridade      TEXT NOT NULL DEFAULT 'media' CHECK (prioridade IN ('baixa', 'media', 'alta', 'urgente')),
    motivo          TEXT NOT NULL CHECK (char_length(motivo) BETWEEN 3 AND 2000),
    status          TEXT NOT NULL DEFAULT 'pendente' CHECK (status IN ('pendente', 'em_atendimento', 'concluido', 'cancelado')),
    observacoes     TEXT NOT NULL DEFAULT '' CHECK (char_length(observacoes) <= 2000),
    tecnico_id      UUID REFERENCES users (id),
    registrado_por  UUID REFERENCES users (id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    atualizado_em   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sempras_enc_ext_cidadao ON sempras_encaminhamentos_externos (cidadao_id, created_at DESC);
CREATE INDEX idx_sempras_enc_ext_unidade ON sempras_encaminhamentos_externos (unidade_id, status);

-- +goose Down
DROP TABLE IF EXISTS sempras_encaminhamentos_externos;
