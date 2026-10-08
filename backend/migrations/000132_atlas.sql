-- +goose Up
-- ==============================================================================
-- 000132_atlas.sql: Módulo Atlas (Catálogo Procedural, SEI e Temporalidade TTDD)
-- Padrão SEI de Processo Administrativo Eletrônico e Resoluções CONARQ / CCPAD
-- ==============================================================================

-- 1. Classificação Documental e Temporalidade (TTDD / CCPAD)
CREATE TABLE IF NOT EXISTS atlas_classificacao_ttdd (
    codigo VARCHAR(32) PRIMARY KEY,
    descritor VARCHAR(255) NOT NULL,
    fase_corrente_anos INT NOT NULL DEFAULT 1,
    fase_interm_anos INT NOT NULL DEFAULT 0,
    destinacao_final VARCHAR(32) NOT NULL CHECK (destinacao_final IN ('GUARDA_PERMANENTE', 'ELIMINACAO')),
    observacoes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 2. Tabela Mestra de Workflows / Tipologias de Processo
CREATE TABLE IF NOT EXISTS atlas_workflows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id VARCHAR(64) NOT NULL,
    codigo_processual VARCHAR(64) NOT NULL,
    titulo VARCHAR(255) NOT NULL,
    objetivo TEXT NOT NULL,
    publico_alvo VARCHAR(255) NOT NULL,
    versao INT NOT NULL DEFAULT 1,
    ativo BOOLEAN NOT NULL DEFAULT TRUE,
    nivel_acesso VARCHAR(32) NOT NULL CHECK (nivel_acesso IN ('PUBLICO', 'RESTRITO', 'SIGILOSO')),
    hipotese_legal_restricao VARCHAR(255),
    codigo_ttdd VARCHAR(32) NOT NULL REFERENCES atlas_classificacao_ttdd(codigo) ON UPDATE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_atlas_workflow_versao UNIQUE (tenant_id, codigo_processual, versao)
);
CREATE INDEX IF NOT EXISTS idx_atlas_wf_tenant ON atlas_workflows(tenant_id, ativo);
CREATE INDEX IF NOT EXISTS idx_atlas_wf_ttdd ON atlas_workflows(codigo_ttdd);

-- 3. Etapas do Fluxo (Nós de Tramitação)
CREATE TABLE IF NOT EXISTS atlas_etapas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID NOT NULL REFERENCES atlas_workflows(id) ON DELETE CASCADE,
    ordem INT NOT NULL,
    unidade_administrativa VARCHAR(64) NOT NULL,
    nome_setor VARCHAR(150) NOT NULL,
    atribuicoes_setor TEXT NOT NULL,
    prazo_sla_em_dias INT NOT NULL DEFAULT 2,
    manter_aberto_apos_remessa BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_atlas_etapa_ordem UNIQUE (workflow_id, ordem)
);
CREATE INDEX IF NOT EXISTS idx_atlas_etapa_wf ON atlas_etapas(workflow_id);

-- 4. Peças e Minutas Exigidas por Etapa
CREATE TABLE IF NOT EXISTS atlas_etapa_documentos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    etapa_id UUID NOT NULL REFERENCES atlas_etapas(id) ON DELETE CASCADE,
    nome_documento VARCHAR(150) NOT NULL,
    obrigatorio BOOLEAN NOT NULL DEFAULT TRUE,
    formato VARCHAR(32) NOT NULL CHECK (formato IN ('NATO_DIGITAL', 'EXTERNO_DIGITALIZADO')),
    tipo_assinatura VARCHAR(32) NOT NULL CHECK (tipo_assinatura IN ('INDIVIDUAL', 'CONJUNTA_MULTINIVEL', 'EM_BLOCO')),
    exige_conferencia_copia BOOLEAN NOT NULL DEFAULT FALSE,
    modelo_minuta_padrao_url TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_atlas_docs_etapa ON atlas_etapa_documentos(etapa_id);

-- 5. Regras de Transição e Trilhas de Diligência
CREATE TABLE IF NOT EXISTS atlas_etapa_transicoes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    origem_etapa_id UUID NOT NULL REFERENCES atlas_etapas(id) ON DELETE CASCADE,
    destino_etapa_id UUID NOT NULL REFERENCES atlas_etapas(id) ON DELETE CASCADE,
    condicao_transicao VARCHAR(255) NOT NULL,
    is_devolucao_diligencia BOOLEAN NOT NULL DEFAULT FALSE,
    descricao_diligencia TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ==============================================================================
