package infrastructure

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-nexus/internal/modules/atlas/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
)

// serieDeProcesso reconhece, pelo início do descritor, a série que é um
// processo ou um pedido (não um registro, relatório ou livro).
const serieDeProcesso = `'^(processos?|requerimento|pedido|solicita|concess|licen|alvar|certid|inscri|cadastr|isen|parcelamento|recurso|aposentadoria|pens)'`

// Cobertura monta o painel em cinco consultas (órgãos, contagens, peças sem
// modelo, modelos sem uso).
func (r *Repository) Cobertura(ctx context.Context, db database.DBTX) (domain.Cobertura, error) {
	out := domain.Cobertura{Orgaos: []domain.CoberturaOrgao{}, PecasSemModelo: []domain.PecaSemModelo{}, ModelosSemUso: []domain.ModeloSemUso{},
		Lacunas: []domain.LacunaSubfuncao{}}
	err := coletar(ctx, db, `SELECT o.prefixo, o.nome, count(c.codigo),
		count(c.codigo) FILTER (WHERE EXISTS (SELECT 1 FROM atlas_workflows w WHERE w.ativo AND w.codigo_ttdd = c.codigo)),
		count(c.codigo) FILTER (WHERE EXISTS (SELECT 1 FROM atlas_ttdd_modelos tm JOIN atlas_modelos m ON m.id = tm.modelo_id AND m.ativo
			WHERE tm.codigo = c.codigo))
		FROM atlas_ttdd_orgaos o
		JOIN atlas_ttdd_funcoes f ON f.orgao_prefixo = o.prefixo
		JOIN atlas_ttdd_subfuncoes s ON s.funcao_codigo = f.codigo
		JOIN atlas_classificacao_ttdd c ON c.subfuncao_codigo = s.codigo AND c.revogada_em IS NULL
		GROUP BY o.prefixo, o.nome ORDER BY split_part(o.prefixo, '.', 1)::int`, func(rows pgx.Rows) error {
		var o domain.CoberturaOrgao
		err := rows.Scan(&o.Prefixo, &o.Nome, &o.Series, &o.SeriesComProcedimento, &o.SeriesComModelo)
		out.Orgaos = append(out.Orgaos, o)
		return err
	})
	if err != nil {
		return out, err
	}
	err = coletar(ctx, db, `SELECT w.id, w.codigo_processual, w.titulo, e.ordem, d.nome_documento
		FROM atlas_etapa_documentos d JOIN atlas_etapas e ON e.id = d.etapa_id JOIN atlas_workflows w ON w.id = e.workflow_id
		WHERE w.ativo AND NOT EXISTS (SELECT 1 FROM atlas_modelos m WHERE m.id = d.modelo_id AND m.ativo)
			AND NOT EXISTS (SELECT 1 FROM atlas_ttdd_modelos tm JOIN atlas_modelos m ON m.id = tm.modelo_id AND m.ativo
				WHERE tm.codigo = w.codigo_ttdd)
		ORDER BY w.codigo_processual, e.ordem, d.nome_documento`, func(rows pgx.Rows) error {
		var p domain.PecaSemModelo
		err := rows.Scan(&p.WorkflowID, &p.CodigoProcessual, &p.Titulo, &p.Etapa, &p.Peca)
		out.PecasSemModelo = append(out.PecasSemModelo, p)
		return err
	})
	if err != nil {
		return out, err
	}
	err = coletar(ctx, db, `SELECT m.id, m.nome FROM atlas_modelos m
		WHERE m.ativo
			AND NOT EXISTS (SELECT 1 FROM atlas_etapa_documentos d JOIN atlas_etapas e ON e.id = d.etapa_id
				JOIN atlas_workflows w ON w.id = e.workflow_id AND w.ativo WHERE d.modelo_id = m.id)
			AND NOT EXISTS (SELECT 1 FROM atlas_ttdd_modelos tm WHERE tm.modelo_id = m.id)
		ORDER BY lower(m.nome)`, func(rows pgx.Rows) error {
		var m domain.ModeloSemUso
		err := rows.Scan(&m.ID, &m.Nome)
		out.ModelosSemUso = append(out.ModelosSemUso, m)
		return err
	})
	if err != nil {
		return out, err
	}
	// Lacunas: séries cujo descritor indica um processo (heurística pelo
	// início: "Processo", "Requerimento", "Solicitação", "Licença"…) em
	// subfunções sem nenhum procedimento, nem rascunho.
	err = coletar(ctx, db, `SELECT s.codigo, s.nome, o.nome, count(*),
		(array_agg(c.descritor ORDER BY c.codigo))[1:3]
		FROM atlas_classificacao_ttdd c
		JOIN atlas_ttdd_subfuncoes s ON s.codigo = c.subfuncao_codigo
		JOIN atlas_ttdd_funcoes f ON f.codigo = s.funcao_codigo
		JOIN atlas_ttdd_orgaos o ON o.prefixo = f.orgao_prefixo
		WHERE c.revogada_em IS NULL AND c.descritor ~* `+serieDeProcesso+`
			AND NOT EXISTS (SELECT 1 FROM atlas_workflows w JOIN atlas_classificacao_ttdd x ON x.codigo = w.codigo_ttdd
				WHERE x.subfuncao_codigo = s.codigo)
		GROUP BY s.codigo, s.nome, o.nome, f.orgao_prefixo
		ORDER BY split_part(f.orgao_prefixo, '.', 1)::int, s.codigo`, func(rows pgx.Rows) error {
		var l domain.LacunaSubfuncao
		err := rows.Scan(&l.Codigo, &l.Nome, &l.Orgao, &l.Series, &l.Exemplos)
		out.Lacunas = append(out.Lacunas, l)
		return err
	})
	if err != nil {
		return out, err
	}
	var procedimentos, pecas, modelos, rascunhos, emValidacao int
	if err := db.QueryRow(ctx, `SELECT (SELECT count(*) FROM atlas_workflows WHERE ativo),
		(SELECT count(*) FROM atlas_etapa_documentos d JOIN atlas_etapas e ON e.id = d.etapa_id
			JOIN atlas_workflows w ON w.id = e.workflow_id WHERE w.ativo),
		(SELECT count(*) FROM atlas_modelos WHERE ativo),
		(SELECT count(*) FROM atlas_workflows w WHERE w.situacao = 'RASCUNHO' AND `+ultimaVersao+`),
		(SELECT count(*) FROM atlas_workflows w WHERE w.situacao = 'EM_VALIDACAO' AND `+ultimaVersao+`)`).Scan(&procedimentos, &pecas, &modelos, &rascunhos, &emValidacao); err != nil {
		return out, wrap(err)
	}
	out.Somar(procedimentos, pecas, modelos)
	out.Totais.Rascunhos, out.Totais.EmValidacao = rascunhos, emValidacao
	return out, nil
}

