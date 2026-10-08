package infrastructure

import (
	"context"
	"time"

	"github.com/yurythx/projeto-nexus/internal/modules/atlas/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
)

// Carga da TTDD pela tela (ADR 022): as mesmas regras de
// deploy/ttdd/carga.sql (make ttdd-impacto / ttdd-aplicar) — mantenha os
// dois em sincronia. Tudo numa transação: as tabelas temporárias somem no
// fim dela (ON COMMIT DROP), e a simulação é desfeita pelo serviço.

var tabelasCarga = []string{
	`CREATE TEMP TABLE carga_orgaos (prefixo TEXT PRIMARY KEY, nome TEXT NOT NULL, edicao_diario TEXT NOT NULL,
		data_publicacao DATE, versao TEXT NOT NULL) ON COMMIT DROP`,
	`CREATE TEMP TABLE carga_funcoes (codigo TEXT PRIMARY KEY, orgao_prefixo TEXT NOT NULL, nome TEXT NOT NULL) ON COMMIT DROP`,
	`CREATE TEMP TABLE carga_subfuncoes (codigo TEXT PRIMARY KEY, funcao_codigo TEXT NOT NULL, nome TEXT NOT NULL,
		recomendacao TEXT NOT NULL) ON COMMIT DROP`,
	`CREATE TEMP TABLE carga_series (codigo TEXT PRIMARY KEY, subfuncao_codigo TEXT NOT NULL, descritor TEXT NOT NULL,
		fase_corrente_anos INT, fase_corrente_condicao TEXT NOT NULL, fase_interm_anos INT, fase_interm_condicao TEXT NOT NULL,
		destinacao_final TEXT, observacoes TEXT NOT NULL) ON COMMIT DROP`,
}

// Situação de cada série ANTES de gravar; série já revogada que continua
// fora da carga não entra. Só órgãos presentes na carga podem perder séries.
const impactoCarga = `CREATE TEMP TABLE carga_impacto ON COMMIT DROP AS
SELECT COALESCE(n.codigo, a.codigo) AS codigo,
	CASE
		WHEN a.codigo IS NULL THEN 'NOVA'
		WHEN n.codigo IS NULL THEN 'REVOGADA'
		WHEN a.revogada_em IS NOT NULL THEN 'RESTABELECIDA'
		WHEN (a.descritor, a.fase_corrente_anos, a.fase_corrente_condicao, a.fase_interm_anos, a.fase_interm_condicao,
			a.destinacao_final::TEXT, a.observacoes) IS DISTINCT FROM (n.descritor, n.fase_corrente_anos, n.fase_corrente_condicao,
			n.fase_interm_anos, n.fase_interm_condicao, n.destinacao_final, n.observacoes) THEN 'ALTERADA'
		ELSE 'INALTERADA'
	END AS situacao,
	a.codigo IS NOT NULL AS tem_antes, a.descritor AS a_desc, a.fase_corrente_anos AS a_ca, a.fase_corrente_condicao AS a_cc,
	a.fase_interm_anos AS a_ia, a.fase_interm_condicao AS a_ic, a.destinacao_final::TEXT AS a_dest, a.observacoes AS a_obs,
	n.codigo IS NOT NULL AS tem_depois, n.descritor AS n_desc, n.fase_corrente_anos AS n_ca, n.fase_corrente_condicao AS n_cc,
	n.fase_interm_anos AS n_ia, n.fase_interm_condicao AS n_ic, n.destinacao_final AS n_dest, n.observacoes AS n_obs
FROM carga_series n
FULL JOIN (
	SELECT c.* FROM atlas_classificacao_ttdd c
	WHERE split_part(c.codigo, '.', 1) || '.0' IN (SELECT prefixo FROM carga_orgaos)
) a ON a.codigo = n.codigo
WHERE n.codigo IS NOT NULL OR a.revogada_em IS NULL`

