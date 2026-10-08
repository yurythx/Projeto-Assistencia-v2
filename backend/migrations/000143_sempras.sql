-- +goose Up
-- SEMPRAS — atendimento socioassistencial por unidade (ADR 030).
--
-- O munícipe é um cadastro único da secretaria (sem duplicidade entre
-- unidades); fila, atendimentos e prontuário são sempre de uma unidade
-- (unidades.id), que é a posição usada na autorização com escopo (ADR 013).

CREATE TABLE sempras_cidadaos (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    nome             TEXT NOT NULL CHECK (char_length(nome) BETWEEN 2 AND 200),
    nome_social      TEXT NOT NULL DEFAULT '' CHECK (char_length(nome_social) <= 200),
    cpf              TEXT CHECK (cpf ~ '^[0-9]{11}$'),
    nis              TEXT CHECK (nis ~ '^[0-9]{11}$'),
    data_nascimento  DATE,
    nome_mae         TEXT NOT NULL DEFAULT '' CHECK (char_length(nome_mae) <= 200),
    telefone         TEXT NOT NULL DEFAULT '' CHECK (char_length(telefone) <= 30),
    endereco         TEXT NOT NULL DEFAULT '' CHECK (char_length(endereco) <= 300),
    created_by       UUID REFERENCES users (id),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Sem CPF nem NIS (população de rua, crianças), a pessoa é identificada
    -- por nascimento + nome da mãe.
    CONSTRAINT sempras_cidadaos_identificacao_check
        CHECK (cpf IS NOT NULL OR nis IS NOT NULL OR (data_nascimento IS NOT NULL AND nome_mae <> ''))
);
CREATE UNIQUE INDEX uq_sempras_cidadaos_cpf ON sempras_cidadaos (cpf) WHERE cpf IS NOT NULL;
CREATE UNIQUE INDEX uq_sempras_cidadaos_nis ON sempras_cidadaos (nis) WHERE nis IS NOT NULL;
-- Busca por nome (sem acento, qualquer trecho): trigramas.
CREATE INDEX idx_sempras_cidadaos_nome ON sempras_cidadaos
    USING gin (lower(nexus_unaccent(nome || ' ' || nome_social)) gin_trgm_ops);
CREATE TRIGGER trg_sempras_cidadaos_set_updated_at BEFORE UPDATE ON sempras_cidadaos
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Senha sequencial por unidade e por dia (o dia é o do fuso do processo).
CREATE TABLE sempras_senhas (
    unidade_id  UUID NOT NULL REFERENCES unidades (id),
    dia         DATE NOT NULL,
    ultima      INT NOT NULL CHECK (ultima > 0),
    PRIMARY KEY (unidade_id, dia)
);

CREATE TABLE sempras_atendimentos (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cidadao_id       UUID NOT NULL REFERENCES sempras_cidadaos (id),
    unidade_id       UUID NOT NULL REFERENCES unidades (id),
    dia              DATE NOT NULL,
    senha            INT NOT NULL CHECK (senha > 0),
    tipo             TEXT NOT NULL CHECK (tipo IN ('social', 'psicologico')),
    -- Prioridade legal (Lei 10.048/2000; 80+ tem precedência — Lei 13.466/2017).
    prioridade       TEXT NOT NULL DEFAULT '' CHECK (prioridade IN
                         ('', 'idoso', 'idoso_80', 'deficiencia', 'tea', 'gestante', 'lactante', 'crianca_de_colo', 'obeso')),
    peso             SMALLINT GENERATED ALWAYS AS (
                         CASE prioridade WHEN '' THEN 0 WHEN 'idoso_80' THEN 2 ELSE 1 END) STORED,
    status           TEXT NOT NULL DEFAULT 'aguardando' CHECK (status IN
                         ('aguardando', 'chamado', 'em_atendimento', 'concluido', 'nao_compareceu', 'cancelado')),
    local            TEXT NOT NULL DEFAULT '' CHECK (char_length(local) <= 60),
    incluido_por     UUID REFERENCES users (id),
    profissional_id  UUID REFERENCES users (id),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    chamado_em       TIMESTAMPTZ,
    iniciado_em      TIMESTAMPTZ,
    encerrado_em     TIMESTAMPTZ,
    CONSTRAINT uq_sempras_atendimentos_senha UNIQUE (unidade_id, dia, senha)
);
-- Uma entrada ativa por munícipe, unidade e dia.
CREATE UNIQUE INDEX uq_sempras_atendimentos_ativo ON sempras_atendimentos (cidadao_id, unidade_id, dia)
    WHERE status IN ('aguardando', 'chamado', 'em_atendimento');
