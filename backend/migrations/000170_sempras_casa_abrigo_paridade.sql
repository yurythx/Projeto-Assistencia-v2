-- +goose Up
-- SEMPRAS — Casa Abrigo com paridade ao sistema que a unidade usa hoje
-- (ADR 032). O que lá é <select> aqui é enum checado; o motivo do
-- desligamento ganha um tipo (as estatísticas contam por ele, não por
-- palavra no texto, como faz o 4.0).

-- 1. Acolhimento: o assistente de 6 etapas ------------------------------

-- motivos: 9 -> 19 (os antigos viram os equivalentes do 4.0)
ALTER TABLE sempras_casa_abrigo_acolhimentos DROP CONSTRAINT sempras_casa_abrigo_acolhimentos_motivos_check;
UPDATE sempras_casa_abrigo_acolhimentos SET motivos = (
    SELECT array_agg(DISTINCT novo ORDER BY novo) FROM unnest(motivos) AS m(antigo),
    LATERAL unnest(CASE antigo
        WHEN 'negligencia_abandono' THEN ARRAY['negligencia', 'abandono']
        WHEN 'orfandade_ausencia_responsavel' THEN ARRAY['orfandade']
        WHEN 'usuario_drogas_responsavel' THEN ARRAY['responsaveis_drogas']
        ELSE ARRAY[antigo] END) AS n(novo));
ALTER TABLE sempras_casa_abrigo_acolhimentos ADD CONSTRAINT sempras_casa_abrigo_acolhimentos_motivos_check CHECK (
    motivos <@ ARRAY['abandono', 'abuso_sexual', 'ameaca_morte_responsaveis', 'ausencia_responsaveis_doenca',
        'ausencia_responsaveis_prisao', 'conflito_familiar', 'entrega_voluntaria', 'exploracao_sexual', 'negligencia',
        'orfandade', 'responsaveis_drogas', 'responsaveis_deficiencia', 'responsaveis_transtorno_mental',
        'responsaveis_alcool', 'situacao_rua', 'trabalho_infantil', 'violencia_fisica', 'violencia_psicologica',
        'outro']::TEXT[]
    AND array_length(motivos, 1) > 0);

-- situação jurídica: texto livre -> uma das 10 do 4.0 (ou não informada)
ALTER TABLE sempras_casa_abrigo_acolhimentos DROP CONSTRAINT sempras_casa_abrigo_acolhimentos_situacao_juridica_check;
UPDATE sempras_casa_abrigo_acolhimentos SET situacao_juridica = '';
ALTER TABLE sempras_casa_abrigo_acolhimentos ADD CONSTRAINT sempras_casa_abrigo_acolhimentos_situacao_juridica_check CHECK (
    situacao_juridica IN ('', 'emergencial_conselho_tutelar', 'adocao_internacional', 'adocao_nacional', 'processo_guarda',
        'processo_tutela', 'poder_familiar_destituido', 'poder_familiar_suspenso', 'reintegracao_familia_extensa',
        'reintegracao_familia_natural', 'sem_previsao'));

-- histórico: booleano -> sim/não/não sei (o 4.0 pergunta "não sei")
ALTER TABLE sempras_casa_abrigo_acolhimentos
    ALTER COLUMN historico_acolhido_anteriormente DROP DEFAULT,
    ALTER COLUMN historico_acolhido_anteriormente TYPE TEXT USING CASE WHEN historico_acolhido_anteriormente THEN 'sim' ELSE '' END,
    ALTER COLUMN historico_acolhido_anteriormente SET DEFAULT '',
    ALTER COLUMN historico_irmaos_outra_unidade DROP DEFAULT,
    ALTER COLUMN historico_irmaos_outra_unidade TYPE TEXT USING CASE WHEN historico_irmaos_outra_unidade THEN 'sim' ELSE '' END,
    ALTER COLUMN historico_irmaos_outra_unidade SET DEFAULT '';

-- o endereço do responsável vira estruturado (CEP, logradouro...)
ALTER TABLE sempras_casa_abrigo_acolhimentos ADD COLUMN responsavel_logradouro TEXT NOT NULL DEFAULT ''
    CHECK (char_length(responsavel_logradouro) <= 200);
UPDATE sempras_casa_abrigo_acolhimentos SET responsavel_logradouro = left(responsavel_endereco, 200);
ALTER TABLE sempras_casa_abrigo_acolhimentos DROP COLUMN responsavel_endereco;