var gravacaoCarga = []string{
	`INSERT INTO atlas_ttdd_orgaos (prefixo, nome, edicao_diario, data_publicacao, versao)
	SELECT prefixo, nome, edicao_diario, data_publicacao, versao FROM carga_orgaos
	ON CONFLICT (prefixo) DO UPDATE SET nome = EXCLUDED.nome, edicao_diario = EXCLUDED.edicao_diario,
		data_publicacao = EXCLUDED.data_publicacao, versao = EXCLUDED.versao`,
	`INSERT INTO atlas_ttdd_funcoes (codigo, orgao_prefixo, nome) SELECT codigo, orgao_prefixo, nome FROM carga_funcoes
	ON CONFLICT (codigo) DO UPDATE SET nome = EXCLUDED.nome`,
	`INSERT INTO atlas_ttdd_subfuncoes (codigo, funcao_codigo, nome, recomendacao)
	SELECT codigo, funcao_codigo, nome, recomendacao FROM carga_subfuncoes
	ON CONFLICT (codigo) DO UPDATE SET nome = EXCLUDED.nome, recomendacao = EXCLUDED.recomendacao`,
	`INSERT INTO atlas_classificacao_ttdd (codigo, subfuncao_codigo, descritor, fase_corrente_anos, fase_corrente_condicao,
		fase_interm_anos, fase_interm_condicao, destinacao_final, observacoes)
	SELECT codigo, subfuncao_codigo, descritor, fase_corrente_anos, fase_corrente_condicao, fase_interm_anos,
		fase_interm_condicao, destinacao_final, observacoes FROM carga_series
	ON CONFLICT (codigo) DO UPDATE SET subfuncao_codigo = EXCLUDED.subfuncao_codigo, descritor = EXCLUDED.descritor,
		fase_corrente_anos = EXCLUDED.fase_corrente_anos, fase_corrente_condicao = EXCLUDED.fase_corrente_condicao,
		fase_interm_anos = EXCLUDED.fase_interm_anos, fase_interm_condicao = EXCLUDED.fase_interm_condicao,
		destinacao_final = EXCLUDED.destinacao_final, observacoes = EXCLUDED.observacoes, revogada_em = NULL, revogada_edicao = ''`,
	`UPDATE atlas_classificacao_ttdd c
	SET revogada_em = COALESCE(o.data_publicacao, CURRENT_DATE), revogada_edicao = o.edicao_diario
	FROM carga_impacto i, carga_orgaos o
	WHERE i.codigo = c.codigo AND i.situacao = 'REVOGADA' AND o.prefixo = split_part(c.codigo, '.', 1) || '.0'`,
}