-- SEEDS CANÔNICAS EXTRAÍDAS DA TTDD OFICIAL (docs/ttdd.pdf - CCPAD / Rondonópolis)
-- ==============================================================================

-- Classificações Documentais Reais da TTDD
INSERT INTO atlas_classificacao_ttdd (codigo, descritor, fase_corrente_anos, fase_interm_anos, destinacao_final, observacoes) VALUES
('2.0.01.00.00', 'Planejamento Institucional Secretarias e Autarquias', 1, 1, 'ELIMINACAO', 'Registros de planejamento e reuniões setoriais de gestão geral'),
('2.0.01.00.01', 'Organogramas e Estruturas Departamentais', 1, 1, 'ELIMINACAO', 'Histórico de versões de organogramas setoriais'),
('2.0.01.00.02', 'Fluxogramas e Mapeamento de Processos de Trabalho', 1, 1, 'ELIMINACAO', 'Manuais de procedimentos operacionais e fluxos padronizados'),
('2.0.01.00.05', 'Gestão de contas e utilização de Energia Elétrica', 1, 1, 'ELIMINACAO', 'Contas e atestes de faturas de concessionárias de energia'),
('2.0.01.00.07', 'Gestão de Contas e Utilização de Água e Saneamento', 1, 1, 'ELIMINACAO', 'Contas de água das repartições públicas municipais'),
('2.0.01.01.00', 'Nota de Transferência de Material Permanente', 1, 1, 'GUARDA_PERMANENTE', 'Termo de responsabilidade patrimonial e tombamento de bens móveis'),
('2.0.01.01.02', 'Solicitação de Material (saída de almoxarifado)', 1, 1, 'ELIMINACAO', 'Requisições internas de consumo e suprimentos de escritório'),
('2.0.01.01.04', 'Inventário Anual - Controle de estoque do almoxarifado', 1, 1, 'GUARDA_PERMANENTE', 'Relatório contábil anual de fechamento de bens em estoque'),
('2.0.01.02.02', 'Licenciamento de veículos oficiais da frota', 1, 1, 'ELIMINACAO', 'CRLV, taxas e seguros obrigatórios dos veículos municipais'),
('2.0.01.02.14', 'Acidentes, Emergência e Sinistros referente aos Veículos da Frota Oficial', 1, 1, 'GUARDA_PERMANENTE', 'Boletins de ocorrência, sindicâncias e indenizações de sinistros'),
('2.0.01.02.16', 'Renovação e Aquisição de Veículos da Frota Oficial', 1, 1, 'GUARDA_PERMANENTE', 'Processo de desincorporação, leilão e incorporação de novos veículos'),
('2.0.02.00.00', 'Termo de Referência (TR) e Estudos Técnicos Preliminares', 1, 1, 'GUARDA_PERMANENTE', 'Peças técnicas basilares para instrução da fase preparatória de licitações'),
('2.0.02.00.01', 'Processos relativos a Atestado de Capacidade Técnica', 1, 1, 'ELIMINACAO', 'Emissão e validação de atestados de fornecedores de serviços'),
('2.0.02.00.04', 'Processos de Compra Direta (Dispensa e Inexigibilidade de Licitação)', 1, 1, 'GUARDA_PERMANENTE', 'Instrução completa com pareceres jurídicos e justificativas de preços'),
('2.0.02.00.07', 'Processos relativos a Pregão Presencial / Pregão Eletrônico', 1, 1, 'GUARDA_PERMANENTE', 'Processo licitatório integral com atas, lances, recursos e homologação'),
('2.0.02.00.08', 'Processos relativos a recursos contra compras e licitações', 1, 1, 'GUARDA_PERMANENTE', 'Peças recursais e julgamentos da autoridade competente'),
('2.0.03.01.04', 'Tabelas de Temporalidade e Destinação de Documentos (TTDD)', 1, 1, 'GUARDA_PERMANENTE', 'Atos de aprovação, decretos regulamentares e matrizes de temporalidade'),
('2.0.03.01.05', 'Processo de Eliminação de documentos públicos', 1, 1, 'GUARDA_PERMANENTE', 'Editais de ciência de eliminação, termos de eliminação e atas da CCPAD'),
('2.0.05.00.00', 'Gestão de Recursos Humanos - Concessão de Direitos e Vantagens', 5, 20, 'GUARDA_PERMANENTE', 'Histórico funcional de licenças, progressões e adicionais'),
('2.0.07.00.00', 'Registro e Controle da Vida Funcional do Servidor', 5, 50, 'GUARDA_PERMANENTE', 'Dossiê do servidor público, assentamentos e tempo de contribuição')
ON CONFLICT (codigo) DO NOTHING;

