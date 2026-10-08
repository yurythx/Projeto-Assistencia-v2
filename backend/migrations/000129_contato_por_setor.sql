-- +goose Up
-- Contato encaminhado por setor (ADR 013, fase 3): a triagem encaminha a
-- mensagem a uma unidade; contact:read / contact:manage com escopo veem e
-- tratam só o que foi encaminhado à área que cobrem. Sem unidade (NULL) é
-- a caixa geral — só a gestão global. É onde ficam as mensagens que já
-- existem e as que chegam pelo formulário público.
--
-- ON DELETE SET NULL: excluir a unidade devolve a mensagem à caixa geral.
ALTER TABLE contact_messages ADD COLUMN unidade_id UUID REFERENCES unidades (id) ON DELETE SET NULL;
CREATE INDEX idx_contact_messages_unidade ON contact_messages (unidade_id, created_at DESC) WHERE unidade_id IS NOT NULL;

-- +goose Down
ALTER TABLE contact_messages DROP COLUMN IF EXISTS unidade_id;
