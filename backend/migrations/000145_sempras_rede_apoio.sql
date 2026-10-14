-- +goose Up
-- SEMPRAS — rede de apoio do munícipe: contatos de família, vizinhos ou
-- referências. Secretaria-wide como o cadastro (não é por unidade); sem
-- valor de prova como o prontuário, por isso pode ser removido (cadastro
-- errado, pessoa que pediu para sair da lista).

CREATE TABLE sempras_rede_apoio (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cidadao_id  UUID NOT NULL REFERENCES sempras_cidadaos (id),
    nome        TEXT NOT NULL CHECK (char_length(nome) BETWEEN 2 AND 200),
    parentesco  TEXT NOT NULL DEFAULT '' CHECK (char_length(parentesco) <= 100),
    telefone    TEXT NOT NULL DEFAULT '' CHECK (char_length(telefone) <= 30),
    created_by  UUID REFERENCES users (id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sempras_rede_apoio_cidadao ON sempras_rede_apoio (cidadao_id, created_at);

-- +goose Down
DROP TABLE IF EXISTS sempras_rede_apoio;
