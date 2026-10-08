// Package infrastructure implementa o repositório do Atlas.
package infrastructure

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/atlas/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
)

// Repository implementa domain.Repository.
type Repository struct{}

// NewRepository cria o repositório.
func NewRepository() *Repository { return &Repository{} }

var _ domain.Repository = (*Repository)(nil)

func wrap(err error) error {
	switch {
	case err == nil:
		return nil
	case database.IsNoRows(err):
		return domain.ErrNotFound
	case database.IsUniqueViolation(err):
		return domain.ErrDuplicate
	}
	return fmt.Errorf("atlas: %w", err)
}

// ---------------------------------------------------------------- TTDD

// ttddCols/ttddJoins: a série e a hierarquia oficial (LEFT JOIN — séries
// antigas podem não ter subfunção). Alias "c" para a classificação.
const ttddCols = `c.codigo, c.descritor, c.fase_corrente_anos, c.fase_corrente_condicao, c.fase_interm_anos, c.fase_interm_condicao,
	c.destinacao_final, c.observacoes, c.revogada_em, c.revogada_edicao, c.created_at,
	s.codigo, s.nome, s.recomendacao, f.codigo, f.nome, o.prefixo, o.nome, o.edicao_diario, o.data_publicacao, o.versao`

const ttddJoins = ` LEFT JOIN atlas_ttdd_subfuncoes s ON s.codigo = c.subfuncao_codigo
	LEFT JOIN atlas_ttdd_funcoes f ON f.codigo = s.funcao_codigo
	LEFT JOIN atlas_ttdd_orgaos o ON o.prefixo = f.orgao_prefixo`

// ttddDest devolve os destinos de Scan das colunas de ttddCols e a função
// que monta a hierarquia depois do Scan.
func ttddDest(c *domain.ClassificacaoTTDD) ([]any, func()) {
	var sCod, sNome, sRec, fCod, fNome, oPref, oNome, oEd, oVer *string
	var oData *time.Time
	dest := []any{&c.Codigo, &c.Descritor, &c.FaseCorrenteAnos, &c.FaseCorrenteCondicao, &c.FaseIntermAnos, &c.FaseIntermCondicao,
		&c.DestinacaoFinal, &c.Observacoes, &c.RevogadaEm, &c.RevogadaEdicao, &c.CreatedAt, &sCod, &sNome, &sRec, &fCod, &fNome, &oPref, &oNome, &oEd, &oData, &oVer}
	return dest, func() {
		if sCod == nil {
			return
		}
		c.Subfuncao = &domain.SubfuncaoTTDD{Codigo: *sCod, Nome: *sNome, Recomendacao: *sRec,
			Funcao: domain.FuncaoTTDD{Codigo: *fCod, Nome: *fNome,
				Orgao: domain.OrgaoTTDD{Prefixo: *oPref, Nome: *oNome, EdicaoDiario: *oEd, DataPublicacao: oData, Versao: *oVer}}}
	}
}

func scanTTDD(row pgx.Row, extra ...any) (domain.ClassificacaoTTDD, error) {
	var c domain.ClassificacaoTTDD
	dest, montar := ttddDest(&c)
	if err := row.Scan(append(dest, extra...)...); err != nil {
		return c, err
	}
	montar()
	return c, nil
}

// ordemTTDD ordena pelo código numericamente (2.0.01.00.10 depois de .09;
// 10.0 depois de 9.0); o sufixo "-2" (código repetido no documento) por último.
const ordemTTDD = `string_to_array(split_part(c.codigo, '-', 1), '.')::int[], c.codigo`