// coletar roda a consulta e passa cada linha a ler.
func coletar(ctx context.Context, db database.DBTX, sql string, ler func(pgx.Rows) error) error {
	rows, err := db.Query(ctx, sql)
	if err != nil {
		return wrap(err)
	}
	defer rows.Close()
	for rows.Next() {
		if err := ler(rows); err != nil {
			return wrap(err)
		}
	}
	return wrap(rows.Err())
}

// Organograma: uma linha por subfunção da TTDD, com as séries vigentes, as
// séries de processo e os procedimentos (pela série em que se enquadram). O
// domínio soma as funções e os órgãos.
func (r *Repository) Organograma(ctx context.Context, db database.DBTX) ([]domain.OrgaoOrganograma, error) {
	out := []domain.OrgaoOrganograma{}
	err := coletar(ctx, db, `SELECT org.prefixo, org.nome, f.codigo, f.nome, s.codigo, s.nome,
		(SELECT count(*) FROM atlas_classificacao_ttdd c WHERE c.subfuncao_codigo = s.codigo AND c.revogada_em IS NULL),
		(SELECT count(*) FROM atlas_classificacao_ttdd c WHERE c.subfuncao_codigo = s.codigo AND c.revogada_em IS NULL
			AND c.descritor ~* `+serieDeProcesso+`),
		count(w.id) FILTER (WHERE w.ativo),
		count(w.id) FILTER (WHERE w.situacao = 'EM_VALIDACAO' AND `+ultimaVersao+`),
		count(w.id) FILTER (WHERE w.situacao = 'RASCUNHO' AND `+ultimaVersao+`),
		count(DISTINCT w.codigo_processual)
		FROM atlas_ttdd_orgaos org
		JOIN atlas_ttdd_funcoes f ON f.orgao_prefixo = org.prefixo
		JOIN atlas_ttdd_subfuncoes s ON s.funcao_codigo = f.codigo
		LEFT JOIN atlas_classificacao_ttdd x ON x.subfuncao_codigo = s.codigo
		LEFT JOIN atlas_workflows w ON w.codigo_ttdd = x.codigo
		GROUP BY org.prefixo, org.nome, f.codigo, f.nome, s.codigo, s.nome
		ORDER BY string_to_array(s.codigo, '.')::int[]`, func(rows pgx.Rows) error {
		var prefixo, orgao, fCod, fNome string
		var sub domain.SubfuncaoOrganograma
		if err := rows.Scan(&prefixo, &orgao, &fCod, &fNome, &sub.Codigo, &sub.Nome, &sub.Series, &sub.SeriesDeProcesso,
			&sub.Publicados, &sub.EmValidacao, &sub.Rascunhos, &sub.Procedimentos); err != nil {
			return err
		}
		if len(out) == 0 || out[len(out)-1].Prefixo != prefixo {
			out = append(out, domain.OrgaoOrganograma{Prefixo: prefixo, Nome: orgao, Funcoes: []domain.FuncaoOrganograma{}})
		}
		o := &out[len(out)-1]
		if len(o.Funcoes) == 0 || o.Funcoes[len(o.Funcoes)-1].Codigo != fCod {
			o.Funcoes = append(o.Funcoes, domain.FuncaoOrganograma{Codigo: fCod, Nome: fNome, Subfuncoes: []domain.SubfuncaoOrganograma{}})
		}
		f := &o.Funcoes[len(o.Funcoes)-1]
		f.Subfuncoes = append(f.Subfuncoes, sub)
		return nil
	})
	return out, err
}