-- Workflows Canônicos SEI
-- +goose StatementBegin
DO $$
DECLARE
    wf_pregao UUID := 'a1000000-0000-0000-0000-000000000001';
    wf_dispensa UUID := 'a1000000-0000-0000-0000-000000000002';
    wf_patrimonio UUID := 'a1000000-0000-0000-0000-000000000003';
    wf_eliminacao UUID := 'a1000000-0000-0000-0000-000000000004';
    
    et_pr_1 UUID := 'b1000000-0000-0000-0000-000000000001';
    et_pr_2 UUID := 'b1000000-0000-0000-0000-000000000002';
    et_pr_3 UUID := 'b1000000-0000-0000-0000-000000000003';
    et_pr_4 UUID := 'b1000000-0000-0000-0000-000000000004';
    et_pr_5 UUID := 'b1000000-0000-0000-0000-000000000005';

    et_disp_1 UUID := 'b2000000-0000-0000-0000-000000000001';
    et_disp_2 UUID := 'b2000000-0000-0000-0000-000000000002';
    et_disp_3 UUID := 'b2000000-0000-0000-0000-000000000003';

    et_pat_1 UUID := 'b3000000-0000-0000-0000-000000000001';
    et_pat_2 UUID := 'b3000000-0000-0000-0000-000000000002';