// ListTTDD consulta a TTDD em vigor (as revogadas só por código, em GetTTDD).
func (r *Repository) ListTTDD(ctx context.Context, db database.DBTX, f domain.FiltroTTDD, p pagination.Params) ([]domain.ClassificacaoTTDD, int64, error) {
	conds, args := []string{"c.revogada_em IS NULL"}, []any{}
	if f.Codigo != "" {
		// Prefixo hierárquico: "2.0" pega o órgão, "2.0.01" a função...
		args = append(args, f.Codigo)
		conds = append(conds, fmt.Sprintf("(c.codigo = $%d OR c.codigo LIKE $%d || '.%%')", len(args), len(args)))
	}
	if f.Query != "" {
		args = append(args, f.Query)
		conds = append(conds, fmt.Sprintf("(c.search @@ nexus_search_tsquery('portuguese', $%d) OR c.codigo LIKE $%d || '%%')", len(args), len(args)))
	}
	where := " WHERE " + strings.Join(conds, " AND ")
	var total int64
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM atlas_classificacao_ttdd c`+where, args...).Scan(&total); err != nil {
		return nil, 0, wrap(err)
	}
	args = append(args, p.Limit(), p.Offset())
	rows, err := db.Query(ctx, fmt.Sprintf(`SELECT %s FROM atlas_classificacao_ttdd c%s%s ORDER BY %s LIMIT $%d OFFSET $%d`,
		ttddCols, ttddJoins, where, ordemTTDD, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, wrap(err)
	}
	defer rows.Close()
	out := []domain.ClassificacaoTTDD{}
	for rows.Next() {
		c, err := scanTTDD(rows)
		if err != nil {
			return nil, 0, wrap(err)
		}
		out = append(out, c)
	}
	return out, total, wrap(rows.Err())
}

func (r *Repository) GetTTDD(ctx context.Context, db database.DBTX, codigo string) (domain.ClassificacaoTTDD, error) {
	c, err := scanTTDD(db.QueryRow(ctx, `SELECT `+ttddCols+` FROM atlas_classificacao_ttdd c`+ttddJoins+` WHERE c.codigo = $1`, codigo))
	if database.IsNoRows(err) {
		return c, domain.ErrTTDDNotFound
	}
	return c, wrap(err)
}

// EstruturaTTDD monta órgão > função > subfunção com a contagem de séries.
func (r *Repository) EstruturaTTDD(ctx context.Context, db database.DBTX) ([]domain.EstruturaTTDD, error) {
	rows, err := db.Query(ctx, `SELECT o.prefixo, o.nome, o.edicao_diario, o.data_publicacao, o.versao, f.codigo, f.nome, s.codigo, s.nome,
			(SELECT COUNT(*) FROM atlas_classificacao_ttdd c WHERE c.subfuncao_codigo = s.codigo AND c.revogada_em IS NULL)
		FROM atlas_ttdd_orgaos o JOIN atlas_ttdd_funcoes f ON f.orgao_prefixo = o.prefixo JOIN atlas_ttdd_subfuncoes s ON s.funcao_codigo = f.codigo
		ORDER BY string_to_array(s.codigo, '.')::int[]`)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	out := []domain.EstruturaTTDD{}
	for rows.Next() {
		var o domain.OrgaoTTDD
		var fCod, fNome string
		var sub domain.EstruturaSubfuncaoTTDD
		if err := rows.Scan(&o.Prefixo, &o.Nome, &o.EdicaoDiario, &o.DataPublicacao, &o.Versao, &fCod, &fNome, &sub.Codigo, &sub.Nome, &sub.Total); err != nil {
			return nil, wrap(err)
		}
		if len(out) == 0 || out[len(out)-1].Prefixo != o.Prefixo {
			out = append(out, domain.EstruturaTTDD{OrgaoTTDD: o, Funcoes: []domain.EstruturaFuncaoTTDD{}})
		}
		org := &out[len(out)-1]
		if len(org.Funcoes) == 0 || org.Funcoes[len(org.Funcoes)-1].Codigo != fCod {
			org.Funcoes = append(org.Funcoes, domain.EstruturaFuncaoTTDD{Codigo: fCod, Nome: fNome, Subfuncoes: []domain.EstruturaSubfuncaoTTDD{}})
		}
		fn := &org.Funcoes[len(org.Funcoes)-1]
		fn.Subfuncoes = append(fn.Subfuncoes, sub)
		fn.Total += sub.Total
		org.Total += sub.Total
	}
	return out, wrap(rows.Err())
}

// ProcedimentosPorOrgao: uma linha por órgão da TTDD, mesmo sem
// procedimento.
func (r *Repository) ProcedimentosPorOrgao(ctx context.Context, db database.DBTX) ([]domain.ProcedimentosOrgao, error) {
	out := []domain.ProcedimentosOrgao{}
	err := coletar(ctx, db, `SELECT o.prefixo, o.nome, count(w.id) FILTER (WHERE w.ativo),
		count(w.id) FILTER (WHERE w.situacao = 'EM_VALIDACAO' AND `+ultimaVersao+`),
		count(w.id) FILTER (WHERE w.situacao = 'RASCUNHO' AND `+ultimaVersao+`)
		FROM atlas_ttdd_orgaos o LEFT JOIN atlas_workflows w ON w.codigo_ttdd LIKE o.prefixo || '.%'
		GROUP BY o.prefixo, o.nome ORDER BY split_part(o.prefixo, '.', 1)::int`, func(rows pgx.Rows) error {
		var p domain.ProcedimentosOrgao
		err := rows.Scan(&p.Prefixo, &p.Nome, &p.Publicados, &p.EmValidacao, &p.Rascunhos)
		out = append(out, p)
		return err
	})
	return out, err
}

// CandidatosTTDD: basta um termo da pergunta casar (OR); a relevância real é
// medida no domínio (domain.RelevanciaTTDD).
func (r *Repository) CandidatosTTDD(ctx context.Context, db database.DBTX, pergunta string, limit int) ([]domain.ClassificacaoTTDD, error) {
	rows, err := db.Query(ctx, `WITH q AS (
			SELECT NULLIF(replace(plainto_tsquery('portuguese', nexus_unaccent($1))::text, ' & ', ' | '), '')::tsquery AS q
		)
		SELECT `+ttddCols+`, ts_rank(c.search, q.q) AS rank FROM q, atlas_classificacao_ttdd c`+ttddJoins+`
		WHERE c.search @@ q.q AND c.revogada_em IS NULL ORDER BY rank DESC, c.codigo LIMIT $2`, pergunta, limit)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	var out []domain.ClassificacaoTTDD
	for rows.Next() {
		var rank float32
		c, err := scanTTDD(rows, &rank)
		if err != nil {
			return nil, wrap(err)
		}
		out = append(out, c)
	}
	return out, wrap(rows.Err())
}

func (r *Repository) LockTTDD(ctx context.Context, db database.DBTX, codigo string) (domain.SituacaoTTDD, error) {
	var vigente bool
	err := db.QueryRow(ctx, `SELECT revogada_em IS NULL FROM atlas_classificacao_ttdd WHERE codigo = $1 FOR SHARE`, codigo).Scan(&vigente)
	switch {
	case database.IsNoRows(err):
		return domain.TTDDInexistente, nil
	case err != nil:
		return domain.TTDDInexistente, wrap(err)
	case vigente:
		return domain.TTDDVigente, nil
	}
	return domain.TTDDRevogada, nil
}

func (r *Repository) HistoricoTTDD(ctx context.Context, db database.DBTX, codigo string) ([]domain.HistoricoTTDD, error) {
	rows, err := db.Query(ctx, `SELECT evento, anterior, edicao_diario, registrado_em FROM atlas_ttdd_historico
		WHERE codigo = $1 ORDER BY registrado_em DESC, id DESC`, codigo)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	out := []domain.HistoricoTTDD{}
	for rows.Next() {
		var h domain.HistoricoTTDD
		if err := rows.Scan(&h.Evento, &h.Anterior, &h.EdicaoDiario, &h.RegistradoEm); err != nil {
			return nil, wrap(err)
		}
		out = append(out, h)
	}
	return out, wrap(rows.Err())
}

// ----------------------------------------------------------- Workflows

const wfCols = `w.id, w.codigo_processual, w.titulo, w.objetivo, w.publico_alvo, w.versao, w.ativo, w.situacao, w.nivel_acesso,
	COALESCE(w.hipotese_legal_restricao, ''), w.codigo_ttdd, w.created_by, w.created_at, w.updated_at,
	(SELECT COUNT(*) FROM atlas_etapas e WHERE e.workflow_id = w.id), ` + ttddCols

const wfFrom = ` FROM atlas_workflows w JOIN atlas_classificacao_ttdd c ON c.codigo = w.codigo_ttdd` + ttddJoins

func scanWorkflow(row pgx.Row, extra ...any) (domain.Workflow, error) {
	var w domain.Workflow
	c := &domain.ClassificacaoTTDD{}
	cdest, montar := ttddDest(c)
	dest := append([]any{&w.ID, &w.CodigoProcessual, &w.Titulo, &w.Objetivo, &w.PublicoAlvo, &w.Versao, &w.Ativo, &w.Situacao, &w.NivelAcesso,
		&w.HipoteseLegal, &w.CodigoTTDD, &w.CreatedBy, &w.CreatedAt, &w.UpdatedAt, &w.TotalEtapas}, cdest...)
	if err := row.Scan(append(dest, extra...)...); err != nil {
		return w, err
	}
	montar()
	w.Classificacao = c
	w.Etapas = []domain.Etapa{}
	return w, nil
}

// ultimaVersao: w é a versão mais recente do seu código.
const ultimaVersao = `NOT EXISTS (SELECT 1 FROM atlas_workflows o WHERE o.codigo_processual = w.codigo_processual AND o.versao > w.versao)`

func (r *Repository) List(ctx context.Context, db database.DBTX, f domain.Filter, p pagination.Params) ([]domain.Workflow, int64, error) {
	conds, args := []string{}, []any{}
	if !f.IncluirInativos {
		conds = append(conds, "w.ativo")
	} else {
		if f.Ultima {
			conds = append(conds, ultimaVersao)
		}
		if f.Situacao != "" {
			args = append(args, f.Situacao)
			conds = append(conds, fmt.Sprintf("w.situacao = $%d", len(args)))
		}
	}
	if f.PrefixoTTDD != "" {
		args = append(args, f.PrefixoTTDD)
		conds = append(conds, fmt.Sprintf("(w.codigo_ttdd = $%d OR w.codigo_ttdd LIKE $%d || '.%%')", len(args), len(args)))
	}
	if f.CodigoTTDD != "" {
		args = append(args, f.CodigoTTDD)
		conds = append(conds, fmt.Sprintf("w.codigo_ttdd = $%d", len(args)))
	}
	if f.CodigoProcessual != "" {
		args = append(args, strings.ToUpper(f.CodigoProcessual))
		conds = append(conds, fmt.Sprintf("w.codigo_processual = $%d", len(args)))
	}
	if f.Query != "" {
		args = append(args, f.Query)
		conds = append(conds, fmt.Sprintf("(w.search @@ nexus_search_tsquery('portuguese', $%d) OR w.codigo_processual ILIKE '%%' || $%d || '%%')", len(args), len(args)))
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}
	var total int64
	if err := db.QueryRow(ctx, `SELECT COUNT(*)`+wfFrom+where, args...).Scan(&total); err != nil {
		return nil, 0, wrap(err)
	}
	args = append(args, p.Limit(), p.Offset())
	rows, err := db.Query(ctx, fmt.Sprintf(`SELECT %s%s%s ORDER BY w.codigo_processual, w.versao DESC LIMIT $%d OFFSET $%d`,
		wfCols, wfFrom, where, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, wrap(err)
	}
	defer rows.Close()
	out := []domain.Workflow{}
	for rows.Next() {
		w, err := scanWorkflow(rows)
		if err != nil {
			return nil, 0, wrap(err)
		}
		out = append(out, w)
	}
	return out, total, wrap(rows.Err())
}

func (r *Repository) Get(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (domain.Workflow, error) {
	lock := ""
	if forUpdate {
		lock = " FOR UPDATE OF w"
	}
	w, err := scanWorkflow(db.QueryRow(ctx, `SELECT `+wfCols+wfFrom+` WHERE w.id = $1`+lock, id))
	if err != nil {
		return w, wrap(err)
	}
	full, err := r.loadEtapas(ctx, db, []domain.Workflow{w})
	if err != nil {
		return w, err
	}
	return full[0], nil
}

// loadEtapas carrega etapas, peças e transições de vários procedimentos em
// três consultas (sem N+1).
func (r *Repository) loadEtapas(ctx context.Context, db database.DBTX, wfs []domain.Workflow) ([]domain.Workflow, error) {
	if len(wfs) == 0 {
		return wfs, nil
	}
	ids := make([]uuid.UUID, len(wfs))
	idx := map[uuid.UUID]int{}
	for i, w := range wfs {
		ids[i] = w.ID
		idx[w.ID] = i
	}
	rows, err := db.Query(ctx, `SELECT id, workflow_id, ordem, unidade_administrativa, nome_setor, atribuicoes_setor,
		prazo_sla_em_dias, manter_aberto_apos_remessa FROM atlas_etapas WHERE workflow_id = ANY($1) ORDER BY workflow_id, ordem`, ids)
	if err != nil {
		return nil, wrap(err)
	}
	type pos struct{ wf, etapa int }
	etapaPos := map[uuid.UUID]pos{}
	ordemByEtapa := map[uuid.UUID]int{}
	var etapaIDs []uuid.UUID
	for rows.Next() {
		var e domain.Etapa
		var wfID uuid.UUID
		if err := rows.Scan(&e.ID, &wfID, &e.Ordem, &e.UnidadeAdministrativa, &e.NomeSetor, &e.AtribuicoesSetor,
			&e.PrazoSLAEmDias, &e.ManterAbertoAposRemessa); err != nil {
			rows.Close()
			return nil, wrap(err)
		}
		e.Documentos, e.Transicoes = []domain.EtapaDocumento{}, []domain.EtapaTransicao{}
		i := idx[wfID]
		wfs[i].Etapas = append(wfs[i].Etapas, e)
		etapaPos[e.ID] = pos{i, len(wfs[i].Etapas) - 1}
		ordemByEtapa[e.ID] = e.Ordem
		etapaIDs = append(etapaIDs, e.ID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, wrap(err)
	}
	if len(etapaIDs) == 0 {
		return wfs, nil
	}

	// Peça ligada a um modelo da biblioteca (ADR 024): nome e versão atual.
	rows, err = db.Query(ctx, `SELECT d.id, d.etapa_id, d.nome_documento, d.obrigatorio, d.formato, d.tipo_assinatura,
		d.exige_conferencia_copia, COALESCE(d.modelo_minuta_padrao_url, ''), d.modelo_id, m.nome, v.versao, v.arquivo_nome
		FROM atlas_etapa_documentos d
		LEFT JOIN atlas_modelos m ON m.id = d.modelo_id
		LEFT JOIN LATERAL (SELECT versao, arquivo_nome FROM atlas_modelo_versoes WHERE modelo_id = d.modelo_id
			ORDER BY versao DESC LIMIT 1) v ON TRUE
		WHERE d.etapa_id = ANY($1) ORDER BY d.nome_documento`, etapaIDs)
	if err != nil {
		return nil, wrap(err)
	}
	for rows.Next() {
		var d domain.EtapaDocumento
		var etapaID uuid.UUID
		var modeloNome, arquivo *string
		var versao *int
		if err := rows.Scan(&d.ID, &etapaID, &d.NomeDocumento, &d.Obrigatorio, &d.Formato, &d.TipoAssinatura,
			&d.ExigeConferenciaCopia, &d.ModeloMinutaPadraoURL, &d.ModeloID, &modeloNome, &versao, &arquivo); err != nil {
			rows.Close()
			return nil, wrap(err)
		}
		if d.ModeloID != nil && modeloNome != nil && versao != nil && arquivo != nil {
			d.Modelo = &domain.ModeloResumo{ID: *d.ModeloID, Nome: *modeloNome, Versao: *versao, ArquivoNome: *arquivo}
		}
		p := etapaPos[etapaID]
		wfs[p.wf].Etapas[p.etapa].Documentos = append(wfs[p.wf].Etapas[p.etapa].Documentos, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, wrap(err)
	}

	rows, err = db.Query(ctx, `SELECT id, origem_etapa_id, destino_etapa_id, condicao_transicao, is_devolucao_diligencia,
		COALESCE(descricao_diligencia, '') FROM atlas_etapa_transicoes WHERE origem_etapa_id = ANY($1) ORDER BY created_at, id`, etapaIDs)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	for rows.Next() {
		var t domain.EtapaTransicao
		var origem, destino uuid.UUID
		if err := rows.Scan(&t.ID, &origem, &destino, &t.CondicaoTransicao, &t.IsDevolucaoDiligencia, &t.DescricaoDiligencia); err != nil {
			return nil, wrap(err)
		}
		t.DestinoOrdem = ordemByEtapa[destino]
		p := etapaPos[origem]
		wfs[p.wf].Etapas[p.etapa].Transicoes = append(wfs[p.wf].Etapas[p.etapa].Transicoes, t)
	}
	return wfs, wrap(rows.Err())
}

func (r *Repository) Insert(ctx context.Context, db database.DBTX, w domain.Workflow) error {
	if _, err := db.Exec(ctx, `INSERT INTO atlas_workflows (id, codigo_processual, titulo, objetivo, publico_alvo, versao, ativo,
		nivel_acesso, hipotese_legal_restricao, codigo_ttdd, created_by, situacao)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9,''),$10,$11,$12)`,
		w.ID, w.CodigoProcessual, w.Titulo, w.Objetivo, w.PublicoAlvo, w.Versao, w.Ativo, string(w.NivelAcesso), w.HipoteseLegal,
		w.CodigoTTDD, w.CreatedBy, w.Situacao); err != nil {
		return wrap(err)
	}
	etapaID := map[int]uuid.UUID{}
	for _, e := range w.Etapas {
		etapaID[e.Ordem] = e.ID
		if _, err := db.Exec(ctx, `INSERT INTO atlas_etapas (id, workflow_id, ordem, unidade_administrativa, nome_setor,
			atribuicoes_setor, prazo_sla_em_dias, manter_aberto_apos_remessa) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			e.ID, w.ID, e.Ordem, e.UnidadeAdministrativa, e.NomeSetor, e.AtribuicoesSetor, e.PrazoSLAEmDias, e.ManterAbertoAposRemessa); err != nil {
			return wrap(err)
		}
		for _, d := range e.Documentos {
			if _, err := db.Exec(ctx, `INSERT INTO atlas_etapa_documentos (id, etapa_id, nome_documento, obrigatorio, formato,
				tipo_assinatura, exige_conferencia_copia, modelo_minuta_padrao_url, modelo_id) VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF($8,''),$9)`,
				d.ID, e.ID, d.NomeDocumento, d.Obrigatorio, string(d.Formato), string(d.TipoAssinatura), d.ExigeConferenciaCopia,
				d.ModeloMinutaPadraoURL, d.ModeloID); err != nil {
				return wrap(err)
			}
		}
	}
	// Transições depois de todas as etapas: o destino pode vir adiante.
	for _, e := range w.Etapas {
		for _, t := range e.Transicoes {
			if _, err := db.Exec(ctx, `INSERT INTO atlas_etapa_transicoes (id, origem_etapa_id, destino_etapa_id, condicao_transicao,
				is_devolucao_diligencia, descricao_diligencia) VALUES ($1,$2,$3,$4,$5,NULLIF($6,''))`,
				t.ID, e.ID, etapaID[t.DestinoOrdem], t.CondicaoTransicao, t.IsDevolucaoDiligencia, t.DescricaoDiligencia); err != nil {
				return wrap(err)
			}
		}
	}
	return nil
}