ALTER TABLE sempras_casa_abrigo_acolhimentos
    -- criança
    ADD COLUMN situacao_especial BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN pcd BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN apelido TEXT NOT NULL DEFAULT '' CHECK (char_length(apelido) <= 100),
    ADD COLUMN identidade_genero TEXT NOT NULL DEFAULT '' CHECK (identidade_genero IN ('', 'cisgenero', 'transgenero', 'nao_binario', 'outro')),
    ADD COLUMN orientacao_sexual TEXT NOT NULL DEFAULT '' CHECK (orientacao_sexual IN ('', 'heterossexual', 'homossexual', 'bissexual', 'pansexual', 'outro')),
    ADD COLUMN raca_cor TEXT NOT NULL DEFAULT '' CHECK (raca_cor IN ('', 'branca', 'preta', 'parda', 'amarela', 'indigena', 'quilombola')),
    ADD COLUMN filiacao_1 TEXT NOT NULL DEFAULT '' CHECK (char_length(filiacao_1) <= 200),
    ADD COLUMN filiacao_2 TEXT NOT NULL DEFAULT '' CHECK (char_length(filiacao_2) <= 200),
    -- responsável antes da medida
    ADD COLUMN responsavel_nao_possui BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN responsavel_nome_social TEXT NOT NULL DEFAULT '' CHECK (char_length(responsavel_nome_social) <= 200),
    ADD COLUMN responsavel_apelido TEXT NOT NULL DEFAULT '' CHECK (char_length(responsavel_apelido) <= 100),
    ADD COLUMN responsavel_cpf TEXT NOT NULL DEFAULT '' CHECK (responsavel_cpf = '' OR responsavel_cpf ~ '^[0-9]{11}$'),
    ADD COLUMN responsavel_nascimento DATE,
    ADD COLUMN responsavel_sexo TEXT NOT NULL DEFAULT '' CHECK (responsavel_sexo IN ('', 'masculino', 'feminino')),
    ADD COLUMN responsavel_cep TEXT NOT NULL DEFAULT '' CHECK (responsavel_cep = '' OR responsavel_cep ~ '^[0-9]{8}$'),
    ADD COLUMN responsavel_numero TEXT NOT NULL DEFAULT '' CHECK (char_length(responsavel_numero) <= 20),
    ADD COLUMN responsavel_complemento TEXT NOT NULL DEFAULT '' CHECK (char_length(responsavel_complemento) <= 100),
    ADD COLUMN responsavel_bairro TEXT NOT NULL DEFAULT '' CHECK (char_length(responsavel_bairro) <= 100),
    ADD COLUMN responsavel_cidade TEXT NOT NULL DEFAULT '' CHECK (char_length(responsavel_cidade) <= 100),
    ADD COLUMN responsavel_uf TEXT NOT NULL DEFAULT '' CHECK (responsavel_uf = '' OR responsavel_uf ~ '^[A-Z]{2}$'),
    ADD COLUMN responsavel_celular TEXT NOT NULL DEFAULT '' CHECK (char_length(responsavel_celular) <= 30),
    ADD COLUMN responsavel_email TEXT NOT NULL DEFAULT '' CHECK (char_length(responsavel_email) <= 200),
    -- acolhimento
    ADD COLUMN processo_judicial_2 TEXT NOT NULL DEFAULT '' CHECK (char_length(processo_judicial_2) <= 200),
    ADD COLUMN data_guia DATE,
    ADD COLUMN historico_irmaos_nesta_unidade TEXT NOT NULL DEFAULT '',
    ADD COLUMN outro_motivo TEXT NOT NULL DEFAULT '' CHECK (char_length(outro_motivo) <= 500),
    -- jurídico
    ADD COLUMN data_situacao_juridica DATE,
    ADD COLUMN cna TEXT NOT NULL DEFAULT '',
    ADD COLUMN cncad TEXT NOT NULL DEFAULT '',
    -- saúde
    ADD COLUMN laudo_medico TEXT NOT NULL DEFAULT '' CHECK (laudo_medico IN ('', 'sim', 'nao')),
    ADD COLUMN laudo_medico_descricao TEXT NOT NULL DEFAULT '' CHECK (char_length(laudo_medico_descricao) <= 1000),
    ADD COLUMN medicacao_continua TEXT NOT NULL DEFAULT '' CHECK (medicacao_continua IN ('', 'sim', 'nao')),
    ADD COLUMN dependencia_quimica TEXT NOT NULL DEFAULT '' CHECK (dependencia_quimica IN ('', 'sim', 'nao')),
    ADD COLUMN possui_transtorno TEXT NOT NULL DEFAULT '' CHECK (possui_transtorno IN ('', 'sim', 'nao')),
    ADD COLUMN transtorno_descricao TEXT NOT NULL DEFAULT '' CHECK (char_length(transtorno_descricao) <= 1000),
    -- contexto social
    ADD COLUMN estuda TEXT NOT NULL DEFAULT '' CHECK (estuda IN ('', 'sim', 'nao')),
    ADD COLUMN ameaca_morte TEXT NOT NULL DEFAULT '' CHECK (ameaca_morte IN ('', 'sim', 'nao')),
    ADD COLUMN recebe_visita TEXT NOT NULL DEFAULT '' CHECK (recebe_visita IN ('', 'sim', 'nao')),
    ADD COLUMN beneficio_governo TEXT NOT NULL DEFAULT '' CHECK (beneficio_governo IN ('', 'sim', 'nao')),
    ADD COLUMN beneficio_qual TEXT NOT NULL DEFAULT '' CHECK (char_length(beneficio_qual) <= 200),
    ADD COLUMN programa_social TEXT NOT NULL DEFAULT '' CHECK (programa_social IN ('', 'sim', 'nao')),
    ADD COLUMN relacao_trabalho TEXT NOT NULL DEFAULT '' CHECK (relacao_trabalho IN ('', 'jovem_aprendiz', 'meu_primeiro_trampo', 'formal', 'informal', 'nao')),
    -- desligamento (só pela ação própria)
    ADD COLUMN desligamento_tipo TEXT NOT NULL DEFAULT '' CHECK (desligamento_tipo IN ('', 'reintegracao_familia_natural',
        'reintegracao_familia_extensa', 'adocao', 'guarda', 'transferencia', 'maioridade', 'evasao', 'outro')),
    ADD COLUMN guia_desligamento TEXT NOT NULL DEFAULT '' CHECK (char_length(guia_desligamento) <= 200),
    ADD COLUMN data_guia_desligamento DATE;