BEGIN
    -- Workflow 1: Pregão Eletrônico (Lei 14.133/2021)
    INSERT INTO atlas_workflows (id, tenant_id, codigo_processual, titulo, objetivo, publico_alvo, versao, ativo, nivel_acesso, codigo_ttdd)
    VALUES (
        wf_pregao, 'nexus', 'ADM.LIC.001',
        'Processo Licitatório de Pregão Eletrônico (Aquisição de Bens e Serviços)',
        'Roteiro procedimental padronizado para aquisições públicas via pregão eletrônico, desde o DFD até a homologação contratual.',
        'Secretarias Municipais, Departamento de Compras e Pregoeiros',
        1, TRUE, 'PUBLICO', '2.0.02.00.07'
    ) ON CONFLICT (tenant_id, codigo_processual, versao) DO NOTHING;

    -- Etapas do Pregão
    INSERT INTO atlas_etapas (id, workflow_id, ordem, unidade_administrativa, nome_setor, atribuicoes_setor, prazo_sla_em_dias, manter_aberto_apos_remessa)
    VALUES
    (et_pr_1, wf_pregao, 1, 'SEC/DEMANDANTE', 'Setor Demandante / Requisitante', 'Elaboração do Documento de Formalização de Demanda (DFD), Estudo Técnico Preliminar (ETP) e Termo de Referência (TR).', 5, TRUE),
    (et_pr_2, wf_pregao, 2, 'SEFIN/COMPRAS', 'Departamento de Compras e Pesquisa de Preços', 'Realização da pesquisa de mercado, consolidação da cesta de preços aceitáveis e dotação orçamentária.', 7, FALSE),
    (et_pr_3, wf_pregao, 3, 'PGM/CONSULTORIA', 'Procuradoria Geral do Município', 'Análise jurídica da minuta de edital e do termo de referência; emissão de parecer conclusivo.', 10, FALSE),
    (et_pr_4, wf_pregao, 4, 'CPL/PREGOEIROS', 'Comissão Permanente de Licitação / Pregoeiro', 'Publicação do edital, condução da sessão pública, análise de propostas, habilitação e julgamento.', 15, TRUE),
    (et_pr_5, wf_pregao, 5, 'GAB/AUTORIDADE', 'Gabinete da Autoridade Competente (Secretário)', 'Adjudicação do objeto, deliberação sobre recursos pendentes e homologação final do certame.', 3, FALSE)
    ON CONFLICT (workflow_id, ordem) DO NOTHING;

    -- Documentos / Peças Exigidas no Pregão
    INSERT INTO atlas_etapa_documentos (etapa_id, nome_documento, obrigatorio, formato, tipo_assinatura, exige_conferencia_copia)
    VALUES
    (et_pr_1, 'Documento de Formalização da Demanda (DFD)', TRUE, 'NATO_DIGITAL', 'INDIVIDUAL', FALSE),
    (et_pr_1, 'Estudo Técnico Preliminar (ETP)', TRUE, 'NATO_DIGITAL', 'CONJUNTA_MULTINIVEL', FALSE),
    (et_pr_1, 'Termo de Referência (TR)', TRUE, 'NATO_DIGITAL', 'CONJUNTA_MULTINIVEL', FALSE),
    (et_pr_2, 'Mapa Comparativo de Preços de Mercado', TRUE, 'NATO_DIGITAL', 'INDIVIDUAL', FALSE),
    (et_pr_2, 'Declaração de Adequação Orçamentária e Financeira', TRUE, 'NATO_DIGITAL', 'INDIVIDUAL', FALSE),
    (et_pr_3, 'Parecer Jurídico Licitatório', TRUE, 'NATO_DIGITAL', 'INDIVIDUAL', FALSE),
    (et_pr_4, 'Minuta de Edital Homologada', TRUE, 'NATO_DIGITAL', 'CONJUNTA_MULTINIVEL', FALSE),
    (et_pr_4, 'Ata da Sessão Pública do Pregão', TRUE, 'EXTERNO_DIGITALIZADO', 'EM_BLOCO', TRUE),
    (et_pr_4, 'Documentos de Habilitação do Fornecedor Vencedor', TRUE, 'EXTERNO_DIGITALIZADO', 'INDIVIDUAL', TRUE),
    (et_pr_5, 'Termo de Homologação e Adjudicação', TRUE, 'NATO_DIGITAL', 'INDIVIDUAL', FALSE)
    ON CONFLICT DO NOTHING;

    -- Transições do Pregão
    INSERT INTO atlas_etapa_transicoes (origem_etapa_id, destino_etapa_id, condicao_transicao, is_devolucao_diligencia, descricao_diligencia)
    VALUES
    (et_pr_1, et_pr_2, 'TR e ETP aprovados pela chefia da unidade demandante', FALSE, NULL),
    (et_pr_2, et_pr_3, 'Cesta de preços consolidada e dotação confirmada pela SEFIN', FALSE, NULL),
    (et_pr_3, et_pr_4, 'Parecer jurídico favorável sem ressalvas impeditivas', FALSE, NULL),
    (et_pr_3, et_pr_1, 'Parecer jurídico apontando necessidade de saneamento do TR', TRUE, 'Diligência jurídica para adequação das especificações técnicas ou justificativas do objeto'),
    (et_pr_4, et_pr_5, 'Proposta aceita, fornecedor habilitado e prazo recursal encerrado', FALSE, NULL),
    (et_pr_4, et_pr_2, 'Preço ofertado acima da estimativa; pedido de renegociação', TRUE, 'Diligência ao setor de compras para validação de justificativa de sobrepreço ou renegociação')
    ON CONFLICT DO NOTHING;

    -- Workflow 2: Compra Direta por Dispensa de Licitação
    INSERT INTO atlas_workflows (id, tenant_id, codigo_processual, titulo, objetivo, publico_alvo, versao, ativo, nivel_acesso, codigo_ttdd)
    VALUES (
        wf_dispensa, 'nexus', 'ADM.DIR.002',
        'Contratação Direta por Dispensa em Razão do Valor (Art. 75, I e II da Lei 14.133/2021)',
        'Fluxo célere para contratações de pequeno valor ou pronta entrega dentro dos limites anuais legais.',
        'Todas as Secretarias e Fundos Municipais',
        1, TRUE, 'PUBLICO', '2.0.02.00.04'
    ) ON CONFLICT (tenant_id, codigo_processual, versao) DO NOTHING;

    INSERT INTO atlas_etapas (id, workflow_id, ordem, unidade_administrativa, nome_setor, atribuicoes_setor, prazo_sla_em_dias, manter_aberto_apos_remessa)
    VALUES
    (et_disp_1, wf_dispensa, 1, 'SEC/DEMANDANTE', 'Setor Demandante', 'Demonstração de necessidade, cotações prévias e Termo de Referência simplificado.', 3, FALSE),
    (et_disp_2, wf_dispensa, 2, 'SEFIN/COMPRAS', 'Compras e Controle de Limites de Dispensa', 'Verificação do limite anual da unidade gestora, certidões negativas e autorização de empenho.', 4, FALSE),
    (et_disp_3, wf_dispensa, 3, 'GAB/SECRETARIO', 'Ordenador de Despesas', 'Ato de ratificação da dispensa e emissão da Nota de Empenho.', 2, FALSE)
    ON CONFLICT (workflow_id, ordem) DO NOTHING;

    INSERT INTO atlas_etapa_documentos (etapa_id, nome_documento, obrigatorio, formato, tipo_assinatura, exige_conferencia_copia)
    VALUES
    (et_disp_1, 'Termo de Referência Simplificado', TRUE, 'NATO_DIGITAL', 'INDIVIDUAL', FALSE),
    (et_disp_1, 'Pesquisa de Preços (3 propostas ou notas fiscais)', TRUE, 'EXTERNO_DIGITALIZADO', 'INDIVIDUAL', TRUE),
    (et_disp_2, 'Relatório de Controle de Limite de Dispensa (Art. 75)', TRUE, 'NATO_DIGITAL', 'INDIVIDUAL', FALSE),
    (et_disp_2, 'Certidões Negativas do Fornecedor (CND, FGTS, Trabalhista)', TRUE, 'EXTERNO_DIGITALIZADO', 'INDIVIDUAL', TRUE),
    (et_disp_3, 'Ato de Ratificação de Dispensa de Licitação', TRUE, 'NATO_DIGITAL', 'INDIVIDUAL', FALSE)
    ON CONFLICT DO NOTHING;

    INSERT INTO atlas_etapa_transicoes (origem_etapa_id, destino_etapa_id, condicao_transicao, is_devolucao_diligencia, descricao_diligencia)
    VALUES
    (et_disp_1, et_disp_2, 'Cotações e TR anexados', FALSE, NULL),
    (et_disp_2, et_disp_3, 'Fornecedor regular e saldo de limite aprovado', FALSE, NULL),
    (et_disp_2, et_disp_1, 'Fornecedor com certidão irregular ou cotação incompleta', TRUE, 'Diligência para obtenção de novas cotações válidas')
    ON CONFLICT DO NOTHING;

    -- Workflow 3: Transferência de Material Permanente
    INSERT INTO atlas_workflows (id, tenant_id, codigo_processual, titulo, objetivo, publico_alvo, versao, ativo, nivel_acesso, codigo_ttdd)
    VALUES (
        wf_patrimonio, 'nexus', 'ADM.MAT.003',
        'Transferência e Carga Patrimonial de Bens Móveis Permanentes',
        'Procedimento para alteração de carga patrimonial entre secretarias, escolas ou unidades de saúde.',
        'Responsáveis patrimoniais de todas as unidades',
        1, TRUE, 'PUBLICO', '2.0.01.01.00'
    ) ON CONFLICT (tenant_id, codigo_processual, versao) DO NOTHING;

    INSERT INTO atlas_etapas (id, workflow_id, ordem, unidade_administrativa, nome_setor, atribuicoes_setor, prazo_sla_em_dias, manter_aberto_apos_remessa)
    VALUES
    (et_pat_1, wf_patrimonio, 1, 'UNIDADE/ORIGEM', 'Unidade Cedente', 'Conferência física do número de tombamento, estado do bem e expedição da nota de transferência.', 2, FALSE),
    (et_pat_2, wf_patrimonio, 2, 'UNIDADE/DESTINO', 'Unidade Receptora', 'Vistoria de recebimento e aceite da carga patrimonial no sistema contábil.', 2, FALSE)
    ON CONFLICT (workflow_id, ordem) DO NOTHING;

    INSERT INTO atlas_etapa_documentos (etapa_id, nome_documento, obrigatorio, formato, tipo_assinatura, exige_conferencia_copia)
    VALUES
    (et_pat_1, 'Nota de Transferência Patrimonial', TRUE, 'NATO_DIGITAL', 'CONJUNTA_MULTINIVEL', FALSE),
    (et_pat_2, 'Termo de Aceite e Responsabilidade Patrimonial', TRUE, 'NATO_DIGITAL', 'INDIVIDUAL', FALSE)
    ON CONFLICT DO NOTHING;

    INSERT INTO atlas_etapa_transicoes (origem_etapa_id, destino_etapa_id, condicao_transicao, is_devolucao_diligencia, descricao_diligencia)
    VALUES
    (et_pat_1, et_pat_2, 'Nota de transferência assinada pelo responsável cedente', FALSE, NULL),
    (et_pat_2, et_pat_1, 'Divergência física do bem ou plaqueta ilegível', TRUE, 'Diligência à unidade cedente para conferência da plaqueta de patrimônio')
    ON CONFLICT DO NOTHING;

END $$;
-- +goose StatementEnd


-- +goose Down
DROP TABLE IF EXISTS atlas_etapa_transicoes CASCADE;
DROP TABLE IF EXISTS atlas_etapa_documentos CASCADE;
DROP TABLE IF EXISTS atlas_etapas CASCADE;
DROP TABLE IF EXISTS atlas_workflows CASCADE;
DROP TABLE IF EXISTS atlas_classificacao_ttdd CASCADE;