func (r *Repository) SetAtivo(ctx context.Context, db database.DBTX, id uuid.UUID, ativo bool) error {
	tag, err := db.Exec(ctx, `UPDATE atlas_workflows SET ativo = $2 WHERE id = $1`, id, ativo)
	if err != nil {
		return wrap(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repository) MaxVersao(ctx context.Context, db database.DBTX, codigo string) (int, error) {
	// FOR UPDATE nas versões: duas "novas versões" simultâneas do mesmo
	// procedimento se serializam (a segunda vê a versão da primeira).
	rows, err := db.Query(ctx, `SELECT versao FROM atlas_workflows WHERE codigo_processual = $1 FOR UPDATE`, codigo)
	if err != nil {
		return 0, wrap(err)
	}
	defer rows.Close()
	maior := 0
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return 0, wrap(err)
		}
		maior = max(maior, v)
	}
	return maior, wrap(rows.Err())
}

func (r *Repository) DesativarVersoes(ctx context.Context, db database.DBTX, codigo string, exceto uuid.UUID) ([]uuid.UUID, error) {
	rows, err := db.Query(ctx, `UPDATE atlas_workflows SET ativo = FALSE
		WHERE codigo_processual = $1 AND id <> $2 AND ativo RETURNING id`, codigo, exceto)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, wrap(err)
		}
		ids = append(ids, id)
	}
	return ids, wrap(rows.Err())
}