-- os desligados antigos ficam como "outro" (não havia tipo)
UPDATE sempras_casa_abrigo_acolhimentos SET desligamento_tipo = 'outro' WHERE status = 'desligado' AND desligamento_tipo = '';

ALTER TABLE sempras_casa_abrigo_acolhimentos
    ADD CONSTRAINT sempras_casa_abrigo_acolhimentos_triestado_check CHECK (
        historico_acolhido_anteriormente IN ('', 'sim', 'nao', 'nao_sei')
        AND historico_irmaos_nesta_unidade IN ('', 'sim', 'nao', 'nao_sei')
        AND historico_irmaos_outra_unidade IN ('', 'sim', 'nao', 'nao_sei')
        AND cna IN ('', 'sim', 'nao', 'nao_sei')
        AND cncad IN ('', 'sim', 'nao', 'nao_sei')),
    -- quem está desligado tem o tipo do desligamento
    ADD CONSTRAINT sempras_casa_abrigo_acolhimentos_desligamento_check CHECK (status = 'acolhido' OR desligamento_tipo <> '');

-- R3: no máximo um acolhimento ativo por criança, em qualquer unidade
CREATE UNIQUE INDEX uq_sempras_casa_abrigo_ativo ON sempras_casa_abrigo_acolhimentos (cidadao_id) WHERE status = 'acolhido';

-- 2. PIA: as 6 abas -----------------------------------------------------

