-- Carga da TTDD oficial: compara os dados de deploy/ttdd/ttdd.sql (tabelas
-- temporárias carga_*) com o banco, mostra o impacto e grava (ADR 019).
--
--   make ttdd-impacto  — mostra o impacto e desfaz tudo (nada é gravado);
--   make ttdd-aplicar  — mostra o impacto e grava.
--
-- Regras:
--   * órgãos (com a edição do Diário Oficial), funções e subfunções:
--     inseridos ou atualizados;
--   * série da carga: NOVA, ALTERADA (o gatilho guarda os valores anteriores
--     em atlas_ttdd_historico), RESTABELECIDA (estava revogada) ou
--     INALTERADA;
--   * série que está no banco, de um órgão presente na carga, e não veio
--     nela: REVOGADA na data e edição da publicação do órgão — nunca
--     apagada (documentos e procedimentos continuam apontando para ela).
\set ON_ERROR_STOP on
\if :{?aplicar}
\else
  \set aplicar 0
\endif
\pset footer off

BEGIN;

CREATE FUNCTION pg_temp.prazo(anos INT, condicao TEXT, corrente BOOLEAN) RETURNS TEXT LANGUAGE sql IMMUTABLE AS $$
  SELECT CASE
    WHEN anos = 1 THEN '1 ano'
    WHEN anos IS NOT NULL THEN anos || ' anos'
    WHEN condicao <> '' THEN condicao
    WHEN corrente THEN 'não informado'
    ELSE 'não há' END
$$;

CREATE FUNCTION pg_temp.resumo(anos_c INT, cond_c TEXT, anos_i INT, cond_i TEXT, dest TEXT) RETURNS TEXT LANGUAGE sql IMMUTABLE AS $$
  SELECT pg_temp.prazo(anos_c, cond_c, TRUE) || ' + ' || pg_temp.prazo(anos_i, cond_i, FALSE) || ' → ' ||
    COALESCE(CASE dest WHEN 'GUARDA_PERMANENTE' THEN 'guarda permanente' WHEN 'ELIMINACAO' THEN 'eliminação' END, 'não definida')
$$;

-- Situação de cada série ANTES de gravar. Série já revogada que continua
-- fora da carga não entra (nada muda).
CREATE TEMP TABLE carga_impacto AS
SELECT COALESCE(n.codigo, a.codigo) AS codigo,
    CASE
        WHEN a.codigo IS NULL THEN 'NOVA'
        WHEN n.codigo IS NULL THEN 'REVOGADA'
        WHEN a.revogada_em IS NOT NULL THEN 'RESTABELECIDA'
        WHEN (a.descritor, a.fase_corrente_anos, a.fase_corrente_condicao, a.fase_interm_anos, a.fase_interm_condicao,
              a.destinacao_final::TEXT, a.observacoes)
             IS DISTINCT FROM
             (n.descritor, n.fase_corrente_anos, n.fase_corrente_condicao, n.fase_interm_anos, n.fase_interm_condicao,
              n.destinacao_final, n.observacoes) THEN 'ALTERADA'
        ELSE 'INALTERADA'
    END AS situacao,
    COALESCE(n.descritor, a.descritor) AS descritor,
    CASE WHEN a.codigo IS NOT NULL THEN pg_temp.resumo(a.fase_corrente_anos, a.fase_corrente_condicao, a.fase_interm_anos,
        a.fase_interm_condicao, a.destinacao_final) END AS antes,
    CASE WHEN n.codigo IS NOT NULL THEN pg_temp.resumo(n.fase_corrente_anos, n.fase_corrente_condicao, n.fase_interm_anos,
        n.fase_interm_condicao, n.destinacao_final) END AS depois,
    a.descritor IS DISTINCT FROM n.descritor AND a.codigo IS NOT NULL AND n.codigo IS NOT NULL AS descritor_mudou
FROM carga_series n
FULL JOIN (
    SELECT c.* FROM atlas_classificacao_ttdd c
    WHERE split_part(c.codigo, '.', 1) || '.0' IN (SELECT prefixo FROM carga_orgaos)
) a ON a.codigo = n.codigo
WHERE n.codigo IS NOT NULL OR a.revogada_em IS NULL;

\echo
\echo '=== Impacto da carga da TTDD ==='
SELECT situacao AS "Situação", count(*) AS "Séries" FROM carga_impacto GROUP BY situacao
ORDER BY array_position(ARRAY['NOVA', 'ALTERADA', 'RESTABELECIDA', 'REVOGADA', 'INALTERADA'], situacao);