CREATE INDEX idx_sempras_atendimentos_fila ON sempras_atendimentos (unidade_id, dia, status, peso DESC, created_at);
CREATE INDEX idx_sempras_atendimentos_cidadao ON sempras_atendimentos (cidadao_id, created_at DESC);

-- Prontuário: registros de evolução, só de inclusão (documento com valor
-- legal — corrige-se com um novo registro).
CREATE TABLE sempras_registros (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cidadao_id      UUID NOT NULL REFERENCES sempras_cidadaos (id),
    unidade_id      UUID NOT NULL REFERENCES unidades (id),
    atendimento_id  UUID REFERENCES sempras_atendimentos (id),
    tipo            TEXT NOT NULL CHECK (tipo IN ('social', 'psicologico')),
    texto           TEXT NOT NULL CHECK (char_length(texto) BETWEEN 1 AND 20000),
    autor_id        UUID REFERENCES users (id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sempras_registros_cidadao ON sempras_registros (cidadao_id, created_at DESC);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION prevent_sempras_registros_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'sempras_registros é só de inclusão: % não é permitido', TG_OP;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER trg_sempras_registros_immutable
    BEFORE UPDATE OR DELETE ON sempras_registros
    FOR EACH ROW EXECUTE FUNCTION prevent_sempras_registros_mutation();

-- Perfis do SEMPRAS (editáveis na tela de Perfis). O superintendente é
-- concedido na entidade SEMPRAS; os demais, na unidade.
INSERT INTO perfis (slug, nome, descricao, permissoes) VALUES
    ('sempras-recepcionista', 'SEMPRAS — Recepcionista',
     'Busca e cadastra o munícipe e o inclui na fila da unidade.',
     ARRAY['sempras:recepcao']),
    ('sempras-tecnico', 'SEMPRAS — Técnico',
     'Recepção, chamada da fila, atendimento social e prontuário da unidade.',
     ARRAY['sempras:recepcao', 'sempras:atendimento']),
    ('sempras-psicologo', 'SEMPRAS — Psicólogo',
     'Atendimento técnico e psicológico, com os registros sigilosos da psicologia.',
     ARRAY['sempras:recepcao', 'sempras:atendimento', 'sempras:psicologia']),
    ('sempras-superintendente', 'SEMPRAS — Superintendente',
     'Conceda na entidade SEMPRAS: fila, atendimento e painel de todas as unidades.',
     ARRAY['sempras:recepcao', 'sempras:atendimento', 'sempras:gestao'])
ON CONFLICT (slug) DO NOTHING;

-- +goose Down
DELETE FROM perfis WHERE slug IN ('sempras-recepcionista', 'sempras-tecnico', 'sempras-psicologo', 'sempras-superintendente');
DROP TRIGGER IF EXISTS trg_sempras_registros_immutable ON sempras_registros;
DROP FUNCTION IF EXISTS prevent_sempras_registros_mutation();
DROP TABLE IF EXISTS sempras_registros;
DROP TABLE IF EXISTS sempras_atendimentos;
DROP TABLE IF EXISTS sempras_senhas;
DROP TABLE IF EXISTS sempras_cidadaos;