ALTER TABLE sempras_casa_abrigo_pia
    ADD COLUMN naturalidade TEXT NOT NULL DEFAULT '' CHECK (char_length(naturalidade) <= 100),
    ADD COLUMN nacionalidade TEXT NOT NULL DEFAULT '' CHECK (char_length(nacionalidade) <= 100),
    ADD COLUMN religiao TEXT NOT NULL DEFAULT '' CHECK (char_length(religiao) <= 100),
    ADD COLUMN doc_carteira_vacinacao BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN doc_certidao_nascimento BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN doc_rg_cpf BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN doc_cartao_sus BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN doc_nis BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN sinais_violencia_fisica BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN tratamento_odontologico BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN tratamento_medico_psicologico BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN deficiencias_restricoes TEXT NOT NULL DEFAULT '' CHECK (char_length(deficiencias_restricoes) <= 2000),
    ADD COLUMN periodo_escolar TEXT NOT NULL DEFAULT '' CHECK (periodo_escolar IN ('', 'matutino', 'vespertino', 'noturno', 'integral')),
    ADD COLUMN frequencia_escolar TEXT NOT NULL DEFAULT '' CHECK (char_length(frequencia_escolar) <= 200),
    ADD COLUMN aula_reforco BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN possui_beneficios BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN moradia_anterior TEXT NOT NULL DEFAULT '' CHECK (moradia_anterior IN ('', 'propria', 'alugada', 'cedida', 'situacao_rua')),
    ADD COLUMN observacoes_familia TEXT NOT NULL DEFAULT '' CHECK (char_length(observacoes_familia) <= 4000),
    ADD COLUMN observacoes_comportamentais TEXT NOT NULL DEFAULT '' CHECK (char_length(observacoes_comportamentais) <= 2000),
    ADD COLUMN intervencao_objetivos TEXT NOT NULL DEFAULT '' CHECK (char_length(intervencao_objetivos) <= 4000),
    ADD COLUMN intervencao_estrategias TEXT NOT NULL DEFAULT '' CHECK (char_length(intervencao_estrategias) <= 4000),
    ADD COLUMN intervencao_metas_prazos TEXT NOT NULL DEFAULT '' CHECK (char_length(intervencao_metas_prazos) <= 4000);

ALTER TABLE sempras_casa_abrigo_pia_metas
    ADD COLUMN esportes TEXT NOT NULL DEFAULT '' CHECK (char_length(esportes) <= 500),
    ADD COLUMN cultura TEXT NOT NULL DEFAULT '' CHECK (char_length(cultura) <= 500),
    ADD COLUMN lazer TEXT NOT NULL DEFAULT '' CHECK (char_length(lazer) <= 500);

-- 3. Visitas da unidade (o acolhido é opcional) -------------------------

ALTER TABLE sempras_casa_abrigo_visitantes ADD COLUMN unidade_id UUID REFERENCES unidades (id);
UPDATE sempras_casa_abrigo_visitantes v SET unidade_id = a.unidade_id
    FROM sempras_casa_abrigo_acolhimentos a WHERE a.id = v.acolhimento_id;
ALTER TABLE sempras_casa_abrigo_visitantes
    ALTER COLUMN unidade_id SET NOT NULL,
    ALTER COLUMN acolhimento_id DROP NOT NULL,
    ADD COLUMN tipo_visita TEXT NOT NULL DEFAULT 'acolhido' CHECK (tipo_visita IN ('acolhido', 'instituicao', 'doacao', 'voluntariado')),
    ADD CONSTRAINT sempras_casa_abrigo_visitantes_acolhido_check CHECK (tipo_visita <> 'acolhido' OR acolhimento_id IS NOT NULL);
CREATE INDEX idx_sempras_casa_abrigo_visitantes_unidade ON sempras_casa_abrigo_visitantes (unidade_id, data_visita DESC);

-- 4. Diário de bordo (só de inclusão) -----------------------------------

