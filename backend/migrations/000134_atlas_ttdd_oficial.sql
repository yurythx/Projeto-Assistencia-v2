-- +goose Up
-- TTDD conforme o documento oficial (docs/ttdd.pdf — CCPAD, Diário Oficial
-- de Rondonópolis), ADR 017:
--   * hierarquia órgão > função > subfunção > série documental, com a
--     publicação de cada TTDD (edição, data, versão) e a recomendação da
--     subfunção;
--   * prazo de cada fase em anos OU por condição ("Enquanto estiver
--     vigorando", "30 dias após a data do evento"); fase intermediária pode
--     não existir; destinação pode não estar definida na TTDD ("X");
--   * descritores longos (até ~400 caracteres) e busca full-text na TTDD;
--   * correção das 20 classificações semeadas pela 000132 (descritores
--     reescritos e dois prazos divergentes do documento) e do procedimento
--     de Dispensa, que apontava para a série de compra direta.
-- A carga completa (1.686 séries) é `make ttdd-aplicar` (deploy/ttdd).

CREATE TABLE atlas_ttdd_orgaos (
    prefixo          TEXT PRIMARY KEY CHECK (prefixo ~ '^[0-9]{1,2}\.0$'),
    nome             TEXT NOT NULL,
    edicao_diario    TEXT NOT NULL DEFAULT '',
    data_publicacao  DATE,
    versao           TEXT NOT NULL DEFAULT ''
);

CREATE TABLE atlas_ttdd_funcoes (
    codigo         TEXT PRIMARY KEY CHECK (codigo ~ '^[0-9]{1,2}\.0\.[0-9]{2}$'),
    orgao_prefixo  TEXT NOT NULL REFERENCES atlas_ttdd_orgaos (prefixo) ON DELETE RESTRICT,
    nome           TEXT NOT NULL
);

CREATE TABLE atlas_ttdd_subfuncoes (
    codigo         TEXT PRIMARY KEY CHECK (codigo ~ '^[0-9]{1,2}\.0\.[0-9]{2}\.[0-9]{2}$'),
    funcao_codigo  TEXT NOT NULL REFERENCES atlas_ttdd_funcoes (codigo) ON DELETE RESTRICT,
    nome           TEXT NOT NULL,
    recomendacao   TEXT NOT NULL DEFAULT ''
);

ALTER TABLE atlas_classificacao_ttdd ALTER COLUMN descritor TYPE TEXT;
ALTER TABLE atlas_classificacao_ttdd ADD COLUMN subfuncao_codigo TEXT REFERENCES atlas_ttdd_subfuncoes (codigo) ON DELETE RESTRICT;
ALTER TABLE atlas_classificacao_ttdd DROP CONSTRAINT IF EXISTS atlas_ttdd_fases_check;
ALTER TABLE atlas_classificacao_ttdd ALTER COLUMN fase_corrente_anos DROP NOT NULL, ALTER COLUMN fase_corrente_anos DROP DEFAULT;
ALTER TABLE atlas_classificacao_ttdd ALTER COLUMN fase_interm_anos DROP NOT NULL, ALTER COLUMN fase_interm_anos DROP DEFAULT;
ALTER TABLE atlas_classificacao_ttdd ADD COLUMN fase_corrente_condicao TEXT NOT NULL DEFAULT '';
ALTER TABLE atlas_classificacao_ttdd ADD COLUMN fase_interm_condicao TEXT NOT NULL DEFAULT '';
ALTER TABLE atlas_classificacao_ttdd ALTER COLUMN destinacao_final DROP NOT NULL;
UPDATE atlas_classificacao_ttdd SET observacoes = '' WHERE observacoes IS NULL;
ALTER TABLE atlas_classificacao_ttdd ALTER COLUMN observacoes SET DEFAULT '', ALTER COLUMN observacoes SET NOT NULL;
-- Cada fase: anos OU condição, nunca os dois; anos não negativos.
ALTER TABLE atlas_classificacao_ttdd ADD CONSTRAINT atlas_ttdd_fase_corrente_check
    CHECK (fase_corrente_anos IS NULL OR (fase_corrente_anos >= 0 AND fase_corrente_condicao = ''));
ALTER TABLE atlas_classificacao_ttdd ADD CONSTRAINT atlas_ttdd_fase_interm_check
    CHECK (fase_interm_anos IS NULL OR (fase_interm_anos >= 0 AND fase_interm_condicao = ''));
ALTER TABLE atlas_classificacao_ttdd ADD CONSTRAINT atlas_ttdd_codigo_check
    CHECK (codigo ~ '^[0-9]{1,2}\.0\.[0-9]{2}\.[0-9]{2}\.[0-9]{2}(-[0-9])?$');
ALTER TABLE atlas_classificacao_ttdd ADD COLUMN search TSVECTOR GENERATED ALWAYS AS (
    setweight(to_tsvector('portuguese', nexus_unaccent(codigo || ' ' || descritor)), 'A') ||
    setweight(to_tsvector('portuguese', nexus_unaccent(observacoes)), 'C')
) STORED;
CREATE INDEX idx_atlas_ttdd_search ON atlas_classificacao_ttdd USING gin (search);
CREATE INDEX idx_atlas_ttdd_subfuncao ON atlas_classificacao_ttdd (subfuncao_codigo);
CREATE INDEX idx_atlas_ttdd_funcoes_orgao ON atlas_ttdd_funcoes (orgao_prefixo);
CREATE INDEX idx_atlas_ttdd_subfuncoes_funcao ON atlas_ttdd_subfuncoes (funcao_codigo);

-- Classificações semeadas pela 000132 com os dados oficiais (+ 2.0.02.01.02,
-- usada pelo procedimento de Dispensa). GERADO de deploy/ttdd/ttdd.json.
INSERT INTO atlas_ttdd_orgaos (prefixo, nome, edicao_diario, data_publicacao, versao) VALUES ('2.0', 'Secretaria Municipal de Administração, Gestão de Pessoas e Inovação', '6.017', '2025-08-25', 'II')
    ON CONFLICT (prefixo) DO NOTHING;
INSERT INTO atlas_ttdd_funcoes (codigo, orgao_prefixo, nome) VALUES ('2.0.01', '2.0', 'Gestão Administrativa e Financeira') ON CONFLICT (codigo) DO NOTHING;
INSERT INTO atlas_ttdd_funcoes (codigo, orgao_prefixo, nome) VALUES ('2.0.02', '2.0', 'Gestão de Compras, Licitações e Contratos') ON CONFLICT (codigo) DO NOTHING;
INSERT INTO atlas_ttdd_funcoes (codigo, orgao_prefixo, nome) VALUES ('2.0.03', '2.0', 'Gestão do Arquivo Público Municipal') ON CONFLICT (codigo) DO NOTHING;
INSERT INTO atlas_ttdd_funcoes (codigo, orgao_prefixo, nome) VALUES ('2.0.05', '2.0', 'Gestão de Recursos Humanos') ON CONFLICT (codigo) DO NOTHING;
INSERT INTO atlas_ttdd_funcoes (codigo, orgao_prefixo, nome) VALUES ('2.0.07', '2.0', 'Registro e Controle da Vida Funcional') ON CONFLICT (codigo) DO NOTHING;
INSERT INTO atlas_ttdd_subfuncoes (codigo, funcao_codigo, nome, recomendacao) VALUES ('2.0.01.00', '2.0.01', 'Administração Geral', '') ON CONFLICT (codigo) DO NOTHING;
INSERT INTO atlas_ttdd_subfuncoes (codigo, funcao_codigo, nome, recomendacao) VALUES ('2.0.01.01', '2.0.01', 'Gestão de Almoxarifado', '') ON CONFLICT (codigo) DO NOTHING;
INSERT INTO atlas_ttdd_subfuncoes (codigo, funcao_codigo, nome, recomendacao) VALUES ('2.0.01.02', '2.0.01', 'Gestão de Frotas e Combustíveis', '') ON CONFLICT (codigo) DO NOTHING;
INSERT INTO atlas_ttdd_subfuncoes (codigo, funcao_codigo, nome, recomendacao) VALUES ('2.0.02.00', '2.0.02', 'Processo de Compra', '') ON CONFLICT (codigo) DO NOTHING;
INSERT INTO atlas_ttdd_subfuncoes (codigo, funcao_codigo, nome, recomendacao) VALUES ('2.0.03.01', '2.0.03', 'Gestão de Documentos', '') ON CONFLICT (codigo) DO NOTHING;
INSERT INTO atlas_ttdd_subfuncoes (codigo, funcao_codigo, nome, recomendacao) VALUES ('2.0.05.00', '2.0.05', 'Política de Pessoal', '') ON CONFLICT (codigo) DO NOTHING;
INSERT INTO atlas_ttdd_subfuncoes (codigo, funcao_codigo, nome, recomendacao) VALUES ('2.0.07.00', '2.0.07', 'Movimentação e Levantamento de Vida Funcional', '') ON CONFLICT (codigo) DO NOTHING;
INSERT INTO atlas_ttdd_subfuncoes (codigo, funcao_codigo, nome, recomendacao) VALUES ('2.0.02.01', '2.0.02', 'Processos de Licitação', '') ON CONFLICT (codigo) DO NOTHING;
INSERT INTO atlas_classificacao_ttdd (codigo, subfuncao_codigo, descritor, fase_corrente_anos, fase_corrente_condicao, fase_interm_anos, fase_interm_condicao, destinacao_final, observacoes)
    VALUES ('2.0.01.00.00', '2.0.01.00', 'Planejamento Institucional Secretarias e Autarquias', 1, '', 1, '', 'ELIMINACAO', '')
    ON CONFLICT (codigo) DO UPDATE SET subfuncao_codigo = EXCLUDED.subfuncao_codigo, descritor = EXCLUDED.descritor, fase_corrente_anos = EXCLUDED.fase_corrente_anos, fase_corrente_condicao = EXCLUDED.fase_corrente_condicao, fase_interm_anos = EXCLUDED.fase_interm_anos, fase_interm_condicao = EXCLUDED.fase_interm_condicao, destinacao_final = EXCLUDED.destinacao_final, observacoes = EXCLUDED.observacoes;
INSERT INTO atlas_classificacao_ttdd (codigo, subfuncao_codigo, descritor, fase_corrente_anos, fase_corrente_condicao, fase_interm_anos, fase_interm_condicao, destinacao_final, observacoes)
    VALUES ('2.0.01.00.01', '2.0.01.00', 'Organogramas', 1, '', 1, '', 'ELIMINACAO', '')
    ON CONFLICT (codigo) DO UPDATE SET subfuncao_codigo = EXCLUDED.subfuncao_codigo, descritor = EXCLUDED.descritor, fase_corrente_anos = EXCLUDED.fase_corrente_anos, fase_corrente_condicao = EXCLUDED.fase_corrente_condicao, fase_interm_anos = EXCLUDED.fase_interm_anos, fase_interm_condicao = EXCLUDED.fase_interm_condicao, destinacao_final = EXCLUDED.destinacao_final, observacoes = EXCLUDED.observacoes;
INSERT INTO atlas_classificacao_ttdd (codigo, subfuncao_codigo, descritor, fase_corrente_anos, fase_corrente_condicao, fase_interm_anos, fase_interm_condicao, destinacao_final, observacoes)
    VALUES ('2.0.01.00.02', '2.0.01.00', 'Fluxogramas', 1, '', 1, '', 'ELIMINACAO', '')
    ON CONFLICT (codigo) DO UPDATE SET subfuncao_codigo = EXCLUDED.subfuncao_codigo, descritor = EXCLUDED.descritor, fase_corrente_anos = EXCLUDED.fase_corrente_anos, fase_corrente_condicao = EXCLUDED.fase_corrente_condicao, fase_interm_anos = EXCLUDED.fase_interm_anos, fase_interm_condicao = EXCLUDED.fase_interm_condicao, destinacao_final = EXCLUDED.destinacao_final, observacoes = EXCLUDED.observacoes;
INSERT INTO atlas_classificacao_ttdd (codigo, subfuncao_codigo, descritor, fase_corrente_anos, fase_corrente_condicao, fase_interm_anos, fase_interm_condicao, destinacao_final, observacoes)
    VALUES ('2.0.01.00.05', '2.0.01.00', 'Gestão de contas e utilização de Energia Elétrica', 1, '', 1, '', 'ELIMINACAO', '')
    ON CONFLICT (codigo) DO UPDATE SET subfuncao_codigo = EXCLUDED.subfuncao_codigo, descritor = EXCLUDED.descritor, fase_corrente_anos = EXCLUDED.fase_corrente_anos, fase_corrente_condicao = EXCLUDED.fase_corrente_condicao, fase_interm_anos = EXCLUDED.fase_interm_anos, fase_interm_condicao = EXCLUDED.fase_interm_condicao, destinacao_final = EXCLUDED.destinacao_final, observacoes = EXCLUDED.observacoes;
INSERT INTO atlas_classificacao_ttdd (codigo, subfuncao_codigo, descritor, fase_corrente_anos, fase_corrente_condicao, fase_interm_anos, fase_interm_condicao, destinacao_final, observacoes)
    VALUES ('2.0.01.00.07', '2.0.01.00', 'Gestão de Contas e Utilização de Água', 1, '', 1, '', 'ELIMINACAO', '')
    ON CONFLICT (codigo) DO UPDATE SET subfuncao_codigo = EXCLUDED.subfuncao_codigo, descritor = EXCLUDED.descritor, fase_corrente_anos = EXCLUDED.fase_corrente_anos, fase_corrente_condicao = EXCLUDED.fase_corrente_condicao, fase_interm_anos = EXCLUDED.fase_interm_anos, fase_interm_condicao = EXCLUDED.fase_interm_condicao, destinacao_final = EXCLUDED.destinacao_final, observacoes = EXCLUDED.observacoes;
INSERT INTO atlas_classificacao_ttdd (codigo, subfuncao_codigo, descritor, fase_corrente_anos, fase_corrente_condicao, fase_interm_anos, fase_interm_condicao, destinacao_final, observacoes)
    VALUES ('2.0.01.01.00', '2.0.01.01', 'Nota de Transferência de Material Permanente', 1, '', 1, '', 'GUARDA_PERMANENTE', '')
    ON CONFLICT (codigo) DO UPDATE SET subfuncao_codigo = EXCLUDED.subfuncao_codigo, descritor = EXCLUDED.descritor, fase_corrente_anos = EXCLUDED.fase_corrente_anos, fase_corrente_condicao = EXCLUDED.fase_corrente_condicao, fase_interm_anos = EXCLUDED.fase_interm_anos, fase_interm_condicao = EXCLUDED.fase_interm_condicao, destinacao_final = EXCLUDED.destinacao_final, observacoes = EXCLUDED.observacoes;
INSERT INTO atlas_classificacao_ttdd (codigo, subfuncao_codigo, descritor, fase_corrente_anos, fase_corrente_condicao, fase_interm_anos, fase_interm_condicao, destinacao_final, observacoes)
    VALUES ('2.0.01.01.02', '2.0.01.01', 'Solicitação de Material (saída de Material)', 1, '', 1, '', 'ELIMINACAO', '')
    ON CONFLICT (codigo) DO UPDATE SET subfuncao_codigo = EXCLUDED.subfuncao_codigo, descritor = EXCLUDED.descritor, fase_corrente_anos = EXCLUDED.fase_corrente_anos, fase_corrente_condicao = EXCLUDED.fase_corrente_condicao, fase_interm_anos = EXCLUDED.fase_interm_anos, fase_interm_condicao = EXCLUDED.fase_interm_condicao, destinacao_final = EXCLUDED.destinacao_final, observacoes = EXCLUDED.observacoes;
INSERT INTO atlas_classificacao_ttdd (codigo, subfuncao_codigo, descritor, fase_corrente_anos, fase_corrente_condicao, fase_interm_anos, fase_interm_condicao, destinacao_final, observacoes)
    VALUES ('2.0.01.01.04', '2.0.01.01', 'Inventário Anual - Controle de estoque do almoxarifado', 1, '', 1, '', 'GUARDA_PERMANENTE', '')
    ON CONFLICT (codigo) DO UPDATE SET subfuncao_codigo = EXCLUDED.subfuncao_codigo, descritor = EXCLUDED.descritor, fase_corrente_anos = EXCLUDED.fase_corrente_anos, fase_corrente_condicao = EXCLUDED.fase_corrente_condicao, fase_interm_anos = EXCLUDED.fase_interm_anos, fase_interm_condicao = EXCLUDED.fase_interm_condicao, destinacao_final = EXCLUDED.destinacao_final, observacoes = EXCLUDED.observacoes;
INSERT INTO atlas_classificacao_ttdd (codigo, subfuncao_codigo, descritor, fase_corrente_anos, fase_corrente_condicao, fase_interm_anos, fase_interm_condicao, destinacao_final, observacoes)
    VALUES ('2.0.01.02.02', '2.0.01.02', 'Licenciamento de veículos oficiais', 1, '', 1, '', 'ELIMINACAO', '')
    ON CONFLICT (codigo) DO UPDATE SET subfuncao_codigo = EXCLUDED.subfuncao_codigo, descritor = EXCLUDED.descritor, fase_corrente_anos = EXCLUDED.fase_corrente_anos, fase_corrente_condicao = EXCLUDED.fase_corrente_condicao, fase_interm_anos = EXCLUDED.fase_interm_anos, fase_interm_condicao = EXCLUDED.fase_interm_condicao, destinacao_final = EXCLUDED.destinacao_final, observacoes = EXCLUDED.observacoes;
INSERT INTO atlas_classificacao_ttdd (codigo, subfuncao_codigo, descritor, fase_corrente_anos, fase_corrente_condicao, fase_interm_anos, fase_interm_condicao, destinacao_final, observacoes)
    VALUES ('2.0.01.02.14', '2.0.01.02', 'Acidentes, Emergência e Sinistros referente aos Veículos da Frota Oficial', 1, '', 1, '', 'GUARDA_PERMANENTE', '')
    ON CONFLICT (codigo) DO UPDATE SET subfuncao_codigo = EXCLUDED.subfuncao_codigo, descritor = EXCLUDED.descritor, fase_corrente_anos = EXCLUDED.fase_corrente_anos, fase_corrente_condicao = EXCLUDED.fase_corrente_condicao, fase_interm_anos = EXCLUDED.fase_interm_anos, fase_interm_condicao = EXCLUDED.fase_interm_condicao, destinacao_final = EXCLUDED.destinacao_final, observacoes = EXCLUDED.observacoes;
INSERT INTO atlas_classificacao_ttdd (codigo, subfuncao_codigo, descritor, fase_corrente_anos, fase_corrente_condicao, fase_interm_anos, fase_interm_condicao, destinacao_final, observacoes)
    VALUES ('2.0.01.02.16', '2.0.01.02', 'Renovação e Aquisição de Veículos da Frota Oficial', 1, '', 1, '', 'GUARDA_PERMANENTE', '')
    ON CONFLICT (codigo) DO UPDATE SET subfuncao_codigo = EXCLUDED.subfuncao_codigo, descritor = EXCLUDED.descritor, fase_corrente_anos = EXCLUDED.fase_corrente_anos, fase_corrente_condicao = EXCLUDED.fase_corrente_condicao, fase_interm_anos = EXCLUDED.fase_interm_anos, fase_interm_condicao = EXCLUDED.fase_interm_condicao, destinacao_final = EXCLUDED.destinacao_final, observacoes = EXCLUDED.observacoes;
INSERT INTO atlas_classificacao_ttdd (codigo, subfuncao_codigo, descritor, fase_corrente_anos, fase_corrente_condicao, fase_interm_anos, fase_interm_condicao, destinacao_final, observacoes)
    VALUES ('2.0.02.00.00', '2.0.02.00', 'Termo de Referência (TR)', 1, '', 1, '', 'GUARDA_PERMANENTE', '')
    ON CONFLICT (codigo) DO UPDATE SET subfuncao_codigo = EXCLUDED.subfuncao_codigo, descritor = EXCLUDED.descritor, fase_corrente_anos = EXCLUDED.fase_corrente_anos, fase_corrente_condicao = EXCLUDED.fase_corrente_condicao, fase_interm_anos = EXCLUDED.fase_interm_anos, fase_interm_condicao = EXCLUDED.fase_interm_condicao, destinacao_final = EXCLUDED.destinacao_final, observacoes = EXCLUDED.observacoes;
INSERT INTO atlas_classificacao_ttdd (codigo, subfuncao_codigo, descritor, fase_corrente_anos, fase_corrente_condicao, fase_interm_anos, fase_interm_condicao, destinacao_final, observacoes)
    VALUES ('2.0.02.00.01', '2.0.02.00', 'Processos relativos à Atestado de Capacidade Técnica', 1, '', 1, '', 'ELIMINACAO', '')
    ON CONFLICT (codigo) DO UPDATE SET subfuncao_codigo = EXCLUDED.subfuncao_codigo, descritor = EXCLUDED.descritor, fase_corrente_anos = EXCLUDED.fase_corrente_anos, fase_corrente_condicao = EXCLUDED.fase_corrente_condicao, fase_interm_anos = EXCLUDED.fase_interm_anos, fase_interm_condicao = EXCLUDED.fase_interm_condicao, destinacao_final = EXCLUDED.destinacao_final, observacoes = EXCLUDED.observacoes;
INSERT INTO atlas_classificacao_ttdd (codigo, subfuncao_codigo, descritor, fase_corrente_anos, fase_corrente_condicao, fase_interm_anos, fase_interm_condicao, destinacao_final, observacoes)
    VALUES ('2.0.02.00.04', '2.0.02.00', 'Processos de compra direta – Produtos e Serviços Técnicos Especializados)', 1, '', 1, '', 'GUARDA_PERMANENTE', '')
    ON CONFLICT (codigo) DO UPDATE SET subfuncao_codigo = EXCLUDED.subfuncao_codigo, descritor = EXCLUDED.descritor, fase_corrente_anos = EXCLUDED.fase_corrente_anos, fase_corrente_condicao = EXCLUDED.fase_corrente_condicao, fase_interm_anos = EXCLUDED.fase_interm_anos, fase_interm_condicao = EXCLUDED.fase_interm_condicao, destinacao_final = EXCLUDED.destinacao_final, observacoes = EXCLUDED.observacoes;
INSERT INTO atlas_classificacao_ttdd (codigo, subfuncao_codigo, descritor, fase_corrente_anos, fase_corrente_condicao, fase_interm_anos, fase_interm_condicao, destinacao_final, observacoes)
    VALUES ('2.0.02.00.07', '2.0.02.00', 'Processos relativos a Pregão Presencial/Pregão Eletrônico', 1, '', 1, '', 'GUARDA_PERMANENTE', '')
    ON CONFLICT (codigo) DO UPDATE SET subfuncao_codigo = EXCLUDED.subfuncao_codigo, descritor = EXCLUDED.descritor, fase_corrente_anos = EXCLUDED.fase_corrente_anos, fase_corrente_condicao = EXCLUDED.fase_corrente_condicao, fase_interm_anos = EXCLUDED.fase_interm_anos, fase_interm_condicao = EXCLUDED.fase_interm_condicao, destinacao_final = EXCLUDED.destinacao_final, observacoes = EXCLUDED.observacoes;
INSERT INTO atlas_classificacao_ttdd (codigo, subfuncao_codigo, descritor, fase_corrente_anos, fase_corrente_condicao, fase_interm_anos, fase_interm_condicao, destinacao_final, observacoes)
    VALUES ('2.0.02.00.08', '2.0.02.00', 'Processos relativos a recursos contra compras e licitações', 1, '', 1, '', 'GUARDA_PERMANENTE', '')
    ON CONFLICT (codigo) DO UPDATE SET subfuncao_codigo = EXCLUDED.subfuncao_codigo, descritor = EXCLUDED.descritor, fase_corrente_anos = EXCLUDED.fase_corrente_anos, fase_corrente_condicao = EXCLUDED.fase_corrente_condicao, fase_interm_anos = EXCLUDED.fase_interm_anos, fase_interm_condicao = EXCLUDED.fase_interm_condicao, destinacao_final = EXCLUDED.destinacao_final, observacoes = EXCLUDED.observacoes;
INSERT INTO atlas_classificacao_ttdd (codigo, subfuncao_codigo, descritor, fase_corrente_anos, fase_corrente_condicao, fase_interm_anos, fase_interm_condicao, destinacao_final, observacoes)
    VALUES ('2.0.03.01.04', '2.0.03.01', 'Tabelas de Temporalidade e Destinação de Documentos / Código de Classificação de documentos', 1, '', 1, '', 'GUARDA_PERMANENTE', '')
    ON CONFLICT (codigo) DO UPDATE SET subfuncao_codigo = EXCLUDED.subfuncao_codigo, descritor = EXCLUDED.descritor, fase_corrente_anos = EXCLUDED.fase_corrente_anos, fase_corrente_condicao = EXCLUDED.fase_corrente_condicao, fase_interm_anos = EXCLUDED.fase_interm_anos, fase_interm_condicao = EXCLUDED.fase_interm_condicao, destinacao_final = EXCLUDED.destinacao_final, observacoes = EXCLUDED.observacoes;
INSERT INTO atlas_classificacao_ttdd (codigo, subfuncao_codigo, descritor, fase_corrente_anos, fase_corrente_condicao, fase_interm_anos, fase_interm_condicao, destinacao_final, observacoes)
    VALUES ('2.0.03.01.05', '2.0.03.01', 'Processo de Eliminação de documentos', 1, '', 1, '', 'GUARDA_PERMANENTE', '')
    ON CONFLICT (codigo) DO UPDATE SET subfuncao_codigo = EXCLUDED.subfuncao_codigo, descritor = EXCLUDED.descritor, fase_corrente_anos = EXCLUDED.fase_corrente_anos, fase_corrente_condicao = EXCLUDED.fase_corrente_condicao, fase_interm_anos = EXCLUDED.fase_interm_anos, fase_interm_condicao = EXCLUDED.fase_interm_condicao, destinacao_final = EXCLUDED.destinacao_final, observacoes = EXCLUDED.observacoes;
INSERT INTO atlas_classificacao_ttdd (codigo, subfuncao_codigo, descritor, fase_corrente_anos, fase_corrente_condicao, fase_interm_anos, fase_interm_condicao, destinacao_final, observacoes)
    VALUES ('2.0.05.00.00', '2.0.05.00', 'Normatização. Regulamentação. Legislação de Pessoal', NULL, 'Enquanto estiver vigorando', 1, '', 'GUARDA_PERMANENTE', '')
    ON CONFLICT (codigo) DO UPDATE SET subfuncao_codigo = EXCLUDED.subfuncao_codigo, descritor = EXCLUDED.descritor, fase_corrente_anos = EXCLUDED.fase_corrente_anos, fase_corrente_condicao = EXCLUDED.fase_corrente_condicao, fase_interm_anos = EXCLUDED.fase_interm_anos, fase_interm_condicao = EXCLUDED.fase_interm_condicao, destinacao_final = EXCLUDED.destinacao_final, observacoes = EXCLUDED.observacoes;
INSERT INTO atlas_classificacao_ttdd (codigo, subfuncao_codigo, descritor, fase_corrente_anos, fase_corrente_condicao, fase_interm_anos, fase_interm_condicao, destinacao_final, observacoes)
    VALUES ('2.0.07.00.00', '2.0.07.00', 'Pasta funcional de Servidores Ativos, inativos e aposentados', 1, '', 99, '', 'ELIMINACAO', '')
    ON CONFLICT (codigo) DO UPDATE SET subfuncao_codigo = EXCLUDED.subfuncao_codigo, descritor = EXCLUDED.descritor, fase_corrente_anos = EXCLUDED.fase_corrente_anos, fase_corrente_condicao = EXCLUDED.fase_corrente_condicao, fase_interm_anos = EXCLUDED.fase_interm_anos, fase_interm_condicao = EXCLUDED.fase_interm_condicao, destinacao_final = EXCLUDED.destinacao_final, observacoes = EXCLUDED.observacoes;
INSERT INTO atlas_classificacao_ttdd (codigo, subfuncao_codigo, descritor, fase_corrente_anos, fase_corrente_condicao, fase_interm_anos, fase_interm_condicao, destinacao_final, observacoes)
    VALUES ('2.0.02.01.02', '2.0.02.01', 'Processos de Dispensa de licitação', 1, '', 1, '', 'GUARDA_PERMANENTE', '')
    ON CONFLICT (codigo) DO UPDATE SET subfuncao_codigo = EXCLUDED.subfuncao_codigo, descritor = EXCLUDED.descritor, fase_corrente_anos = EXCLUDED.fase_corrente_anos, fase_corrente_condicao = EXCLUDED.fase_corrente_condicao, fase_interm_anos = EXCLUDED.fase_interm_anos, fase_interm_condicao = EXCLUDED.fase_interm_condicao, destinacao_final = EXCLUDED.destinacao_final, observacoes = EXCLUDED.observacoes;

-- Dispensa de licitação é a série 2.0.02.01.02 (2.0.02.00.04 é compra direta
-- de produtos e serviços técnicos especializados).
UPDATE atlas_workflows SET codigo_ttdd = '2.0.02.01.02' WHERE codigo_processual = 'ADM.DIR.002' AND codigo_ttdd = '2.0.02.00.04';

-- +goose Down
UPDATE atlas_workflows SET codigo_ttdd = '2.0.02.00.04' WHERE codigo_processual = 'ADM.DIR.002' AND codigo_ttdd = '2.0.02.01.02';
-- Linhas que o esquema antigo não representa: remove as não usadas por
-- procedimentos; nas usadas, valores padrão.
DELETE FROM atlas_classificacao_ttdd WHERE codigo NOT IN (SELECT codigo_ttdd FROM atlas_workflows)
    AND (fase_corrente_anos IS NULL OR fase_interm_anos IS NULL OR destinacao_final IS NULL OR length(descritor) > 255 OR codigo LIKE '%-%');
UPDATE atlas_classificacao_ttdd SET fase_corrente_anos = COALESCE(fase_corrente_anos, 0), fase_interm_anos = COALESCE(fase_interm_anos, 0),
    destinacao_final = COALESCE(destinacao_final, 'GUARDA_PERMANENTE');
DROP INDEX IF EXISTS idx_atlas_ttdd_subfuncao;
DROP INDEX IF EXISTS idx_atlas_ttdd_search;
ALTER TABLE atlas_classificacao_ttdd DROP COLUMN IF EXISTS search;
ALTER TABLE atlas_classificacao_ttdd DROP CONSTRAINT IF EXISTS atlas_ttdd_codigo_check;
ALTER TABLE atlas_classificacao_ttdd DROP CONSTRAINT IF EXISTS atlas_ttdd_fase_interm_check;
ALTER TABLE atlas_classificacao_ttdd DROP CONSTRAINT IF EXISTS atlas_ttdd_fase_corrente_check;
ALTER TABLE atlas_classificacao_ttdd ALTER COLUMN observacoes DROP NOT NULL, ALTER COLUMN observacoes DROP DEFAULT;
ALTER TABLE atlas_classificacao_ttdd ALTER COLUMN destinacao_final SET NOT NULL;
ALTER TABLE atlas_classificacao_ttdd DROP COLUMN IF EXISTS fase_interm_condicao;
ALTER TABLE atlas_classificacao_ttdd DROP COLUMN IF EXISTS fase_corrente_condicao;
ALTER TABLE atlas_classificacao_ttdd ALTER COLUMN fase_interm_anos SET DEFAULT 0, ALTER COLUMN fase_interm_anos SET NOT NULL;
ALTER TABLE atlas_classificacao_ttdd ALTER COLUMN fase_corrente_anos SET DEFAULT 1, ALTER COLUMN fase_corrente_anos SET NOT NULL;
ALTER TABLE atlas_classificacao_ttdd ADD CONSTRAINT atlas_ttdd_fases_check CHECK (fase_corrente_anos >= 0 AND fase_interm_anos >= 0);
ALTER TABLE atlas_classificacao_ttdd DROP COLUMN IF EXISTS subfuncao_codigo;
UPDATE atlas_classificacao_ttdd SET descritor = left(descritor, 255) WHERE length(descritor) > 255;
ALTER TABLE atlas_classificacao_ttdd ALTER COLUMN descritor TYPE VARCHAR(255);
DROP TABLE IF EXISTS atlas_ttdd_subfuncoes;
DROP TABLE IF EXISTS atlas_ttdd_funcoes;
DROP TABLE IF EXISTS atlas_ttdd_orgaos;
