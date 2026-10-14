-- +goose Up
-- SEMPRAS — pedidos do titular (ADR 030 §8): acesso, correção e
-- anonimização. O munícipe não tem conta na plataforma (lgpd.PersonalData
-- é indexado por usuário) — por isso este é um fluxo próprio, registrado
-- por quem atende o titular presencialmente.

CREATE TYPE sempras_pedido_tipo AS ENUM ('acesso', 'correcao', 'anonimizacao');
CREATE TYPE sempras_pedido_status AS ENUM ('pendente', 'concluido', 'negado');

CREATE TABLE sempras_pedidos_titular (
    id                         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cidadao_id                 UUID NOT NULL REFERENCES sempras_cidadaos (id),
    tipo                       sempras_pedido_tipo NOT NULL,
    status                     sempras_pedido_status NOT NULL DEFAULT 'pendente',
    justificativa_solicitacao  TEXT NOT NULL CHECK (char_length(justificativa_solicitacao) BETWEEN 3 AND 1000),
    solicitado_por             UUID NOT NULL REFERENCES users (id),
    justificativa_decisao      TEXT CHECK (justificativa_decisao IS NULL OR char_length(justificativa_decisao) BETWEEN 3 AND 1000),
    decidido_por               UUID REFERENCES users (id),
    decidido_em                TIMESTAMPTZ,
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sempras_pedidos_cidadao ON sempras_pedidos_titular (cidadao_id, created_at DESC);
-- Fila de decisão da gestão: só pedidos pendentes (hoje, só anonimização
-- fica pendente — acesso e correção se resolvem na hora).
CREATE INDEX idx_sempras_pedidos_pendentes ON sempras_pedidos_titular (created_at) WHERE status = 'pendente';

-- Decisão é definitiva: nem a própria gestão desfaz uma anonimização já
-- aprovada, nem reabre uma negada. Mesma defesa de
-- trg_sempras_registros_immutable (000143), mas travando só depois de
-- decidido — a transação que resolve acesso/correção/decide a
-- anonimização ainda precisa fazer esse primeiro UPDATE.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION prevent_sempras_pedido_redecisao()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.status <> 'pendente' THEN
        RAISE EXCEPTION 'sempras_pedidos_titular: pedido % já foi decidido, não pode ser alterado', OLD.id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER trg_sempras_pedidos_immutable
    BEFORE UPDATE ON sempras_pedidos_titular
    FOR EACH ROW EXECUTE FUNCTION prevent_sempras_pedido_redecisao();

-- +goose Down
DROP TRIGGER IF EXISTS trg_sempras_pedidos_immutable ON sempras_pedidos_titular;
DROP FUNCTION IF EXISTS prevent_sempras_pedido_redecisao();
DROP TABLE IF EXISTS sempras_pedidos_titular;
DROP TYPE IF EXISTS sempras_pedido_status;
DROP TYPE IF EXISTS sempras_pedido_tipo;