CREATE TABLE sempras_casa_abrigo_diario (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    unidade_id      UUID NOT NULL REFERENCES unidades (id),
    acolhimento_id  UUID REFERENCES sempras_casa_abrigo_acolhimentos (id),
    data_referencia DATE NOT NULL,
    texto           TEXT NOT NULL CHECK (char_length(texto) BETWEEN 3 AND 4000),
    autor_nome      TEXT NOT NULL DEFAULT '',
    registrado_por  UUID REFERENCES users (id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sempras_casa_abrigo_diario_unidade ON sempras_casa_abrigo_diario (unidade_id, data_referencia DESC);

-- 5. Relatórios escritos (o entregue não muda mais) ---------------------

CREATE TABLE sempras_casa_abrigo_relatorios (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    unidade_id     UUID NOT NULL REFERENCES unidades (id),
    acolhimento_id UUID REFERENCES sempras_casa_abrigo_acolhimentos (id),
    tipo           TEXT NOT NULL CHECK (tipo IN ('cinco_dias', 'sete_dias', 'quinzenal', 'mensal', 'bimestral', 'trimestral',
                       'anual', 'psicossocial', 'circunstanciado', 'informativo', 'outro')),
    tipo_outro     TEXT NOT NULL DEFAULT '' CHECK (char_length(tipo_outro) <= 200),
    status         TEXT NOT NULL DEFAULT 'em_elaboracao' CHECK (status IN ('em_elaboracao', 'revisao', 'aprovado', 'entregue')),
    data_entrega   DATE NOT NULL,
    conteudo       TEXT NOT NULL DEFAULT '' CHECK (char_length(conteudo) <= 100000),
    registrado_por UUID REFERENCES users (id),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    atualizado_em  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (tipo <> 'outro' OR tipo_outro <> '')
);
CREATE INDEX idx_sempras_casa_abrigo_relatorios_unidade ON sempras_casa_abrigo_relatorios (unidade_id, data_entrega);

-- 6. Tarefas da equipe (Kanban) -----------------------------------------

CREATE TABLE sempras_casa_abrigo_tarefas (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    unidade_id     UUID NOT NULL REFERENCES unidades (id),
    titulo         TEXT NOT NULL CHECK (char_length(titulo) BETWEEN 2 AND 200),
    descricao      TEXT NOT NULL DEFAULT '' CHECK (char_length(descricao) <= 4000),
    status         TEXT NOT NULL DEFAULT 'backlog' CHECK (status IN ('backlog', 'todo', 'doing', 'review', 'done')),
    prioridade     TEXT NOT NULL DEFAULT 'media' CHECK (prioridade IN ('baixa', 'media', 'alta', 'critica')),
    responsavel    TEXT NOT NULL DEFAULT '' CHECK (char_length(responsavel) <= 200),
    sprint         TEXT NOT NULL DEFAULT '' CHECK (char_length(sprint) <= 100),
    prazo          DATE,
    ordem          INTEGER NOT NULL DEFAULT 0,
    registrado_por UUID REFERENCES users (id),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    atualizado_em  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sempras_casa_abrigo_tarefas_unidade ON sempras_casa_abrigo_tarefas (unidade_id, status, ordem);

-- +goose Down
DROP TABLE IF EXISTS sempras_casa_abrigo_tarefas;
DROP TABLE IF EXISTS sempras_casa_abrigo_relatorios;
DROP TABLE IF EXISTS sempras_casa_abrigo_diario;

DROP INDEX IF EXISTS idx_sempras_casa_abrigo_visitantes_unidade;
DELETE FROM sempras_casa_abrigo_visitantes WHERE acolhimento_id IS NULL;
ALTER TABLE sempras_casa_abrigo_visitantes
    DROP CONSTRAINT sempras_casa_abrigo_visitantes_acolhido_check,
    DROP COLUMN tipo_visita,
    DROP COLUMN unidade_id,
    ALTER COLUMN acolhimento_id SET NOT NULL;

ALTER TABLE sempras_casa_abrigo_pia_metas DROP COLUMN esportes, DROP COLUMN cultura, DROP COLUMN lazer;
ALTER TABLE sempras_casa_abrigo_pia
    DROP COLUMN naturalidade, DROP COLUMN nacionalidade, DROP COLUMN religiao, DROP COLUMN doc_carteira_vacinacao,
    DROP COLUMN doc_certidao_nascimento, DROP COLUMN doc_rg_cpf, DROP COLUMN doc_cartao_sus, DROP COLUMN doc_nis,
    DROP COLUMN sinais_violencia_fisica, DROP COLUMN tratamento_odontologico, DROP COLUMN tratamento_medico_psicologico,
    DROP COLUMN deficiencias_restricoes, DROP COLUMN periodo_escolar, DROP COLUMN frequencia_escolar, DROP COLUMN aula_reforco,
    DROP COLUMN possui_beneficios, DROP COLUMN moradia_anterior, DROP COLUMN observacoes_familia,
    DROP COLUMN observacoes_comportamentais, DROP COLUMN intervencao_objetivos, DROP COLUMN intervencao_estrategias,
    DROP COLUMN intervencao_metas_prazos;

DROP INDEX IF EXISTS uq_sempras_casa_abrigo_ativo;
ALTER TABLE sempras_casa_abrigo_acolhimentos
    DROP CONSTRAINT sempras_casa_abrigo_acolhimentos_desligamento_check,
    DROP CONSTRAINT sempras_casa_abrigo_acolhimentos_triestado_check,
    DROP COLUMN situacao_especial, DROP COLUMN pcd, DROP COLUMN apelido, DROP COLUMN identidade_genero,
    DROP COLUMN orientacao_sexual, DROP COLUMN raca_cor, DROP COLUMN filiacao_1, DROP COLUMN filiacao_2,
    DROP COLUMN responsavel_nao_possui, DROP COLUMN responsavel_nome_social, DROP COLUMN responsavel_apelido,
    DROP COLUMN responsavel_cpf, DROP COLUMN responsavel_nascimento, DROP COLUMN responsavel_sexo, DROP COLUMN responsavel_cep,
    DROP COLUMN responsavel_numero, DROP COLUMN responsavel_complemento, DROP COLUMN responsavel_bairro,
    DROP COLUMN responsavel_cidade, DROP COLUMN responsavel_uf, DROP COLUMN responsavel_celular, DROP COLUMN responsavel_email,
    DROP COLUMN processo_judicial_2, DROP COLUMN data_guia, DROP COLUMN historico_irmaos_nesta_unidade, DROP COLUMN outro_motivo,
    DROP COLUMN data_situacao_juridica, DROP COLUMN cna, DROP COLUMN cncad, DROP COLUMN laudo_medico,
    DROP COLUMN laudo_medico_descricao, DROP COLUMN medicacao_continua, DROP COLUMN dependencia_quimica,
    DROP COLUMN possui_transtorno, DROP COLUMN transtorno_descricao, DROP COLUMN estuda, DROP COLUMN ameaca_morte,
    DROP COLUMN recebe_visita, DROP COLUMN beneficio_governo, DROP COLUMN beneficio_qual, DROP COLUMN programa_social,
    DROP COLUMN relacao_trabalho, DROP COLUMN desligamento_tipo, DROP COLUMN guia_desligamento, DROP COLUMN data_guia_desligamento;

ALTER TABLE sempras_casa_abrigo_acolhimentos ADD COLUMN responsavel_endereco TEXT NOT NULL DEFAULT ''
    CHECK (char_length(responsavel_endereco) <= 300);
UPDATE sempras_casa_abrigo_acolhimentos SET responsavel_endereco = responsavel_logradouro;
ALTER TABLE sempras_casa_abrigo_acolhimentos DROP COLUMN responsavel_logradouro;

ALTER TABLE sempras_casa_abrigo_acolhimentos
    ALTER COLUMN historico_acolhido_anteriormente DROP DEFAULT,
    ALTER COLUMN historico_acolhido_anteriormente TYPE BOOLEAN USING historico_acolhido_anteriormente = 'sim',
    ALTER COLUMN historico_acolhido_anteriormente SET DEFAULT false,
    ALTER COLUMN historico_irmaos_outra_unidade DROP DEFAULT,
    ALTER COLUMN historico_irmaos_outra_unidade TYPE BOOLEAN USING historico_irmaos_outra_unidade = 'sim',
    ALTER COLUMN historico_irmaos_outra_unidade SET DEFAULT false;

ALTER TABLE sempras_casa_abrigo_acolhimentos DROP CONSTRAINT sempras_casa_abrigo_acolhimentos_situacao_juridica_check;
ALTER TABLE sempras_casa_abrigo_acolhimentos ADD CONSTRAINT sempras_casa_abrigo_acolhimentos_situacao_juridica_check
    CHECK (char_length(situacao_juridica) <= 500);

ALTER TABLE sempras_casa_abrigo_acolhimentos DROP CONSTRAINT sempras_casa_abrigo_acolhimentos_motivos_check;
UPDATE sempras_casa_abrigo_acolhimentos SET motivos = (
    SELECT array_agg(DISTINCT novo ORDER BY novo) FROM unnest(motivos) AS m(atual),
    LATERAL unnest(CASE
        WHEN atual IN ('negligencia', 'abandono') THEN ARRAY['negligencia_abandono']
        WHEN atual = 'orfandade' THEN ARRAY['orfandade_ausencia_responsavel']
        WHEN atual = 'responsaveis_drogas' THEN ARRAY['usuario_drogas_responsavel']
        WHEN atual IN ('violencia_fisica', 'violencia_psicologica', 'abuso_sexual', 'situacao_rua', 'trabalho_infantil') THEN ARRAY[atual]
        ELSE ARRAY['outro'] END) AS n(novo));
ALTER TABLE sempras_casa_abrigo_acolhimentos ADD CONSTRAINT sempras_casa_abrigo_acolhimentos_motivos_check CHECK (
    motivos <@ ARRAY['negligencia_abandono', 'violencia_fisica', 'violencia_psicologica', 'abuso_sexual',
        'orfandade_ausencia_responsavel', 'usuario_drogas_responsavel', 'situacao_rua', 'trabalho_infantil', 'outro']::TEXT[]
    AND array_length(motivos, 1) > 0);