// insercoes devolve os INSERTs (unnest de arrays: uma instrução por tabela).
func insercoes(c domain.CargaTTDD) []struct {
	sql  string
	args []any
} {
	var (
		oPref, oNome, oEd, oVer []string
		oData                   []*time.Time
		fCod, fOrg, fNome       []string
		sCod, sFun, sNome, sRec []string
		eCod, eSub, eDesc       []string
		eCC, eIC, eObs          []string
		eCA, eIA                []*int
		eDest                   []*string
	)
	for _, o := range c.Orgaos {
		oPref, oNome, oEd, oVer, oData = append(oPref, o.Prefixo), append(oNome, o.Nome), append(oEd, o.EdicaoDiario),
			append(oVer, o.Versao), append(oData, o.DataPublicacao)
	}
	for _, f := range c.Funcoes {
		fCod, fOrg, fNome = append(fCod, f.Codigo), append(fOrg, pai(f.Codigo, 2)), append(fNome, f.Nome)
	}
	for _, s := range c.Subfuncoes {
		sCod, sFun, sNome, sRec = append(sCod, s.Codigo), append(sFun, pai(s.Codigo, 3)), append(sNome, s.Nome), append(sRec, s.Recomendacao)
	}
	for _, s := range c.Series {
		var dest *string
		if s.DestinacaoFinal != nil {
			d := string(*s.DestinacaoFinal)
			dest = &d
		}
		eCod, eSub, eDesc = append(eCod, s.Codigo), append(eSub, s.Subfuncao), append(eDesc, s.Descritor)
		eCA, eCC, eIA, eIC = append(eCA, s.FaseCorrenteAnos), append(eCC, s.FaseCorrenteCondicao), append(eIA, s.FaseIntermAnos), append(eIC, s.FaseIntermCondicao)
		eDest, eObs = append(eDest, dest), append(eObs, s.Observacoes)
	}
	return []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO carga_orgaos SELECT * FROM unnest($1::text[], $2::text[], $3::text[], $4::date[], $5::text[])`,
			[]any{oPref, oNome, oEd, oData, oVer}},
		{`INSERT INTO carga_funcoes SELECT * FROM unnest($1::text[], $2::text[], $3::text[])`, []any{fCod, fOrg, fNome}},
		{`INSERT INTO carga_subfuncoes SELECT * FROM unnest($1::text[], $2::text[], $3::text[], $4::text[])`, []any{sCod, sFun, sNome, sRec}},
		{`INSERT INTO carga_series SELECT * FROM unnest($1::text[], $2::text[], $3::text[], $4::int[], $5::text[], $6::int[],
			$7::text[], $8::text[], $9::text[])`, []any{eCod, eSub, eDesc, eCA, eCC, eIA, eIC, eDest, eObs}},
	}
}

// pai devolve os n primeiros segmentos do código.
func pai(codigo string, n int) string {
	partes := 0
	for i, r := range codigo {
		if r == '.' {
			partes++
			if partes == n {
				return codigo[:i]
			}
		}
	}
	return codigo
}

// prazos monta os valores de um lado (antes/depois) da comparação.
func prazos(tem bool, desc, cc, ic, dest, obs *string, ca, ia *int) *domain.PrazosTTDD {
	if !tem {
		return nil
	}
	p := &domain.PrazosTTDD{Descritor: *desc, FaseCorrenteAnos: ca, FaseCorrenteCondicao: *cc, FaseIntermAnos: ia,
		FaseIntermCondicao: *ic, Observacoes: *obs}
	if dest != nil {
		d := domain.DestinacaoFinal(*dest)
		p.DestinacaoFinal = &d
	}
	return p
}

func (r *Repository) CargaTTDD(ctx context.Context, db database.DBTX, c domain.CargaTTDD, aplicar bool) (domain.ImpactoCarga, error) {
	out := domain.ImpactoCarga{Totais: map[string]int{}, Series: []domain.SerieImpacto{}, Procedimentos: []domain.ProcedimentoAfetado{}}
	for _, sql := range tabelasCarga {
		if _, err := db.Exec(ctx, sql); err != nil {
			return out, wrap(err)
		}
	}
	for _, ins := range insercoes(c) {
		if _, err := db.Exec(ctx, ins.sql, ins.args...); err != nil {
			return out, wrap(err)
		}
	}
	if _, err := db.Exec(ctx, impactoCarga); err != nil {
		return out, wrap(err)
	}

	rows, err := db.Query(ctx, `SELECT codigo, situacao, tem_antes, a_desc, a_ca, a_cc, a_ia, a_ic, a_dest, a_obs,
		tem_depois, n_desc, n_ca, n_cc, n_ia, n_ic, n_dest, n_obs FROM carga_impacto
		ORDER BY string_to_array(split_part(codigo, '-', 1), '.')::int[], codigo`)
	if err != nil {
		return out, wrap(err)
	}
	for rows.Next() {
		var (
			s                            domain.SerieImpacto
			temA, temD                   bool
			aDesc, aCC, aIC, aDest, aObs *string
			nDesc, nCC, nIC, nDest, nObs *string
			aCA, aIA, nCA, nIA           *int
		)
		if err := rows.Scan(&s.Codigo, &s.Situacao, &temA, &aDesc, &aCA, &aCC, &aIA, &aIC, &aDest, &aObs,
			&temD, &nDesc, &nCA, &nCC, &nIA, &nIC, &nDest, &nObs); err != nil {
			rows.Close()
			return out, wrap(err)
		}
		out.Totais[s.Situacao]++
		if s.Situacao == domain.SituacaoInalterada {
			continue
		}
		s.Antes, s.Depois = prazos(temA, aDesc, aCC, aIC, aDest, aObs, aCA, aIA), prazos(temD, nDesc, nCC, nIC, nDest, nObs, nCA, nIA)
		if lado := s.Depois; lado != nil {
			s.Descritor = lado.Descritor
		} else {
			s.Descritor = s.Antes.Descritor
		}
		out.Series = append(out.Series, s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, wrap(err)
	}

	rows, err = db.Query(ctx, `SELECT w.id, w.codigo_processual, w.versao, w.ativo, w.codigo_ttdd, i.situacao
		FROM atlas_workflows w JOIN carga_impacto i ON i.codigo = w.codigo_ttdd
		WHERE i.situacao IN ('ALTERADA', 'REVOGADA') ORDER BY w.ativo DESC, w.codigo_processual, w.versao`)
	if err != nil {
		return out, wrap(err)
	}
	for rows.Next() {
		var p domain.ProcedimentoAfetado
		if err := rows.Scan(&p.ID, &p.CodigoProcessual, &p.Versao, &p.Ativo, &p.CodigoTTDD, &p.Situacao); err != nil {
			rows.Close()
			return out, wrap(err)
		}
		out.Procedimentos = append(out.Procedimentos, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, wrap(err)
	}

	if !aplicar {
		return out, nil
	}
	for _, sql := range gravacaoCarga {
		if _, err := db.Exec(ctx, sql); err != nil {
			return out, wrap(err)
		}
	}
	return out, nil
}