\echo '--- Séries alteradas (prazos: corrente + intermediária → destinação) ---'
SELECT codigo AS "Código", left(descritor, 60) AS "Série", antes AS "Antes", depois AS "Depois",
    CASE WHEN descritor_mudou THEN 'sim' ELSE '' END AS "Descritor mudou"
FROM carga_impacto WHERE situacao = 'ALTERADA'
ORDER BY string_to_array(split_part(codigo, '-', 1), '.')::INT[], codigo;

\echo '--- Séries revogadas (saem da TTDD em vigor; continuam consultáveis pelo código) ---'
SELECT codigo AS "Código", left(descritor, 70) AS "Série", antes AS "Prazos" FROM carga_impacto WHERE situacao = 'REVOGADA'
ORDER BY string_to_array(split_part(codigo, '-', 1), '.')::INT[], codigo;

\echo '--- Séries restabelecidas ---'
SELECT codigo AS "Código", left(descritor, 70) AS "Série" FROM carga_impacto WHERE situacao = 'RESTABELECIDA'
ORDER BY string_to_array(split_part(codigo, '-', 1), '.')::INT[], codigo;

\echo '--- Procedimentos afetados (série alterada ou revogada): revisar e, se preciso, criar nova versão ---'
SELECT w.codigo_processual AS "Procedimento", w.versao AS "Versão", CASE WHEN w.ativo THEN 'ativo' ELSE 'inativo' END AS "Estado",
    w.codigo_ttdd AS "Série", i.situacao AS "Mudança"
FROM atlas_workflows w JOIN carga_impacto i ON i.codigo = w.codigo_ttdd
WHERE i.situacao IN ('ALTERADA', 'REVOGADA')
ORDER BY w.ativo DESC, w.codigo_processual, w.versao;

-- Gravação (dentro da transação; a simulação desfaz no fim).
INSERT INTO atlas_ttdd_orgaos (prefixo, nome, edicao_diario, data_publicacao, versao)
SELECT prefixo, nome, edicao_diario, data_publicacao, versao FROM carga_orgaos
ON CONFLICT (prefixo) DO UPDATE SET nome = EXCLUDED.nome, edicao_diario = EXCLUDED.edicao_diario,
    data_publicacao = EXCLUDED.data_publicacao, versao = EXCLUDED.versao;

INSERT INTO atlas_ttdd_funcoes (codigo, orgao_prefixo, nome)
SELECT codigo, orgao_prefixo, nome FROM carga_funcoes
ON CONFLICT (codigo) DO UPDATE SET nome = EXCLUDED.nome;

INSERT INTO atlas_ttdd_subfuncoes (codigo, funcao_codigo, nome, recomendacao)
SELECT codigo, funcao_codigo, nome, recomendacao FROM carga_subfuncoes
ON CONFLICT (codigo) DO UPDATE SET nome = EXCLUDED.nome, recomendacao = EXCLUDED.recomendacao;

INSERT INTO atlas_classificacao_ttdd (codigo, subfuncao_codigo, descritor, fase_corrente_anos, fase_corrente_condicao,
    fase_interm_anos, fase_interm_condicao, destinacao_final, observacoes)
SELECT codigo, subfuncao_codigo, descritor, fase_corrente_anos, fase_corrente_condicao, fase_interm_anos,
    fase_interm_condicao, destinacao_final, observacoes FROM carga_series
ON CONFLICT (codigo) DO UPDATE SET subfuncao_codigo = EXCLUDED.subfuncao_codigo, descritor = EXCLUDED.descritor,
    fase_corrente_anos = EXCLUDED.fase_corrente_anos, fase_corrente_condicao = EXCLUDED.fase_corrente_condicao,
    fase_interm_anos = EXCLUDED.fase_interm_anos, fase_interm_condicao = EXCLUDED.fase_interm_condicao,
    destinacao_final = EXCLUDED.destinacao_final, observacoes = EXCLUDED.observacoes,
    revogada_em = NULL, revogada_edicao = '';

UPDATE atlas_classificacao_ttdd c
SET revogada_em = COALESCE(o.data_publicacao, CURRENT_DATE), revogada_edicao = o.edicao_diario
FROM carga_impacto i, carga_orgaos o
WHERE i.codigo = c.codigo AND i.situacao = 'REVOGADA' AND o.prefixo = split_part(c.codigo, '.', 1) || '.0';

\if :aplicar
COMMIT;
\echo 'TTDD gravada.'
\else
ROLLBACK;
\echo 'Simulação: nada foi gravado. Para gravar: make ttdd-aplicar'
\endif
