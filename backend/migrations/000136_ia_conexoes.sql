-- +goose Up
-- Conexões de inteligência artificial editáveis pela tela (ADR 020):
-- fornecedor, endereço, modelo e chave de API (cifrada com
-- CONFIG_ENCRYPTION_KEY, nunca devolvida pela API), com o resultado do
-- último teste; e, por função que usa IA, a conexão principal, a reserva,
-- o mascaramento de dados pessoais e quem autorizou o envio externo.

CREATE TABLE ia_conexoes (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    nome               TEXT NOT NULL UNIQUE CHECK (char_length(nome) BETWEEN 1 AND 80),
    provedor           TEXT NOT NULL CHECK (provedor IN
                           ('ollama', 'openai', 'azure', 'gemini', 'anthropic', 'groq', 'openrouter', 'mistral', 'compativel')),
    endpoint           TEXT NOT NULL CHECK (endpoint ~ '^https?://'),
    modelo             TEXT NOT NULL CHECK (char_length(modelo) BETWEEN 1 AND 120),
    chave_cifrada      TEXT NOT NULL DEFAULT '',
    externo            BOOLEAN NOT NULL,
    timeout_segundos   INT NOT NULL DEFAULT 30 CHECK (timeout_segundos BETWEEN 1 AND 300),
    teste_ok           BOOLEAN,
    teste_em           TIMESTAMPTZ,
    teste_latencia_ms  INT,
    teste_erro         TEXT NOT NULL DEFAULT '',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by         TEXT NOT NULL DEFAULT '',
    CHECK ((teste_ok IS NULL) = (teste_em IS NULL) AND (teste_ok IS NULL) = (teste_latencia_ms IS NULL))
);

-- Sem linha = a função usa o ambiente (ATLAS_AI_*). Linha com principal
-- nula = IA desligada pela tela.
CREATE TABLE ia_uso (
    funcao                   TEXT PRIMARY KEY CHECK (funcao IN ('atlas.assistente')),
    principal_id             UUID REFERENCES ia_conexoes (id) ON DELETE RESTRICT,
    reserva_id               UUID REFERENCES ia_conexoes (id) ON DELETE RESTRICT,
    mascarar_dados_pessoais  BOOLEAN NOT NULL DEFAULT TRUE,
    externo_autorizado_por   TEXT NOT NULL DEFAULT '',
    externo_autorizado_em    TIMESTAMPTZ,
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by               TEXT NOT NULL DEFAULT '',
    CHECK (reserva_id IS NULL OR (principal_id IS NOT NULL AND reserva_id <> principal_id))
);

-- +goose Down
DROP TABLE IF EXISTS ia_uso;
DROP TABLE IF EXISTS ia_conexoes;