func (r *Repository) Search(ctx context.Context, db database.DBTX, query string, limit int) ([]domain.Workflow, []float64, error) {
	rows, err := db.Query(ctx, `SELECT `+wfCols+`, ts_rank(w.search, q) AS rank`+wfFrom+`, nexus_search_tsquery('portuguese', $1) q
		WHERE w.ativo AND w.search @@ q ORDER BY rank DESC LIMIT $2`, query, limit)
	if err != nil {
		return nil, nil, wrap(err)
	}
	defer rows.Close()
	var out []domain.Workflow
	var ranks []float64
	for rows.Next() {
		var rank float32
		w, err := scanWorkflow(rows, &rank)
		if err != nil {
			return nil, nil, wrap(err)
		}
		out = append(out, w)
		ranks = append(ranks, float64(rank))
	}
	return out, ranks, wrap(rows.Err())
}

// Candidatos: basta UM termo da pergunta casar (OR) — em linguagem natural
// a pergunta tem palavras que o procedimento não usa. A relevância real é
// medida depois, no domínio (domain.RelevanciaProcedimento), sobre o procedimento
// completo; aqui só se recorta o universo pelo índice.
func (r *Repository) Candidatos(ctx context.Context, db database.DBTX, pergunta string, limit int) ([]domain.Workflow, error) {
	rows, err := db.Query(ctx, `WITH q AS (
			SELECT NULLIF(replace(plainto_tsquery('portuguese', nexus_unaccent($1))::text, ' & ', ' | '), '')::tsquery AS q
		), hits AS (
			SELECT w.id, ts_rank(w.search, q.q) AS rank FROM atlas_workflows w, q WHERE w.ativo AND w.search @@ q.q
			UNION ALL
			SELECT e.workflow_id, ts_rank(e.search, q.q) * 0.5 FROM atlas_etapas e, q WHERE e.search @@ q.q
			UNION ALL
			SELECT e.workflow_id, ts_rank(d.search, q.q) * 0.5 FROM atlas_etapa_documentos d
				JOIN atlas_etapas e ON e.id = d.etapa_id, q WHERE d.search @@ q.q
		)
		SELECT `+wfCols+wfFrom+` JOIN (SELECT id, SUM(rank) AS rank FROM hits GROUP BY id) h ON h.id = w.id
		WHERE w.ativo ORDER BY h.rank DESC, w.codigo_processual LIMIT $2`, pergunta, limit)
	if err != nil {
		return nil, wrap(err)
	}
	var out []domain.Workflow
	for rows.Next() {
		w, err := scanWorkflow(rows)
		if err != nil {
			rows.Close()
			return nil, wrap(err)
		}
		out = append(out, w)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, wrap(err)
	}
	return r.loadEtapas(ctx, db, out)
}
