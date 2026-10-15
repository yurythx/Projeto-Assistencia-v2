// Package infrastructure implementa a persistência do SEMPRAS em
// PostgreSQL (queries parametrizadas via pgx). As listagens recebem a área
// de quem consulta e a aplicam no SQL (ADR 013: CoverageOf).
package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/domain"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/database"
)

// Repository implementa domain.Repository.
type Repository struct{}

// NewRepository cria o repositório.
func NewRepository() *Repository { return &Repository{} }

func wrap(op string, err error) error { return fmt.Errorf("sempras.%s: %w", op, err) }

// areaSQL filtra pela área (u = unidades): $a tudo, $u unidades, $e entidades.
func areaSQL(all, unidades, entidades int) string {
	return fmt.Sprintf(`($%d OR u.id = ANY($%d::uuid[]) OR u.entidade_id = ANY($%d::uuid[]))`, all, unidades, entidades)
}

func areaArgs(a domain.Area) []any {
	return []any{a.All, ids(a.Unidades), ids(a.Entidades)}
}

func ids(in []uuid.UUID) []string {
	out := make([]string, 0, len(in))
	for _, id := range in {
		out = append(out, id.String())
	}
	return out
}

// collect lê todas as linhas com scan; nunca devolve nil.
func collect[T any](rows pgx.Rows, op string, scan func(pgx.Row) (T, error)) ([]T, error) {
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, wrap(op, err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, wrap(op, err)
	}
	return out, nil
}

// ---------------------------------------------------------------- unidades

const unidadeCols = `u.id, u.entidade_id, u.nome, u.sigla, (u.ativo AND e.ativo)`
const unidadeFrom = ` FROM unidades u JOIN entidades e ON e.id = u.entidade_id`

func scanUnidade(row pgx.Row) (domain.Unidade, error) {
	var u domain.Unidade
	err := row.Scan(&u.ID, &u.EntidadeID, &u.Nome, &u.Sigla, &u.Ativa)
	return u, err
}

// GetUnidade busca a unidade (ativa = ela e a entidade ativas).
func (r *Repository) GetUnidade(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Unidade, error) {
	u, err := scanUnidade(db.QueryRow(ctx, `SELECT `+unidadeCols+unidadeFrom+` WHERE u.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Unidade{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Unidade{}, wrap("GetUnidade", err)
	}
	return u, nil
}

// ListUnidades lista as unidades ativas da área. Com acesso a tudo, as da
// entidade de slug entidadeSlug (a SEMPRAS) — e não as da prefeitura toda.
func (r *Repository) ListUnidades(ctx context.Context, db database.DBTX, area domain.Area, entidadeSlug string) ([]domain.Unidade, error) {
	rows, err := db.Query(ctx, `SELECT `+unidadeCols+unidadeFrom+`
		WHERE u.ativo AND e.ativo
		  AND CASE WHEN $1 THEN e.slug = $4 ELSE (u.id = ANY($2::uuid[]) OR u.entidade_id = ANY($3::uuid[])) END
		ORDER BY u.nome`, append(areaArgs(area), entidadeSlug)...)
	if err != nil {
		return nil, wrap("ListUnidades", err)
	}
	return collect(rows, "ListUnidades", scanUnidade)
}

// ----------------------------------------------------------------- munícipe

const cidadaoCols = `c.id, c.nome, c.nome_social, COALESCE(c.cpf, ''), COALESCE(c.nis, ''),
	COALESCE(to_char(c.data_nascimento, 'YYYY-MM-DD'), ''), c.nome_mae, c.telefone, c.endereco, c.anonimo, c.created_at, c.updated_at`

func scanCidadao(row pgx.Row) (domain.Cidadao, error) {
	var c domain.Cidadao
	err := row.Scan(&c.ID, &c.Nome, &c.NomeSocial, &c.CPF, &c.NIS, &c.DataNascimento, &c.NomeMae, &c.Telefone,
		&c.Endereco, &c.Anonimo, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

func documentoDuplicado(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" &&
		(pgErr.ConstraintName == "uq_sempras_cidadaos_cpf" || pgErr.ConstraintName == "uq_sempras_cidadaos_nis")
}

// InsertCidadao grava o cadastro (CPF/NIS repetido = ErrDocumentoDuplicado).
func (r *Repository) InsertCidadao(ctx context.Context, db database.DBTX, c domain.Cidadao, autor uuid.UUID) error {
	_, err := db.Exec(ctx, `INSERT INTO sempras_cidadaos
		(id, nome, nome_social, cpf, nis, data_nascimento, nome_mae, telefone, endereco, anonimo, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, '')::date, $7, $8, $9, $10, $11, $12, $12)`,
		c.ID, c.Nome, c.NomeSocial, c.CPF, c.NIS, c.DataNascimento, c.NomeMae, c.Telefone, c.Endereco, c.Anonimo, autor, c.CreatedAt)
	if documentoDuplicado(err) {
		return domain.ErrDocumentoDuplicado
	}
	if err != nil {
		return wrap("InsertCidadao", err)
	}
	return nil
}

// UpdateCidadao regrava os campos editáveis.
func (r *Repository) UpdateCidadao(ctx context.Context, db database.DBTX, c domain.Cidadao) error {
	_, err := db.Exec(ctx, `UPDATE sempras_cidadaos SET nome = $2, nome_social = $3, cpf = NULLIF($4, ''),
		nis = NULLIF($5, ''), data_nascimento = NULLIF($6, '')::date, nome_mae = $7, telefone = $8, endereco = $9, anonimo = $10
		WHERE id = $1`,
		c.ID, c.Nome, c.NomeSocial, c.CPF, c.NIS, c.DataNascimento, c.NomeMae, c.Telefone, c.Endereco, c.Anonimo)
	if documentoDuplicado(err) {
		return domain.ErrDocumentoDuplicado
	}
	if err != nil {
		return wrap("UpdateCidadao", err)
	}
	return nil
}

// GetCidadao busca o cadastro (forUpdate trava a linha na transação).
func (r *Repository) GetCidadao(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (domain.Cidadao, error) {
	q := `SELECT ` + cidadaoCols + ` FROM sempras_cidadaos c WHERE c.id = $1`
	if forUpdate {
		q += ` FOR UPDATE`
	}
	c, err := scanCidadao(db.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Cidadao{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Cidadao{}, wrap("GetCidadao", err)
	}
	return c, nil
}

// BuscarPorDocumento procura o CPF ou o NIS exatos.
func (r *Repository) BuscarPorDocumento(ctx context.Context, db database.DBTX, digitos string) ([]domain.Cidadao, error) {
	rows, err := db.Query(ctx, `SELECT `+cidadaoCols+` FROM sempras_cidadaos c WHERE c.cpf = $1 OR c.nis = $1 ORDER BY c.nome`, digitos)
	if err != nil {
		return nil, wrap("BuscarPorDocumento", err)
	}
	return collect(rows, "BuscarPorDocumento", scanCidadao)
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// BuscarPorNome procura o trecho no nome ou no nome social, sem acento.
func (r *Repository) BuscarPorNome(ctx context.Context, db database.DBTX, nome string, limit int) ([]domain.Cidadao, error) {
	rows, err := db.Query(ctx, `SELECT `+cidadaoCols+` FROM sempras_cidadaos c
		WHERE lower(nexus_unaccent(c.nome || ' ' || c.nome_social)) LIKE '%' || lower(nexus_unaccent($1)) || '%'
		ORDER BY c.nome, c.id LIMIT $2`, likeEscaper.Replace(nome), limit)
	if err != nil {
		return nil, wrap("BuscarPorNome", err)
	}
	return collect(rows, "BuscarPorNome", scanCidadao)
}

// HistoricoNaArea conta os atendimentos (menos os cancelados) de cada
// munícipe nas unidades da área — o que aconteceu fora dela não aparece.
func (r *Repository) HistoricoNaArea(ctx context.Context, db database.DBTX, cidadaos []uuid.UUID, area domain.Area) (map[uuid.UUID]int, error) {
	rows, err := db.Query(ctx, `SELECT a.cidadao_id, count(*) FROM sempras_atendimentos a
		JOIN unidades u ON u.id = a.unidade_id
		WHERE a.cidadao_id = ANY($4::uuid[]) AND a.status <> 'cancelado' AND `+areaSQL(1, 2, 3)+`
		GROUP BY a.cidadao_id`, append(areaArgs(area), ids(cidadaos))...)
	if err != nil {
		return nil, wrap("HistoricoNaArea", err)
	}
	type par struct {
		id uuid.UUID
		n  int
	}
	pares, err := collect(rows, "HistoricoNaArea", func(row pgx.Row) (par, error) {
		var p par
		err := row.Scan(&p.id, &p.n)
		return p, err
	})
	if err != nil {
		return nil, err
	}
	out := map[uuid.UUID]int{}
	for _, p := range pares {
		out[p.id] = p.n
	}
	return out, nil
}

// ------------------------------------------------------- fila e atendimento

const atendCols = `a.id, a.cidadao_id, a.unidade_id, to_char(a.dia, 'YYYY-MM-DD'), a.senha, a.tipo, a.prioridade,
	a.status, a.local, a.incluido_por, a.profissional_id, a.created_at, a.chamado_em, a.iniciado_em, a.encerrado_em`

func scanAtend(row pgx.Row, extra ...any) (domain.Atendimento, error) {
	var a domain.Atendimento
	err := row.Scan(append([]any{&a.ID, &a.CidadaoID, &a.UnidadeID, &a.Dia, &a.Senha, &a.Tipo, &a.Prioridade,
		&a.Status, &a.Local, &a.IncluidoPor, &a.ProfissionalID, &a.CreatedAt, &a.ChamadoEm, &a.IniciadoEm,
		&a.EncerradoEm}, extra...)...)
	return a, err
}

func scanAtendimento(row pgx.Row) (domain.Atendimento, error) { return scanAtend(row) }

// ProximaSenha incrementa o contador da unidade no dia (trava a linha até o
// fim da transação — senhas nunca repetem).
func (r *Repository) ProximaSenha(ctx context.Context, db database.DBTX, unidade uuid.UUID, dia string) (int, error) {
	var n int
	err := db.QueryRow(ctx, `INSERT INTO sempras_senhas (unidade_id, dia, ultima) VALUES ($1, $2::date, 1)
		ON CONFLICT (unidade_id, dia) DO UPDATE SET ultima = sempras_senhas.ultima + 1
		RETURNING ultima`, unidade, dia).Scan(&n)
	if err != nil {
		return 0, wrap("ProximaSenha", err)
	}
	return n, nil
}

// InsertAtendimento grava a entrada na fila (já na fila hoje = ErrJaNaFila).
func (r *Repository) InsertAtendimento(ctx context.Context, db database.DBTX, a domain.Atendimento) error {
	_, err := db.Exec(ctx, `INSERT INTO sempras_atendimentos
		(id, cidadao_id, unidade_id, dia, senha, tipo, prioridade, status, local, incluido_por, created_at)
		VALUES ($1, $2, $3, $4::date, $5, $6, $7, $8, $9, $10, $11)`,
		a.ID, a.CidadaoID, a.UnidadeID, a.Dia, a.Senha, a.Tipo, a.Prioridade, a.Status, a.Local, a.IncluidoPor, a.CreatedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.ConstraintName == "uq_sempras_atendimentos_ativo" {
		return domain.ErrJaNaFila
	}
	if err != nil {
		return wrap("InsertAtendimento", err)
	}
	return nil
}

// GetAtendimento busca a entrada (forUpdate trava a linha na transação).
func (r *Repository) GetAtendimento(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (domain.Atendimento, error) {
	q := `SELECT ` + atendCols + ` FROM sempras_atendimentos a WHERE a.id = $1`
	if forUpdate {
		q += ` FOR UPDATE`
	}
	a, err := scanAtend(db.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Atendimento{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Atendimento{}, wrap("GetAtendimento", err)
	}
	return a, nil
}

// ProximoAguardando trava o próximo da fila (prioridade, depois chegada).
// SKIP LOCKED: dois técnicos chamando juntos recebem pessoas diferentes.
func (r *Repository) ProximoAguardando(ctx context.Context, db database.DBTX, unidade uuid.UUID, dia string, tipos []string) (domain.Atendimento, error) {
	a, err := scanAtend(db.QueryRow(ctx, `SELECT `+atendCols+` FROM sempras_atendimentos a
		WHERE a.unidade_id = $1 AND a.dia = $2::date AND a.status = 'aguardando' AND a.tipo = ANY($3::text[])
		ORDER BY a.peso DESC, a.created_at, a.senha
		LIMIT 1 FOR UPDATE SKIP LOCKED`, unidade, dia, tipos))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Atendimento{}, domain.ErrFilaVazia
	}
	if err != nil {
		return domain.Atendimento{}, wrap("ProximoAguardando", err)
	}
	return a, nil
}

// UpdateAtendimento grava o novo estado.
func (r *Repository) UpdateAtendimento(ctx context.Context, db database.DBTX, a domain.Atendimento) error {
	if _, err := db.Exec(ctx, `UPDATE sempras_atendimentos SET status = $2, local = $3, profissional_id = $4,
		chamado_em = $5, iniciado_em = $6, encerrado_em = $7 WHERE id = $1`,
		a.ID, a.Status, a.Local, a.ProfissionalID, a.ChamadoEm, a.IniciadoEm, a.EncerradoEm); err != nil {
		return wrap("UpdateAtendimento", err)
	}
	return nil
}

// Fila lista as entradas do dia: em atendimento, chamados, aguardando (por
// prioridade e chegada) e, por último, as encerradas.
func (r *Repository) Fila(ctx context.Context, db database.DBTX, unidade uuid.UUID, dia string) ([]domain.FilaItem, error) {
	rows, err := db.Query(ctx, `SELECT `+atendCols+`, c.nome, c.nome_social
		FROM sempras_atendimentos a JOIN sempras_cidadaos c ON c.id = a.cidadao_id
		WHERE a.unidade_id = $1 AND a.dia = $2::date
		ORDER BY CASE a.status WHEN 'em_atendimento' THEN 0 WHEN 'chamado' THEN 1 WHEN 'aguardando' THEN 2 ELSE 3 END,
		         a.peso DESC, a.created_at, a.senha`, unidade, dia)
	if err != nil {
		return nil, wrap("Fila", err)
	}
	return collect(rows, "Fila", func(row pgx.Row) (domain.FilaItem, error) {
		var c domain.Cidadao
		a, err := scanAtend(row, &c.Nome, &c.NomeSocial)
		return domain.FilaItem{Atendimento: a, SenhaExibida: a.SenhaExibida(), Nome: c.NomeExibicao()}, err
	})
}

// AtendimentosNaArea lista os atendimentos do munícipe nas unidades da área.
func (r *Repository) AtendimentosNaArea(ctx context.Context, db database.DBTX, cidadao uuid.UUID, area domain.Area) ([]domain.Atendimento, error) {
	rows, err := db.Query(ctx, `SELECT `+atendCols+` FROM sempras_atendimentos a JOIN unidades u ON u.id = a.unidade_id
		WHERE a.cidadao_id = $4 AND `+areaSQL(1, 2, 3)+`
		ORDER BY a.created_at DESC LIMIT 200`, append(areaArgs(area), cidadao)...)
	if err != nil {
		return nil, wrap("AtendimentosNaArea", err)
	}
	return collect(rows, "AtendimentosNaArea", scanAtendimento)
}

// Painel conta o dia de cada unidade (unidades sem movimento saem zeradas).
func (r *Repository) Painel(ctx context.Context, db database.DBTX, unidades []domain.Unidade, dia string) ([]domain.PainelUnidade, error) {
	lista := make([]uuid.UUID, 0, len(unidades))
	for _, u := range unidades {
		lista = append(lista, u.ID)
	}
	rows, err := db.Query(ctx, `SELECT a.unidade_id,
			count(*) FILTER (WHERE a.status IN ('aguardando', 'chamado')),
			count(*) FILTER (WHERE a.status = 'em_atendimento'),
			count(*) FILTER (WHERE a.status = 'concluido'),
			count(*) FILTER (WHERE a.status = 'nao_compareceu'),
			COALESCE(avg(extract(epoch FROM a.chamado_em - a.created_at)) FILTER (WHERE a.chamado_em IS NOT NULL), 0) / 60
		FROM sempras_atendimentos a WHERE a.unidade_id = ANY($1::uuid[]) AND a.dia = $2::date
		GROUP BY a.unidade_id`, ids(lista), dia)
	if err != nil {
		return nil, wrap("Painel", err)
	}
	contagens, err := collect(rows, "Painel", func(row pgx.Row) (domain.PainelUnidade, error) {
		var p domain.PainelUnidade
		err := row.Scan(&p.Unidade.ID, &p.Aguardando, &p.EmAtendimento, &p.Concluidos, &p.NaoCompareceram, &p.EsperaMediaMin)
		return p, err
	})
	if err != nil {
		return nil, err
	}
	porUnidade := map[uuid.UUID]domain.PainelUnidade{}
	for _, p := range contagens {
		porUnidade[p.Unidade.ID] = p
	}
	out := make([]domain.PainelUnidade, 0, len(unidades))
	for _, u := range unidades {
		p := porUnidade[u.ID]
		p.Unidade = u
		out = append(out, p)
	}
	return out, nil
}

const atendimentoColetivoCols = `c.id, c.unidade_id, c.data_evento, c.categoria, c.participantes, c.descricao, c.created_at`

func scanAtendimentoColetivo(row pgx.Row) (domain.AtendimentoColetivo, error) {
	var a domain.AtendimentoColetivo
	err := row.Scan(&a.ID, &a.UnidadeID, &a.DataEvento, &a.Categoria, &a.Participantes, &a.Descricao, &a.CreatedAt)
	return a, err
}

// InsertAtendimentoColetivo grava um evento de grupo (SCFV) da unidade.
func (r *Repository) InsertAtendimentoColetivo(ctx context.Context, db database.DBTX, a domain.AtendimentoColetivo, autor uuid.UUID) error {
	if _, err := db.Exec(ctx, `INSERT INTO sempras_atendimentos_coletivos
		(id, unidade_id, data_evento, categoria, participantes, descricao, registrado_por)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		a.ID, a.UnidadeID, a.DataEvento, a.Categoria, a.Participantes, a.Descricao, autor); err != nil {
		return wrap("InsertAtendimentoColetivo", err)
	}
	return nil
}

// ListAtendimentosColetivos lista os eventos de uma unidade num período.
func (r *Repository) ListAtendimentosColetivos(ctx context.Context, db database.DBTX, unidade uuid.UUID, inicio, fim time.Time) ([]domain.AtendimentoColetivo, error) {
	rows, err := db.Query(ctx, `SELECT `+atendimentoColetivoCols+` FROM sempras_atendimentos_coletivos c
		WHERE c.unidade_id = $1 AND c.data_evento >= $2 AND c.data_evento < $3
		ORDER BY c.data_evento DESC LIMIT 500`, unidade, inicio, fim)
	if err != nil {
		return nil, wrap("ListAtendimentosColetivos", err)
	}
	return collect(rows, "ListAtendimentosColetivos", scanAtendimentoColetivo)
}

// rmaAgregar roda uma query agregada por unidade_id e aplica cada linha
// num *domain.RmaUnidade do mapa (criando-o na primeira vez).
func rmaAgregar(ctx context.Context, db database.DBTX, out map[uuid.UUID]*domain.RmaUnidade, sql string, args []any, aplicar func(row pgx.Row, m *domain.RmaUnidade) error) error {
	rows, err := db.Query(ctx, sql, args...)
	if err != nil {
		return wrap("Rma", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		m := &domain.RmaUnidade{}
		if err := aplicar(multiScanRow{rows, &id}, m); err != nil {
			return wrap("Rma", err)
		}
		if out[id] == nil {
			out[id] = &domain.RmaUnidade{}
		}
		atual := out[id]
		*atual = domain.RmaUnidade{
			Atendimentos: atual.Atendimentos + m.Atendimentos, AtendimentosSociais: atual.AtendimentosSociais + m.AtendimentosSociais,
			AtendimentosPsicologicos: atual.AtendimentosPsicologicos + m.AtendimentosPsicologicos,
			PaifAtivos:               atual.PaifAtivos + m.PaifAtivos, PaifExtremaPobreza: atual.PaifExtremaPobreza + m.PaifExtremaPobreza,
			PaifBolsaFamilia: atual.PaifBolsaFamilia + m.PaifBolsaFamilia, PaifBPC: atual.PaifBPC + m.PaifBPC,
			PaifSCFVParticipantes:     atual.PaifSCFVParticipantes + m.PaifSCFVParticipantes,
			BeneficiosConcedidosNoMes: atual.BeneficiosConcedidosNoMes + m.BeneficiosConcedidosNoMes,
			PaefiCasosAtivos:          atual.PaefiCasosAtivos + m.PaefiCasosAtivos, PaefiCasosAbertosNoMes: atual.PaefiCasosAbertosNoMes + m.PaefiCasosAbertosNoMes,
			MseAtivas: atual.MseAtivas + m.MseAtivas, MseConcluidasNoMes: atual.MseConcluidasNoMes + m.MseConcluidasNoMes,
			ConselhoAtendimentosNoMes:         atual.ConselhoAtendimentosNoMes + m.ConselhoAtendimentosNoMes,
			CreasNotificacoesRecebidasNoMes:   atual.CreasNotificacoesRecebidasNoMes + m.CreasNotificacoesRecebidasNoMes,
			CreasNotificacoesRespondidasNoMes: atual.CreasNotificacoesRespondidasNoMes + m.CreasNotificacoesRespondidasNoMes,
			CasaAbrigoAcolhidosAtivos:         atual.CasaAbrigoAcolhidosAtivos + m.CasaAbrigoAcolhidosAtivos,
			CasaAbrigoEntradasNoMes:           atual.CasaAbrigoEntradasNoMes + m.CasaAbrigoEntradasNoMes,
			CasaAbrigoDesligamentosNoMes:      atual.CasaAbrigoDesligamentosNoMes + m.CasaAbrigoDesligamentosNoMes,
			EncaminhamentosEnviadosNoMes:      atual.EncaminhamentosEnviadosNoMes + m.EncaminhamentosEnviadosNoMes,
		}
	}
	if err := rows.Err(); err != nil {
		return wrap("Rma", err)
	}
	return nil
}

// multiScanRow faz um row.Scan comum escrever também o primeiro valor
// (o unidade_id) num ponteiro externo — evita repetir as 9 queries com
// um scan manual de "var id uuid.UUID" cada.
type multiScanRow struct {
	pgx.Rows
	id *uuid.UUID
}

func (r multiScanRow) Scan(dest ...any) error { return r.Rows.Scan(append([]any{r.id}, dest...)...) }

// Rma consolida o Relatório Mensal de Atendimento de cada unidade numa
// competência — uma query agregada por entidade, casada por unidade_id
// (igual ao Painel, unidades sem movimento saem zeradas).
func (r *Repository) Rma(ctx context.Context, db database.DBTX, unidades []domain.Unidade, inicio, fim time.Time) ([]domain.RmaUnidade, error) {
	lista := make([]uuid.UUID, 0, len(unidades))
	for _, u := range unidades {
		lista = append(lista, u.ID)
	}
	args := []any{ids(lista), inicio, fim}
	out := map[uuid.UUID]*domain.RmaUnidade{}

	queries := []struct {
		sql     string
		aplicar func(row pgx.Row, m *domain.RmaUnidade) error
	}{
		{
			`SELECT unidade_id, count(*), count(*) FILTER (WHERE tipo = 'social'), count(*) FILTER (WHERE tipo = 'psicologico')
			 FROM sempras_atendimentos WHERE unidade_id = ANY($1::uuid[]) AND created_at >= $2 AND created_at < $3 GROUP BY unidade_id`,
			func(row pgx.Row, m *domain.RmaUnidade) error {
				return row.Scan(&m.Atendimentos, &m.AtendimentosSociais, &m.AtendimentosPsicologicos)
			},
		},
		{
			// Ativos é uma fotografia (sem relação com a competência) —
			// $2/$3 aparecem só para manter os 3 parâmetros iguais às
			// outras queries desta função.
			`SELECT unidade_id, count(*) FILTER (WHERE status = 'ativo'), count(*) FILTER (WHERE status = 'ativo' AND extrema_pobreza),
			 count(*) FILTER (WHERE status = 'ativo' AND bolsa_familia), count(*) FILTER (WHERE status = 'ativo' AND bpc),
			 COALESCE(sum(scfv_participantes) FILTER (WHERE status = 'ativo'), 0)
			 FROM sempras_paif_vinculos WHERE unidade_id = ANY($1::uuid[]) AND $2::timestamptz IS NOT NULL AND $3::timestamptz IS NOT NULL
			 GROUP BY unidade_id`,
			func(row pgx.Row, m *domain.RmaUnidade) error {
				return row.Scan(&m.PaifAtivos, &m.PaifExtremaPobreza, &m.PaifBolsaFamilia, &m.PaifBPC, &m.PaifSCFVParticipantes)
			},
		},
		{
			`SELECT unidade_id, count(*) FILTER (WHERE status = 'concedido' AND data_decisao >= $2 AND data_decisao < $3)
			 FROM sempras_beneficios WHERE unidade_id = ANY($1::uuid[]) GROUP BY unidade_id`,
			func(row pgx.Row, m *domain.RmaUnidade) error { return row.Scan(&m.BeneficiosConcedidosNoMes) },
		},
		{
			`SELECT unidade_id, count(*) FILTER (WHERE status = 'em_acompanhamento'),
			 count(*) FILTER (WHERE data_abertura >= $2 AND data_abertura < $3)
			 FROM sempras_paefi_casos WHERE unidade_id = ANY($1::uuid[]) GROUP BY unidade_id`,
			func(row pgx.Row, m *domain.RmaUnidade) error {
				return row.Scan(&m.PaefiCasosAtivos, &m.PaefiCasosAbertosNoMes)
			},
		},
		{
			`SELECT unidade_id, count(*) FILTER (WHERE status = 'em_cumprimento'),
			 count(*) FILTER (WHERE data_conclusao >= $2 AND data_conclusao < $3)
			 FROM sempras_mse_medidas WHERE unidade_id = ANY($1::uuid[]) GROUP BY unidade_id`,
			func(row pgx.Row, m *domain.RmaUnidade) error { return row.Scan(&m.MseAtivas, &m.MseConcluidasNoMes) },
		},
		{
			`SELECT unidade_id, count(*) FILTER (WHERE created_at >= $2 AND created_at < $3)
			 FROM sempras_conselho_atendimentos WHERE unidade_id = ANY($1::uuid[]) GROUP BY unidade_id`,
			func(row pgx.Row, m *domain.RmaUnidade) error { return row.Scan(&m.ConselhoAtendimentosNoMes) },
		},
		{
			`SELECT unidade_id, count(*) FILTER (WHERE data_entrada >= $2 AND data_entrada < $3),
			 count(*) FILTER (WHERE data_resposta >= $2 AND data_resposta < $3)
			 FROM sempras_creas_notificacoes WHERE unidade_id = ANY($1::uuid[]) GROUP BY unidade_id`,
			func(row pgx.Row, m *domain.RmaUnidade) error {
				return row.Scan(&m.CreasNotificacoesRecebidasNoMes, &m.CreasNotificacoesRespondidasNoMes)
			},
		},
		{
			`SELECT unidade_id, count(*) FILTER (WHERE status = 'acolhido'),
			 count(*) FILTER (WHERE data_entrada >= $2 AND data_entrada < $3),
			 count(*) FILTER (WHERE data_desligamento >= $2 AND data_desligamento < $3)
			 FROM sempras_casa_abrigo_acolhimentos WHERE unidade_id = ANY($1::uuid[]) GROUP BY unidade_id`,
			func(row pgx.Row, m *domain.RmaUnidade) error {
				return row.Scan(&m.CasaAbrigoAcolhidosAtivos, &m.CasaAbrigoEntradasNoMes, &m.CasaAbrigoDesligamentosNoMes)
			},
		},
		{
			`SELECT unidade_origem_id, count(*) FILTER (WHERE created_at >= $2 AND created_at < $3)
			 FROM sempras_encaminhamentos WHERE unidade_origem_id = ANY($1::uuid[]) GROUP BY unidade_origem_id`,
			func(row pgx.Row, m *domain.RmaUnidade) error { return row.Scan(&m.EncaminhamentosEnviadosNoMes) },
		},
		{
			`SELECT unidade_id, count(*) FILTER (WHERE data_recebimento >= $2 AND data_recebimento < $3),
			 count(*) FILTER (WHERE data_resposta >= $2 AND data_resposta < $3)
			 FROM sempras_cras_oficios WHERE unidade_id = ANY($1::uuid[]) GROUP BY unidade_id`,
			func(row pgx.Row, m *domain.RmaUnidade) error {
				return row.Scan(&m.OficiosCrasRecebidosNoMes, &m.OficiosCrasRespondidosNoMes)
			},
		},
		{
			`SELECT unidade_id,
			 COALESCE(sum(participantes) FILTER (WHERE categoria = 'paif_familias'), 0),
			 COALESCE(sum(participantes) FILTER (WHERE categoria = 'scfv_0_6'), 0),
			 COALESCE(sum(participantes) FILTER (WHERE categoria = 'scfv_7_14'), 0),
			 COALESCE(sum(participantes) FILTER (WHERE categoria = 'scfv_15_17'), 0),
			 COALESCE(sum(participantes) FILTER (WHERE categoria = 'scfv_18_59'), 0),
			 COALESCE(sum(participantes) FILTER (WHERE categoria = 'scfv_idosos'), 0),
			 COALESCE(sum(participantes) FILTER (WHERE categoria = 'palestras_nao_continuado'), 0),
			 COALESCE(sum(participantes) FILTER (WHERE categoria = 'pcd'), 0),
			 COALESCE(sum(participantes), 0)
			 FROM sempras_atendimentos_coletivos
			 WHERE unidade_id = ANY($1::uuid[]) AND data_evento >= $2 AND data_evento < $3 GROUP BY unidade_id`,
			func(row pgx.Row, m *domain.RmaUnidade) error {
				return row.Scan(&m.ColetivoPaifFamiliasNoMes, &m.ColetivoSCFV0a6NoMes, &m.ColetivoSCFV7a14NoMes,
					&m.ColetivoSCFV15a17NoMes, &m.ColetivoSCFV18a59NoMes, &m.ColetivoSCFVIdososNoMes,
					&m.ColetivoPalestrasNoMes, &m.ColetivoPCDNoMes, &m.ColetivoTotalNoMes)
			},
		},
	}
	for _, q := range queries {
		if err := rmaAgregar(ctx, db, out, q.sql, args, q.aplicar); err != nil {
			return nil, err
		}
	}

	resultado := make([]domain.RmaUnidade, 0, len(unidades))
	for _, u := range unidades {
		m := domain.RmaUnidade{}
		if p, ok := out[u.ID]; ok {
			m = *p
		}
		m.Unidade = u
		resultado = append(resultado, m)
	}
	return resultado, nil
}

// --------------------------------------------------------------- prontuário

// InsertRegistro grava a evolução (a tabela é só de inclusão).
func (r *Repository) InsertRegistro(ctx context.Context, db database.DBTX, reg domain.Registro) error {
	if _, err := db.Exec(ctx, `INSERT INTO sempras_registros
		(id, cidadao_id, unidade_id, atendimento_id, tipo, texto, autor_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		reg.ID, reg.CidadaoID, reg.UnidadeID, reg.AtendimentoID, reg.Tipo, reg.Texto, reg.AutorID, reg.CreatedAt); err != nil {
		return wrap("InsertRegistro", err)
	}
	return nil
}

// RegistrosNaArea lista as evoluções do munícipe nas unidades da área (o
// sigilo psicológico é aplicado pela aplicação, unidade a unidade).
func (r *Repository) RegistrosNaArea(ctx context.Context, db database.DBTX, cidadao uuid.UUID, area domain.Area) ([]domain.Registro, error) {
	rows, err := db.Query(ctx, `SELECT r.id, r.cidadao_id, r.unidade_id, r.atendimento_id, r.tipo, r.texto, r.autor_id,
			COALESCE(au.display_name, ''), r.created_at
		FROM sempras_registros r JOIN unidades u ON u.id = r.unidade_id LEFT JOIN users au ON au.id = r.autor_id
		WHERE r.cidadao_id = $4 AND `+areaSQL(1, 2, 3)+`
		ORDER BY r.created_at DESC LIMIT 500`, append(areaArgs(area), cidadao)...)
	if err != nil {
		return nil, wrap("RegistrosNaArea", err)
	}
	return collect(rows, "RegistrosNaArea", func(row pgx.Row) (domain.Registro, error) {
		var reg domain.Registro
		err := row.Scan(&reg.ID, &reg.CidadaoID, &reg.UnidadeID, &reg.AtendimentoID, &reg.Tipo, &reg.Texto, &reg.AutorID,
			&reg.AutorNome, &reg.CreatedAt)
		return reg, err
	})
}

// ------------------------------------------------------ direitos do titular

const pedidoCols = `p.id, p.cidadao_id, COALESCE(NULLIF(c.nome_social, ''), c.nome, ''),
	p.tipo, p.status, p.justificativa_solicitacao, p.solicitado_por,
	COALESCE(sol.display_name, ''), COALESCE(p.justificativa_decisao, ''), p.decidido_por,
	COALESCE(dec.display_name, ''), p.decidido_em, p.created_at`
const pedidoFrom = ` FROM sempras_pedidos_titular p
	LEFT JOIN sempras_cidadaos c ON c.id = p.cidadao_id
	LEFT JOIN users sol ON sol.id = p.solicitado_por
	LEFT JOIN users dec ON dec.id = p.decidido_por`

func scanPedido(row pgx.Row) (domain.PedidoTitular, error) {
	var p domain.PedidoTitular
	err := row.Scan(&p.ID, &p.CidadaoID, &p.CidadaoNome, &p.Tipo, &p.Status, &p.JustificativaSolicitacao, &p.SolicitadoPor,
		&p.SolicitadoPorNome, &p.JustificativaDecisao, &p.DecididoPor, &p.DecididoPorNome, &p.DecididoEm, &p.CreatedAt)
	return p, err
}

// InsertPedido grava um pedido do titular (acesso/correção já concluídos;
// anonimização nasce pendente).
func (r *Repository) InsertPedido(ctx context.Context, db database.DBTX, p domain.PedidoTitular) error {
	if _, err := db.Exec(ctx, `INSERT INTO sempras_pedidos_titular
		(id, cidadao_id, tipo, status, justificativa_solicitacao, solicitado_por, decidido_por, decidido_em, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		p.ID, p.CidadaoID, p.Tipo, p.Status, p.JustificativaSolicitacao, p.SolicitadoPor, p.DecididoPor, p.DecididoEm, p.CreatedAt); err != nil {
		return wrap("InsertPedido", err)
	}
	return nil
}

// GetPedido busca um pedido (forUpdate trava a linha na transação da decisão).
func (r *Repository) GetPedido(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (domain.PedidoTitular, error) {
	q := `SELECT ` + pedidoCols + pedidoFrom + ` WHERE p.id = $1`
	if forUpdate {
		q += ` FOR UPDATE OF p`
	}
	p, err := scanPedido(db.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PedidoTitular{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.PedidoTitular{}, wrap("GetPedido", err)
	}
	return p, nil
}

// ListPedidosPendentes lista os pedidos à espera de decisão (só
// anonimização chega pendente). Sem filtro de área: decidir exige
// cobertura total (ADR 030 §8).
func (r *Repository) ListPedidosPendentes(ctx context.Context, db database.DBTX) ([]domain.PedidoTitular, error) {
	rows, err := db.Query(ctx, `SELECT `+pedidoCols+pedidoFrom+`
		WHERE p.status = 'pendente' ORDER BY p.created_at`)
	if err != nil {
		return nil, wrap("ListPedidosPendentes", err)
	}
	return collect(rows, "ListPedidosPendentes", scanPedido)
}

// DecidirPedido grava a decisão da gestão (o trigger impede decidir de novo).
func (r *Repository) DecidirPedido(ctx context.Context, db database.DBTX, id uuid.UUID, status string, decididoPor uuid.UUID, justificativa string, decididoEm time.Time) error {
	if _, err := db.Exec(ctx, `UPDATE sempras_pedidos_titular
		SET status = $2, decidido_por = $3, justificativa_decisao = NULLIF($4, ''), decidido_em = $5
		WHERE id = $1`, id, status, decididoPor, justificativa, decididoEm); err != nil {
		return wrap("DecidirPedido", err)
	}
	return nil
}

// AnonimizarCidadao zera os campos identificadores do cadastro — nunca
// DELETE: sempras_atendimentos/sempras_registros referenciam o id, e são o
// registro do próprio serviço prestado (valor legal e estatístico), não
// "dado do titular" neste sentido. data_nascimento e nome_mae ficam com um
// valor-tombstone não vazio porque a CHECK de identificação da tabela
// exige cpf OU nis OU (data_nascimento E nome_mae) — zerar os três ao
// mesmo tempo quebraria a integridade do próprio cadastro.
func (r *Repository) AnonimizarCidadao(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if _, err := db.Exec(ctx, `UPDATE sempras_cidadaos SET
		nome = 'Munícipe anonimizado', nome_social = '', cpf = NULL, nis = NULL,
		nome_mae = '(anonimizado)', telefone = '', endereco = ''
		WHERE id = $1`, id); err != nil {
		return wrap("AnonimizarCidadao", err)
	}
	return nil
}

// ------------------------------------------------------------ rede de apoio

const redeApoioCols = `a.id, a.cidadao_id, a.nome, a.parentesco, a.telefone, a.created_at`

func scanContatoRedeApoio(row pgx.Row) (domain.ContatoRedeApoio, error) {
	var c domain.ContatoRedeApoio
	err := row.Scan(&c.ID, &c.CidadaoID, &c.Nome, &c.Parentesco, &c.Telefone, &c.CreatedAt)
	return c, err
}

// InsertContatoRedeApoio grava um contato da rede de apoio.
func (r *Repository) InsertContatoRedeApoio(ctx context.Context, db database.DBTX, c domain.ContatoRedeApoio, autor uuid.UUID) error {
	if _, err := db.Exec(ctx, `INSERT INTO sempras_rede_apoio (id, cidadao_id, nome, parentesco, telefone, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, c.ID, c.CidadaoID, c.Nome, c.Parentesco, c.Telefone, autor, c.CreatedAt); err != nil {
		return wrap("InsertContatoRedeApoio", err)
	}
	return nil
}

// ListContatosRedeApoio lista a rede de apoio do munícipe (não é por
// unidade — mesmo escopo secretaria-wide do cadastro).
func (r *Repository) ListContatosRedeApoio(ctx context.Context, db database.DBTX, cidadao uuid.UUID) ([]domain.ContatoRedeApoio, error) {
	rows, err := db.Query(ctx, `SELECT `+redeApoioCols+` FROM sempras_rede_apoio a
		WHERE a.cidadao_id = $1 ORDER BY a.created_at`, cidadao)
	if err != nil {
		return nil, wrap("ListContatosRedeApoio", err)
	}
	return collect(rows, "ListContatosRedeApoio", scanContatoRedeApoio)
}

// GetContatoRedeApoio busca um contato (confere a quem pertence antes de remover).
func (r *Repository) GetContatoRedeApoio(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.ContatoRedeApoio, error) {
	c, err := scanContatoRedeApoio(db.QueryRow(ctx, `SELECT `+redeApoioCols+` FROM sempras_rede_apoio a WHERE a.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ContatoRedeApoio{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.ContatoRedeApoio{}, wrap("GetContatoRedeApoio", err)
	}
	return c, nil
}

// DeleteContatoRedeApoio remove um contato (cadastrado errado, pessoa que
// pediu pra sair da lista etc. — sem valor de prova como o prontuário).
func (r *Repository) DeleteContatoRedeApoio(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if _, err := db.Exec(ctx, `DELETE FROM sempras_rede_apoio WHERE id = $1`, id); err != nil {
		return wrap("DeleteContatoRedeApoio", err)
	}
	return nil
}

// -------------------------------------------------- benefícios eventuais

const beneficioCols = `b.id, b.beneficiario_id, b.solicitante_id, b.unidade_id, b.tipo, b.forma_acesso, b.quantidade, b.status,
	b.observacao, b.motivo_decisao, b.tecnico_decisao_id, b.data_solicitacao, b.data_decisao`

func scanBeneficio(row pgx.Row) (domain.Beneficio, error) {
	var b domain.Beneficio
	err := row.Scan(&b.ID, &b.BeneficiarioID, &b.SolicitanteID, &b.UnidadeID, &b.Tipo, &b.FormaAcesso, &b.Quantidade, &b.Status,
		&b.Observacao, &b.MotivoDecisao, &b.TecnicoDecisaoID, &b.DataSolicitacao, &b.DataDecisao)
	return b, err
}

// InsertBeneficio grava um pedido de benefício eventual (nasce 'solicitado').
func (r *Repository) InsertBeneficio(ctx context.Context, db database.DBTX, b domain.Beneficio, autor uuid.UUID) error {
	if _, err := db.Exec(ctx, `INSERT INTO sempras_beneficios
		(id, beneficiario_id, solicitante_id, unidade_id, tipo, forma_acesso, quantidade, status, observacao, solicitado_por, data_solicitacao)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		b.ID, b.BeneficiarioID, b.SolicitanteID, b.UnidadeID, b.Tipo, b.FormaAcesso, b.Quantidade, b.Status, b.Observacao, autor, b.DataSolicitacao); err != nil {
		return wrap("InsertBeneficio", err)
	}
	return nil
}

// GetBeneficio busca o benefício (forUpdate trava a linha na decisão).
func (r *Repository) GetBeneficio(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (domain.Beneficio, error) {
	q := `SELECT ` + beneficioCols + ` FROM sempras_beneficios b WHERE b.id = $1`
	if forUpdate {
		q += ` FOR UPDATE`
	}
	b, err := scanBeneficio(db.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Beneficio{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Beneficio{}, wrap("GetBeneficio", err)
	}
	return b, nil
}

// UpdateBeneficio grava a decisão (conceder/negar/suspender).
func (r *Repository) UpdateBeneficio(ctx context.Context, db database.DBTX, b domain.Beneficio) error {
	if _, err := db.Exec(ctx, `UPDATE sempras_beneficios SET status = $2, motivo_decisao = $3,
		tecnico_decisao_id = $4, data_decisao = $5 WHERE id = $1`,
		b.ID, b.Status, b.MotivoDecisao, b.TecnicoDecisaoID, b.DataDecisao); err != nil {
		return wrap("UpdateBeneficio", err)
	}
	return nil
}

// BeneficiosNaArea lista os benefícios do munícipe concedidos nas unidades
// da área de quem consulta (mesmo recorte de AtendimentosNaArea).
func (r *Repository) BeneficiosNaArea(ctx context.Context, db database.DBTX, beneficiario uuid.UUID, area domain.Area) ([]domain.Beneficio, error) {
	rows, err := db.Query(ctx, `SELECT `+beneficioCols+` FROM sempras_beneficios b JOIN unidades u ON u.id = b.unidade_id
		WHERE b.beneficiario_id = $4 AND `+areaSQL(1, 2, 3)+`
		ORDER BY b.data_solicitacao DESC LIMIT 200`, append(areaArgs(area), beneficiario)...)
	if err != nil {
		return nil, wrap("BeneficiosNaArea", err)
	}
	return collect(rows, "BeneficiosNaArea", scanBeneficio)
}

// ---------------------------------------------------------- encaminhamentos

const encaminhamentoCols = `e.id, e.cidadao_id, e.unidade_origem_id, e.unidade_destino_id, e.motivo, e.status,
	e.observacao_decisao, e.encaminhado_por, e.decidido_por, e.created_at, e.decidido_em`

func scanEncaminhamento(row pgx.Row) (domain.Encaminhamento, error) {
	var e domain.Encaminhamento
	err := row.Scan(&e.ID, &e.CidadaoID, &e.UnidadeOrigemID, &e.UnidadeDestinoID, &e.Motivo, &e.Status,
		&e.ObservacaoDecisao, &e.EncaminhadoPor, &e.DecididoPor, &e.CreatedAt, &e.DecididoEm)
	return e, err
}

// InsertEncaminhamento grava um encaminhamento (nasce 'pendente').
func (r *Repository) InsertEncaminhamento(ctx context.Context, db database.DBTX, e domain.Encaminhamento, autor uuid.UUID) error {
	if _, err := db.Exec(ctx, `INSERT INTO sempras_encaminhamentos
		(id, cidadao_id, unidade_origem_id, unidade_destino_id, motivo, status, encaminhado_por, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		e.ID, e.CidadaoID, e.UnidadeOrigemID, e.UnidadeDestinoID, e.Motivo, e.Status, autor, e.CreatedAt); err != nil {
		return wrap("InsertEncaminhamento", err)
	}
	return nil
}

// GetEncaminhamento busca o encaminhamento (forUpdate trava a linha na decisão).
func (r *Repository) GetEncaminhamento(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (domain.Encaminhamento, error) {
	q := `SELECT ` + encaminhamentoCols + ` FROM sempras_encaminhamentos e WHERE e.id = $1`
	if forUpdate {
		q += ` FOR UPDATE`
	}
	e, err := scanEncaminhamento(db.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Encaminhamento{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Encaminhamento{}, wrap("GetEncaminhamento", err)
	}
	return e, nil
}

// UpdateEncaminhamento grava a decisão (aceitar/recusar).
func (r *Repository) UpdateEncaminhamento(ctx context.Context, db database.DBTX, e domain.Encaminhamento) error {
	if _, err := db.Exec(ctx, `UPDATE sempras_encaminhamentos SET status = $2, observacao_decisao = $3,
		decidido_por = $4, decidido_em = $5 WHERE id = $1`,
		e.ID, e.Status, e.ObservacaoDecisao, e.DecididoPor, e.DecididoEm); err != nil {
		return wrap("UpdateEncaminhamento", err)
	}
	return nil
}

// EncaminhamentosDoCidadaoNaArea lista os encaminhamentos do munícipe em
// que a origem OU o destino estão na área de quem consulta.
func (r *Repository) EncaminhamentosDoCidadaoNaArea(ctx context.Context, db database.DBTX, cidadao uuid.UUID, area domain.Area) ([]domain.Encaminhamento, error) {
	rows, err := db.Query(ctx, `SELECT `+encaminhamentoCols+`
		FROM sempras_encaminhamentos e
		JOIN unidades uo ON uo.id = e.unidade_origem_id
		JOIN unidades ud ON ud.id = e.unidade_destino_id
		WHERE e.cidadao_id = $4 AND (
			$1 OR uo.id = ANY($2::uuid[]) OR uo.entidade_id = ANY($3::uuid[])
			   OR ud.id = ANY($2::uuid[]) OR ud.entidade_id = ANY($3::uuid[])
		)
		ORDER BY e.created_at DESC LIMIT 200`, append(areaArgs(area), cidadao)...)
	if err != nil {
		return nil, wrap("EncaminhamentosDoCidadaoNaArea", err)
	}
	return collect(rows, "EncaminhamentosDoCidadaoNaArea", scanEncaminhamento)
}

// ListEncaminhamentosPendentes é a fila de encaminhamentos recebidos por
// uma unidade — quem atende lá decide aceitar ou recusar.
func (r *Repository) ListEncaminhamentosPendentes(ctx context.Context, db database.DBTX, unidadeDestino uuid.UUID) ([]domain.Encaminhamento, error) {
	rows, err := db.Query(ctx, `SELECT `+encaminhamentoCols+` FROM sempras_encaminhamentos e
		WHERE e.unidade_destino_id = $1 AND e.status = 'pendente' ORDER BY e.created_at`, unidadeDestino)
	if err != nil {
		return nil, wrap("ListEncaminhamentosPendentes", err)
	}
	return collect(rows, "ListEncaminhamentosPendentes", scanEncaminhamento)
}

// ------------------------------------------------------------------- PAIF

const paifCols = `p.id, p.cidadao_id, p.unidade_id, p.status, p.tipo_atendimento, p.extrema_pobreza, p.bolsa_familia, p.bpc,
	p.scfv_participantes, p.motivo_exclusao, p.incluido_por, p.excluido_por, p.data_inclusao, p.data_exclusao`

func scanPaifVinculo(row pgx.Row) (domain.PaifVinculo, error) {
	var p domain.PaifVinculo
	err := row.Scan(&p.ID, &p.CidadaoID, &p.UnidadeID, &p.Status, &p.TipoAtendimento, &p.ExtremaPobreza, &p.BolsaFamilia, &p.BPC,
		&p.SCFVParticipantes, &p.MotivoExclusao, &p.IncluidoPor, &p.ExcluidoPor, &p.DataInclusao, &p.DataExclusao)
	return p, err
}

// InsertPaifVinculo grava uma inclusão no PAIF (duas ativas pra mesma
// família no mesmo CRAS vira ErrPaifJaAtivo).
func (r *Repository) InsertPaifVinculo(ctx context.Context, db database.DBTX, p domain.PaifVinculo, autor uuid.UUID) error {
	_, err := db.Exec(ctx, `INSERT INTO sempras_paif_vinculos
		(id, cidadao_id, unidade_id, status, tipo_atendimento, extrema_pobreza, bolsa_familia, bpc, scfv_participantes, incluido_por, data_inclusao)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		p.ID, p.CidadaoID, p.UnidadeID, p.Status, p.TipoAtendimento, p.ExtremaPobreza, p.BolsaFamilia, p.BPC, p.SCFVParticipantes, autor, p.DataInclusao)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.ConstraintName == "uq_sempras_paif_ativo" {
		return domain.ErrPaifJaAtivo
	}
	if err != nil {
		return wrap("InsertPaifVinculo", err)
	}
	return nil
}

// GetPaifVinculo busca o vínculo (forUpdate trava a linha na exclusão).
func (r *Repository) GetPaifVinculo(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (domain.PaifVinculo, error) {
	q := `SELECT ` + paifCols + ` FROM sempras_paif_vinculos p WHERE p.id = $1`
	if forUpdate {
		q += ` FOR UPDATE`
	}
	p, err := scanPaifVinculo(db.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PaifVinculo{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.PaifVinculo{}, wrap("GetPaifVinculo", err)
	}
	return p, nil
}

// UpdatePaifVinculo grava a exclusão do vínculo.
func (r *Repository) UpdatePaifVinculo(ctx context.Context, db database.DBTX, p domain.PaifVinculo) error {
	if _, err := db.Exec(ctx, `UPDATE sempras_paif_vinculos SET status = $2, motivo_exclusao = $3,
		excluido_por = $4, data_exclusao = $5 WHERE id = $1`,
		p.ID, p.Status, p.MotivoExclusao, p.ExcluidoPor, p.DataExclusao); err != nil {
		return wrap("UpdatePaifVinculo", err)
	}
	return nil
}

// PaifVinculosNaArea lista os vínculos PAIF da família nas unidades da
// área de quem consulta.
func (r *Repository) PaifVinculosNaArea(ctx context.Context, db database.DBTX, cidadao uuid.UUID, area domain.Area) ([]domain.PaifVinculo, error) {
	rows, err := db.Query(ctx, `SELECT `+paifCols+` FROM sempras_paif_vinculos p JOIN unidades u ON u.id = p.unidade_id
		WHERE p.cidadao_id = $4 AND `+areaSQL(1, 2, 3)+`
		ORDER BY p.data_inclusao DESC LIMIT 200`, append(areaArgs(area), cidadao)...)
	if err != nil {
		return nil, wrap("PaifVinculosNaArea", err)
	}
	return collect(rows, "PaifVinculosNaArea", scanPaifVinculo)
}

// ------------------------------------------------------- Conselho Tutelar

const conselhoCols = `c.id, c.protocolo, c.crianca_id, c.unidade_id, c.responsavel_nome, c.responsavel_cpf,
	c.responsavel_telefone, c.origem_demanda, c.motivo_violacao, c.medida_protetiva, c.status, c.observacoes,
	c.conselheiro_id, c.created_at, c.atualizado_em`

func scanConselhoAtendimento(row pgx.Row) (domain.ConselhoAtendimento, error) {
	var c domain.ConselhoAtendimento
	err := row.Scan(&c.ID, &c.Protocolo, &c.CriancaID, &c.UnidadeID, &c.ResponsavelNome, &c.ResponsavelCPF,
		&c.ResponsavelTelefone, &c.OrigemDemanda, &c.MotivoViolacao, &c.MedidaProtetiva, &c.Status, &c.Observacoes,
		&c.ConselheiroID, &c.CreatedAt, &c.AtualizadoEm)
	return c, err
}

// InsertConselhoAtendimento grava um atendimento (nasce 'em_andamento').
func (r *Repository) InsertConselhoAtendimento(ctx context.Context, db database.DBTX, c domain.ConselhoAtendimento, autor uuid.UUID) error {
	if _, err := db.Exec(ctx, `INSERT INTO sempras_conselho_atendimentos
		(id, protocolo, crianca_id, unidade_id, responsavel_nome, responsavel_cpf, responsavel_telefone,
		 origem_demanda, motivo_violacao, status, observacoes, registrado_por, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		c.ID, c.Protocolo, c.CriancaID, c.UnidadeID, c.ResponsavelNome, c.ResponsavelCPF, c.ResponsavelTelefone,
		c.OrigemDemanda, c.MotivoViolacao, c.Status, c.Observacoes, autor, c.CreatedAt); err != nil {
		return wrap("InsertConselhoAtendimento", err)
	}
	return nil
}

// GetConselhoAtendimento busca o atendimento (forUpdate trava a linha na transição).
func (r *Repository) GetConselhoAtendimento(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (domain.ConselhoAtendimento, error) {
	q := `SELECT ` + conselhoCols + ` FROM sempras_conselho_atendimentos c WHERE c.id = $1`
	if forUpdate {
		q += ` FOR UPDATE`
	}
	c, err := scanConselhoAtendimento(db.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ConselhoAtendimento{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.ConselhoAtendimento{}, wrap("GetConselhoAtendimento", err)
	}
	return c, nil
}

// UpdateConselhoAtendimento grava a transição (medida/encaminhamento/arquivamento).
func (r *Repository) UpdateConselhoAtendimento(ctx context.Context, db database.DBTX, c domain.ConselhoAtendimento) error {
	if _, err := db.Exec(ctx, `UPDATE sempras_conselho_atendimentos SET status = $2, medida_protetiva = $3,
		conselheiro_id = $4, atualizado_em = $5 WHERE id = $1`,
		c.ID, c.Status, c.MedidaProtetiva, c.ConselheiroID, c.AtualizadoEm); err != nil {
		return wrap("UpdateConselhoAtendimento", err)
	}
	return nil
}

// ConselhoAtendimentosNaArea lista os atendimentos da criança nas
// unidades da área de quem consulta.
func (r *Repository) ConselhoAtendimentosNaArea(ctx context.Context, db database.DBTX, crianca uuid.UUID, area domain.Area) ([]domain.ConselhoAtendimento, error) {
	rows, err := db.Query(ctx, `SELECT `+conselhoCols+` FROM sempras_conselho_atendimentos c JOIN unidades u ON u.id = c.unidade_id
		WHERE c.crianca_id = $4 AND `+areaSQL(1, 2, 3)+`
		ORDER BY c.created_at DESC LIMIT 200`, append(areaArgs(area), crianca)...)
	if err != nil {
		return nil, wrap("ConselhoAtendimentosNaArea", err)
	}
	return collect(rows, "ConselhoAtendimentosNaArea", scanConselhoAtendimento)
}

// ------------------------------------------------------------------- PAEFI

const paefiCols = `p.id, p.caso_codigo, p.cidadao_id, p.unidade_id, p.violacoes, p.membros_vulneraveis, p.pia_elaborado,
	p.status, p.motivo_encerramento, p.tecnico_referencia_id, p.observacoes, p.data_abertura, p.data_encerramento`

func scanPaefiCaso(row pgx.Row) (domain.PaefiCaso, error) {
	var p domain.PaefiCaso
	err := row.Scan(&p.ID, &p.CasoCodigo, &p.CidadaoID, &p.UnidadeID, &p.Violacoes, &p.MembrosVulneraveis, &p.PiaElaborado,
		&p.Status, &p.MotivoEncerramento, &p.TecnicoReferenciaID, &p.Observacoes, &p.DataAbertura, &p.DataEncerramento)
	return p, err
}

// InsertPaefiCaso grava um caso recém-aberto (nasce 'em_acompanhamento').
func (r *Repository) InsertPaefiCaso(ctx context.Context, db database.DBTX, p domain.PaefiCaso, autor uuid.UUID) error {
	if _, err := db.Exec(ctx, `INSERT INTO sempras_paefi_casos
		(id, caso_codigo, cidadao_id, unidade_id, violacoes, membros_vulneraveis, status, observacoes, registrado_por, data_abertura)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		p.ID, p.CasoCodigo, p.CidadaoID, p.UnidadeID, p.Violacoes, p.MembrosVulneraveis, p.Status, p.Observacoes, autor, p.DataAbertura); err != nil {
		return wrap("InsertPaefiCaso", err)
	}
	return nil
}

// GetPaefiCaso busca o caso (forUpdate trava a linha na transição).
func (r *Repository) GetPaefiCaso(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (domain.PaefiCaso, error) {
	q := `SELECT ` + paefiCols + ` FROM sempras_paefi_casos p WHERE p.id = $1`
	if forUpdate {
		q += ` FOR UPDATE`
	}
	p, err := scanPaefiCaso(db.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PaefiCaso{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.PaefiCaso{}, wrap("GetPaefiCaso", err)
	}
	return p, nil
}

// UpdatePaefiCaso grava a elaboração do PIA ou o encerramento do caso.
func (r *Repository) UpdatePaefiCaso(ctx context.Context, db database.DBTX, p domain.PaefiCaso) error {
	if _, err := db.Exec(ctx, `UPDATE sempras_paefi_casos SET pia_elaborado = $2, status = $3, motivo_encerramento = $4,
		tecnico_referencia_id = $5, data_encerramento = $6 WHERE id = $1`,
		p.ID, p.PiaElaborado, p.Status, p.MotivoEncerramento, p.TecnicoReferenciaID, p.DataEncerramento); err != nil {
		return wrap("UpdatePaefiCaso", err)
	}
	return nil
}

// PaefiCasosNaArea lista os casos do munícipe nas unidades da área de quem consulta.
func (r *Repository) PaefiCasosNaArea(ctx context.Context, db database.DBTX, cidadao uuid.UUID, area domain.Area) ([]domain.PaefiCaso, error) {
	rows, err := db.Query(ctx, `SELECT `+paefiCols+` FROM sempras_paefi_casos p JOIN unidades u ON u.id = p.unidade_id
		WHERE p.cidadao_id = $4 AND `+areaSQL(1, 2, 3)+`
		ORDER BY p.data_abertura DESC LIMIT 200`, append(areaArgs(area), cidadao)...)
	if err != nil {
		return nil, wrap("PaefiCasosNaArea", err)
	}
	return collect(rows, "PaefiCasosNaArea", scanPaefiCaso)
}

// --------------------------------------------------------------------- MSE

const mseCols = `m.id, m.adolescente_id, m.unidade_id, m.numero_processo, m.vara_origem, m.ato_infracional, m.tipo_medida,
	m.instituicao_parceira, m.status, m.tecnico_referencia_id, m.observacoes, m.data_inicio, m.prazo_previsto, m.data_conclusao`

func scanMseMedida(row pgx.Row) (domain.MseMedida, error) {
	var m domain.MseMedida
	err := row.Scan(&m.ID, &m.AdolescenteID, &m.UnidadeID, &m.NumeroProcesso, &m.VaraOrigem, &m.AtoInfracional, &m.TipoMedida,
		&m.InstituicaoParceira, &m.Status, &m.TecnicoReferenciaID, &m.Observacoes, &m.DataInicio, &m.PrazoPrevisto, &m.DataConclusao)
	return m, err
}

// InsertMseMedida grava uma medida recém-registrada (nasce 'em_cumprimento').
func (r *Repository) InsertMseMedida(ctx context.Context, db database.DBTX, m domain.MseMedida, autor uuid.UUID) error {
	if _, err := db.Exec(ctx, `INSERT INTO sempras_mse_medidas
		(id, adolescente_id, unidade_id, numero_processo, vara_origem, ato_infracional, tipo_medida, instituicao_parceira,
		 status, observacoes, registrado_por, data_inicio, prazo_previsto)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		m.ID, m.AdolescenteID, m.UnidadeID, m.NumeroProcesso, m.VaraOrigem, m.AtoInfracional, m.TipoMedida, m.InstituicaoParceira,
		m.Status, m.Observacoes, autor, m.DataInicio, m.PrazoPrevisto); err != nil {
		return wrap("InsertMseMedida", err)
	}
	return nil
}

// GetMseMedida busca a medida (forUpdate trava a linha na conclusão).
func (r *Repository) GetMseMedida(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (domain.MseMedida, error) {
	q := `SELECT ` + mseCols + ` FROM sempras_mse_medidas m WHERE m.id = $1`
	if forUpdate {
		q += ` FOR UPDATE`
	}
	m, err := scanMseMedida(db.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.MseMedida{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.MseMedida{}, wrap("GetMseMedida", err)
	}
	return m, nil
}

// UpdateMseMedida grava a conclusão do acompanhamento.
func (r *Repository) UpdateMseMedida(ctx context.Context, db database.DBTX, m domain.MseMedida) error {
	if _, err := db.Exec(ctx, `UPDATE sempras_mse_medidas SET status = $2, tecnico_referencia_id = $3, data_conclusao = $4 WHERE id = $1`,
		m.ID, m.Status, m.TecnicoReferenciaID, m.DataConclusao); err != nil {
		return wrap("UpdateMseMedida", err)
	}
	return nil
}

// MseMedidasNaArea lista as medidas do adolescente nas unidades da área de quem consulta.
func (r *Repository) MseMedidasNaArea(ctx context.Context, db database.DBTX, adolescente uuid.UUID, area domain.Area) ([]domain.MseMedida, error) {
	rows, err := db.Query(ctx, `SELECT `+mseCols+` FROM sempras_mse_medidas m JOIN unidades u ON u.id = m.unidade_id
		WHERE m.adolescente_id = $4 AND `+areaSQL(1, 2, 3)+`
		ORDER BY m.data_inicio DESC LIMIT 200`, append(areaArgs(area), adolescente)...)
	if err != nil {
		return nil, wrap("MseMedidasNaArea", err)
	}
	return collect(rows, "MseMedidasNaArea", scanMseMedida)
}

// ----------------------------------------------------- notificações (CREAS)

const notificacaoCreasCols = `n.id, n.protocolo, n.vitima_id, n.unidade_id, n.origem_entidade, n.assunto, n.status,
	n.resposta, n.tecnico_referencia_id, n.data_entrada, n.prazo_resposta, n.data_resposta`

func scanNotificacaoCreas(row pgx.Row) (domain.NotificacaoCreas, error) {
	var n domain.NotificacaoCreas
	err := row.Scan(&n.ID, &n.Protocolo, &n.VitimaID, &n.UnidadeID, &n.OrigemEntidade, &n.Assunto, &n.Status,
		&n.Resposta, &n.TecnicoReferenciaID, &n.DataEntrada, &n.PrazoResposta, &n.DataResposta)
	return n, err
}

// InsertNotificacaoCreas grava uma notificação recém-recebida (nasce 'recebida').
func (r *Repository) InsertNotificacaoCreas(ctx context.Context, db database.DBTX, n domain.NotificacaoCreas, autor uuid.UUID) error {
	if _, err := db.Exec(ctx, `INSERT INTO sempras_creas_notificacoes
		(id, protocolo, vitima_id, unidade_id, origem_entidade, assunto, status, registrado_por, data_entrada, prazo_resposta)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		n.ID, n.Protocolo, n.VitimaID, n.UnidadeID, n.OrigemEntidade, n.Assunto, n.Status, autor, n.DataEntrada, n.PrazoResposta); err != nil {
		return wrap("InsertNotificacaoCreas", err)
	}
	return nil
}

// GetNotificacaoCreas busca a notificação (forUpdate trava a linha na transição).
func (r *Repository) GetNotificacaoCreas(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (domain.NotificacaoCreas, error) {
	q := `SELECT ` + notificacaoCreasCols + ` FROM sempras_creas_notificacoes n WHERE n.id = $1`
	if forUpdate {
		q += ` FOR UPDATE`
	}
	n, err := scanNotificacaoCreas(db.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.NotificacaoCreas{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.NotificacaoCreas{}, wrap("GetNotificacaoCreas", err)
	}
	return n, nil
}

// UpdateNotificacaoCreas grava a transição (diligência, resposta ou arquivamento).
func (r *Repository) UpdateNotificacaoCreas(ctx context.Context, db database.DBTX, n domain.NotificacaoCreas) error {
	if _, err := db.Exec(ctx, `UPDATE sempras_creas_notificacoes SET status = $2, resposta = $3,
		tecnico_referencia_id = $4, data_resposta = $5 WHERE id = $1`,
		n.ID, n.Status, n.Resposta, n.TecnicoReferenciaID, n.DataResposta); err != nil {
		return wrap("UpdateNotificacaoCreas", err)
	}
	return nil
}

// NotificacoesCreasNaArea lista as notificações da vítima nas unidades da área de quem consulta.
func (r *Repository) NotificacoesCreasNaArea(ctx context.Context, db database.DBTX, vitima uuid.UUID, area domain.Area) ([]domain.NotificacaoCreas, error) {
	rows, err := db.Query(ctx, `SELECT `+notificacaoCreasCols+` FROM sempras_creas_notificacoes n JOIN unidades u ON u.id = n.unidade_id
		WHERE n.vitima_id = $4 AND `+areaSQL(1, 2, 3)+`
		ORDER BY n.data_entrada DESC LIMIT 200`, append(areaArgs(area), vitima)...)
	if err != nil {
		return nil, wrap("NotificacoesCreasNaArea", err)
	}
	return collect(rows, "NotificacoesCreasNaArea", scanNotificacaoCreas)
}

// --------------------------------------------------------------- Ofícios (CRAS)

const oficioCrasCols = `o.id, o.cidadao_id, o.unidade_id, o.orgao_emissor, o.numero_oficio, o.data_oficio, o.data_recebimento,
	o.assunto, o.status, o.numero_resposta, o.data_resposta, o.tecnico_responsavel_id`

func scanOficioCras(row pgx.Row) (domain.OficioCras, error) {
	var o domain.OficioCras
	err := row.Scan(&o.ID, &o.CidadaoID, &o.UnidadeID, &o.OrgaoEmissor, &o.NumeroOficio, &o.DataOficio, &o.DataRecebimento,
		&o.Assunto, &o.Status, &o.NumeroResposta, &o.DataResposta, &o.TecnicoResponsavelID)
	return o, err
}

// InsertOficioCras grava um ofício recém-recebido (nasce 'pendente').
func (r *Repository) InsertOficioCras(ctx context.Context, db database.DBTX, o domain.OficioCras, autor uuid.UUID) error {
	if _, err := db.Exec(ctx, `INSERT INTO sempras_cras_oficios
		(id, cidadao_id, unidade_id, orgao_emissor, numero_oficio, data_oficio, assunto, status, registrado_por)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		o.ID, o.CidadaoID, o.UnidadeID, o.OrgaoEmissor, o.NumeroOficio, o.DataOficio, o.Assunto, o.Status, autor); err != nil {
		return wrap("InsertOficioCras", err)
	}
	return nil
}

// GetOficioCras busca o ofício (forUpdate trava a linha ao responder).
func (r *Repository) GetOficioCras(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (domain.OficioCras, error) {
	q := `SELECT ` + oficioCrasCols + ` FROM sempras_cras_oficios o WHERE o.id = $1`
	if forUpdate {
		q += ` FOR UPDATE`
	}
	o, err := scanOficioCras(db.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.OficioCras{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.OficioCras{}, wrap("GetOficioCras", err)
	}
	return o, nil
}

// UpdateOficioCras grava a resposta do ofício.
func (r *Repository) UpdateOficioCras(ctx context.Context, db database.DBTX, o domain.OficioCras) error {
	if _, err := db.Exec(ctx, `UPDATE sempras_cras_oficios SET status = $2, numero_resposta = $3,
		data_resposta = $4, tecnico_responsavel_id = $5 WHERE id = $1`,
		o.ID, o.Status, o.NumeroResposta, o.DataResposta, o.TecnicoResponsavelID); err != nil {
		return wrap("UpdateOficioCras", err)
	}
	return nil
}

// OficiosCrasNaArea lista os ofícios de um cidadão nas unidades da área.
func (r *Repository) OficiosCrasNaArea(ctx context.Context, db database.DBTX, cidadao uuid.UUID, area domain.Area) ([]domain.OficioCras, error) {
	rows, err := db.Query(ctx, `SELECT `+oficioCrasCols+` FROM sempras_cras_oficios o JOIN unidades u ON u.id = o.unidade_id
		WHERE o.cidadao_id = $4 AND `+areaSQL(1, 2, 3)+`
		ORDER BY o.data_recebimento DESC LIMIT 200`, append(areaArgs(area), cidadao)...)
	if err != nil {
		return nil, wrap("OficiosCrasNaArea", err)
	}
	return collect(rows, "OficiosCrasNaArea", scanOficioCras)
}

// --------------------------------------------------------------- Casa Abrigo

const acolhimentoCols = `a.id, a.cidadao_id, a.unidade_id, a.sexo, a.responsavel_nome, a.responsavel_nis, a.responsavel_telefone,
	a.responsavel_vinculo, a.responsavel_endereco, a.motivos, a.processo_judicial, a.guia_acolhimento, a.conselheiro_nome,
	a.conselheiro_telefone, a.situacao_juridica, a.historico_acolhido_anteriormente, a.historico_irmaos_outra_unidade,
	a.observacoes_gerais, a.status, a.motivo_desligamento, a.desligado_por, a.data_entrada, a.data_desligamento`

func scanAcolhimento(row pgx.Row) (domain.Acolhimento, error) {
	var a domain.Acolhimento
	err := row.Scan(&a.ID, &a.CidadaoID, &a.UnidadeID, &a.Sexo, &a.ResponsavelNome, &a.ResponsavelNIS, &a.ResponsavelTelefone,
		&a.ResponsavelVinculo, &a.ResponsavelEndereco, &a.Motivos, &a.ProcessoJudicial, &a.GuiaAcolhimento, &a.ConselheiroNome,
		&a.ConselheiroTelefone, &a.SituacaoJuridica, &a.HistoricoAcolhidoAnteriormente, &a.HistoricoIrmaosOutraUnidade,
		&a.Observacoes, &a.Status, &a.MotivoDesligamento, &a.DesligadoPor, &a.DataEntrada, &a.DataDesligamento)
	return a, err
}

// InsertAcolhimento grava um acolhimento recém-registrado (nasce 'acolhido').
func (r *Repository) InsertAcolhimento(ctx context.Context, db database.DBTX, a domain.Acolhimento, autor uuid.UUID) error {
	if _, err := db.Exec(ctx, `INSERT INTO sempras_casa_abrigo_acolhimentos
		(id, cidadao_id, unidade_id, sexo, responsavel_nome, responsavel_nis, responsavel_telefone, responsavel_vinculo,
		 responsavel_endereco, motivos, processo_judicial, guia_acolhimento, conselheiro_nome, conselheiro_telefone,
		 situacao_juridica, historico_acolhido_anteriormente, historico_irmaos_outra_unidade, observacoes_gerais,
		 status, registrado_por, data_entrada)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)`,
		a.ID, a.CidadaoID, a.UnidadeID, a.Sexo, a.ResponsavelNome, a.ResponsavelNIS, a.ResponsavelTelefone, a.ResponsavelVinculo,
		a.ResponsavelEndereco, a.Motivos, a.ProcessoJudicial, a.GuiaAcolhimento, a.ConselheiroNome, a.ConselheiroTelefone,
		a.SituacaoJuridica, a.HistoricoAcolhidoAnteriormente, a.HistoricoIrmaosOutraUnidade, a.Observacoes,
		a.Status, autor, a.DataEntrada); err != nil {
		return wrap("InsertAcolhimento", err)
	}
	return nil
}

// GetAcolhimento busca o acolhimento (forUpdate trava a linha no desligamento).
func (r *Repository) GetAcolhimento(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (domain.Acolhimento, error) {
	q := `SELECT ` + acolhimentoCols + ` FROM sempras_casa_abrigo_acolhimentos a WHERE a.id = $1`
	if forUpdate {
		q += ` FOR UPDATE`
	}
	a, err := scanAcolhimento(db.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Acolhimento{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Acolhimento{}, wrap("GetAcolhimento", err)
	}
	return a, nil
}

// UpdateAcolhimento grava a edição descritiva ou o desligamento.
func (r *Repository) UpdateAcolhimento(ctx context.Context, db database.DBTX, a domain.Acolhimento) error {
	if _, err := db.Exec(ctx, `UPDATE sempras_casa_abrigo_acolhimentos SET sexo = $2, responsavel_nome = $3,
		responsavel_nis = $4, responsavel_telefone = $5, responsavel_vinculo = $6, responsavel_endereco = $7, motivos = $8,
		processo_judicial = $9, guia_acolhimento = $10, conselheiro_nome = $11, conselheiro_telefone = $12,
		situacao_juridica = $13, historico_acolhido_anteriormente = $14, historico_irmaos_outra_unidade = $15,
		observacoes_gerais = $16, status = $17, motivo_desligamento = $18, desligado_por = $19, data_desligamento = $20
		WHERE id = $1`,
		a.ID, a.Sexo, a.ResponsavelNome, a.ResponsavelNIS, a.ResponsavelTelefone, a.ResponsavelVinculo, a.ResponsavelEndereco,
		a.Motivos, a.ProcessoJudicial, a.GuiaAcolhimento, a.ConselheiroNome, a.ConselheiroTelefone, a.SituacaoJuridica,
		a.HistoricoAcolhidoAnteriormente, a.HistoricoIrmaosOutraUnidade, a.Observacoes, a.Status, a.MotivoDesligamento,
		a.DesligadoPor, a.DataDesligamento); err != nil {
		return wrap("UpdateAcolhimento", err)
	}
	return nil
}

// AcolhimentosNaArea lista os acolhimentos do munícipe nas unidades da área de quem consulta.
func (r *Repository) AcolhimentosNaArea(ctx context.Context, db database.DBTX, cidadao uuid.UUID, area domain.Area) ([]domain.Acolhimento, error) {
	rows, err := db.Query(ctx, `SELECT `+acolhimentoCols+` FROM sempras_casa_abrigo_acolhimentos a JOIN unidades u ON u.id = a.unidade_id
		WHERE a.cidadao_id = $4 AND `+areaSQL(1, 2, 3)+`
		ORDER BY a.data_entrada DESC LIMIT 200`, append(areaArgs(area), cidadao)...)
	if err != nil {
		return nil, wrap("AcolhimentosNaArea", err)
	}
	return collect(rows, "AcolhimentosNaArea", scanAcolhimento)
}

const irmaoCols = `i.id, i.acolhimento_id, i.nome, i.data_nascimento, i.esta_acolhido, i.unidade_descricao, i.observacao, i.created_at`

func scanIrmao(row pgx.Row) (domain.Irmao, error) {
	var i domain.Irmao
	err := row.Scan(&i.ID, &i.AcolhimentoID, &i.Nome, &i.DataNascimento, &i.EstaAcolhido, &i.UnidadeDescricao, &i.Observacao, &i.CreatedAt)
	return i, err
}

// InsertIrmao grava um irmão do acolhido (RN-07).
func (r *Repository) InsertIrmao(ctx context.Context, db database.DBTX, i domain.Irmao) error {
	if _, err := db.Exec(ctx, `INSERT INTO sempras_casa_abrigo_irmaos
		(id, acolhimento_id, nome, data_nascimento, esta_acolhido, unidade_descricao, observacao, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		i.ID, i.AcolhimentoID, i.Nome, i.DataNascimento, i.EstaAcolhido, i.UnidadeDescricao, i.Observacao, i.CreatedAt); err != nil {
		return wrap("InsertIrmao", err)
	}
	return nil
}

// ListIrmaos lista os irmãos registrados de um acolhimento.
func (r *Repository) ListIrmaos(ctx context.Context, db database.DBTX, acolhimento uuid.UUID) ([]domain.Irmao, error) {
	rows, err := db.Query(ctx, `SELECT `+irmaoCols+` FROM sempras_casa_abrigo_irmaos i
		WHERE i.acolhimento_id = $1 ORDER BY i.created_at`, acolhimento)
	if err != nil {
		return nil, wrap("ListIrmaos", err)
	}
	return collect(rows, "ListIrmaos", scanIrmao)
}

// GetIrmao busca um irmão (confere a quem pertence antes de remover).
func (r *Repository) GetIrmao(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Irmao, error) {
	i, err := scanIrmao(db.QueryRow(ctx, `SELECT `+irmaoCols+` FROM sempras_casa_abrigo_irmaos i WHERE i.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Irmao{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Irmao{}, wrap("GetIrmao", err)
	}
	return i, nil
}

// DeleteIrmao remove um irmão (cadastrado errado, por exemplo).
func (r *Repository) DeleteIrmao(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if _, err := db.Exec(ctx, `DELETE FROM sempras_casa_abrigo_irmaos WHERE id = $1`, id); err != nil {
		return wrap("DeleteIrmao", err)
	}
	return nil
}

const diagnosticoCols = `d.id, d.acolhimento_id, d.categoria, d.diagnostico, d.descricao, d.created_at`

func scanDiagnostico(row pgx.Row) (domain.Diagnostico, error) {
	var d domain.Diagnostico
	err := row.Scan(&d.ID, &d.AcolhimentoID, &d.Categoria, &d.Diagnostico, &d.Descricao, &d.CreatedAt)
	return d, err
}

// InsertDiagnostico grava um diagnóstico do acolhido (RN-12).
func (r *Repository) InsertDiagnostico(ctx context.Context, db database.DBTX, d domain.Diagnostico) error {
	if _, err := db.Exec(ctx, `INSERT INTO sempras_casa_abrigo_diagnosticos
		(id, acolhimento_id, categoria, diagnostico, descricao, created_at) VALUES ($1, $2, $3, $4, $5, $6)`,
		d.ID, d.AcolhimentoID, d.Categoria, d.Diagnostico, d.Descricao, d.CreatedAt); err != nil {
		return wrap("InsertDiagnostico", err)
	}
	return nil
}

// ListDiagnosticos lista os diagnósticos registrados de um acolhimento.
func (r *Repository) ListDiagnosticos(ctx context.Context, db database.DBTX, acolhimento uuid.UUID) ([]domain.Diagnostico, error) {
	rows, err := db.Query(ctx, `SELECT `+diagnosticoCols+` FROM sempras_casa_abrigo_diagnosticos d
		WHERE d.acolhimento_id = $1 ORDER BY d.created_at`, acolhimento)
	if err != nil {
		return nil, wrap("ListDiagnosticos", err)
	}
	return collect(rows, "ListDiagnosticos", scanDiagnostico)
}

// GetDiagnostico busca um diagnóstico (confere a quem pertence antes de remover).
func (r *Repository) GetDiagnostico(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Diagnostico, error) {
	d, err := scanDiagnostico(db.QueryRow(ctx, `SELECT `+diagnosticoCols+` FROM sempras_casa_abrigo_diagnosticos d WHERE d.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Diagnostico{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Diagnostico{}, wrap("GetDiagnostico", err)
	}
	return d, nil
}

// DeleteDiagnostico remove um diagnóstico (cadastrado errado, por exemplo).
func (r *Repository) DeleteDiagnostico(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if _, err := db.Exec(ctx, `DELETE FROM sempras_casa_abrigo_diagnosticos WHERE id = $1`, id); err != nil {
		return wrap("DeleteDiagnostico", err)
	}
	return nil
}

// --------------------------------------------- Casa Abrigo: visitantes

const visitanteCols = `v.id, v.acolhimento_id, v.nome_visitante, v.documento_identidade, v.parentesco_vinculo,
	v.data_visita, COALESCE(to_char(v.hora_entrada, 'HH24:MI'), ''), COALESCE(to_char(v.hora_saida, 'HH24:MI'), ''),
	v.observacoes, v.created_at`

func scanVisitante(row pgx.Row) (domain.Visitante, error) {
	var v domain.Visitante
	err := row.Scan(&v.ID, &v.AcolhimentoID, &v.NomeVisitante, &v.DocumentoIdentidade, &v.ParentescoVinculo,
		&v.DataVisita, &v.HoraEntrada, &v.HoraSaida, &v.Observacoes, &v.CreatedAt)
	return v, err
}

// InsertVisitante grava uma visita recebida pelo acolhido.
func (r *Repository) InsertVisitante(ctx context.Context, db database.DBTX, v domain.Visitante, autor uuid.UUID) error {
	if _, err := db.Exec(ctx, `INSERT INTO sempras_casa_abrigo_visitantes
		(id, acolhimento_id, nome_visitante, documento_identidade, parentesco_vinculo, data_visita, hora_entrada, hora_saida,
		 observacoes, registrado_por, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, '')::time, NULLIF($8, '')::time, $9, $10, $11)`,
		v.ID, v.AcolhimentoID, v.NomeVisitante, v.DocumentoIdentidade, v.ParentescoVinculo, v.DataVisita, v.HoraEntrada,
		v.HoraSaida, v.Observacoes, autor, v.CreatedAt); err != nil {
		return wrap("InsertVisitante", err)
	}
	return nil
}

// ListVisitantes lista as visitas registradas de um acolhimento.
func (r *Repository) ListVisitantes(ctx context.Context, db database.DBTX, acolhimento uuid.UUID) ([]domain.Visitante, error) {
	rows, err := db.Query(ctx, `SELECT `+visitanteCols+` FROM sempras_casa_abrigo_visitantes v
		WHERE v.acolhimento_id = $1 ORDER BY v.data_visita DESC, v.created_at DESC`, acolhimento)
	if err != nil {
		return nil, wrap("ListVisitantes", err)
	}
	return collect(rows, "ListVisitantes", scanVisitante)
}

// GetVisitante busca uma visita (confere a quem pertence antes de remover).
func (r *Repository) GetVisitante(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Visitante, error) {
	v, err := scanVisitante(db.QueryRow(ctx, `SELECT `+visitanteCols+` FROM sempras_casa_abrigo_visitantes v WHERE v.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Visitante{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Visitante{}, wrap("GetVisitante", err)
	}
	return v, nil
}

// DeleteVisitante remove uma visita (cadastrada errada, por exemplo).
func (r *Repository) DeleteVisitante(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if _, err := db.Exec(ctx, `DELETE FROM sempras_casa_abrigo_visitantes WHERE id = $1`, id); err != nil {
		return wrap("DeleteVisitante", err)
	}
	return nil
}

// -------------------------------------------------- Casa Abrigo: termos

const termoCols = `t.id, t.acolhimento_id, t.tipo_termo, t.responsavel_retirada, t.parentesco_responsavel,
	t.motivo_observacao, t.data_termo, t.created_at`

func scanTermo(row pgx.Row) (domain.Termo, error) {
	var t domain.Termo
	err := row.Scan(&t.ID, &t.AcolhimentoID, &t.TipoTermo, &t.ResponsavelRetirada, &t.ParentescoResponsavel,
		&t.MotivoObservacao, &t.DataTermo, &t.CreatedAt)
	return t, err
}

// InsertTermo grava um termo de advertência, passeio ou saída.
func (r *Repository) InsertTermo(ctx context.Context, db database.DBTX, t domain.Termo, autor uuid.UUID) error {
	if _, err := db.Exec(ctx, `INSERT INTO sempras_casa_abrigo_termos
		(id, acolhimento_id, tipo_termo, responsavel_retirada, parentesco_responsavel, motivo_observacao, data_termo, registrado_por, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		t.ID, t.AcolhimentoID, t.TipoTermo, t.ResponsavelRetirada, t.ParentescoResponsavel, t.MotivoObservacao,
		t.DataTermo, autor, t.CreatedAt); err != nil {
		return wrap("InsertTermo", err)
	}
	return nil
}

// ListTermos lista os termos registrados de um acolhimento.
func (r *Repository) ListTermos(ctx context.Context, db database.DBTX, acolhimento uuid.UUID) ([]domain.Termo, error) {
	rows, err := db.Query(ctx, `SELECT `+termoCols+` FROM sempras_casa_abrigo_termos t
		WHERE t.acolhimento_id = $1 ORDER BY t.data_termo DESC, t.created_at DESC`, acolhimento)
	if err != nil {
		return nil, wrap("ListTermos", err)
	}
	return collect(rows, "ListTermos", scanTermo)
}

// GetTermo busca um termo (confere a quem pertence antes de remover).
func (r *Repository) GetTermo(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Termo, error) {
	t, err := scanTermo(db.QueryRow(ctx, `SELECT `+termoCols+` FROM sempras_casa_abrigo_termos t WHERE t.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Termo{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Termo{}, wrap("GetTermo", err)
	}
	return t, nil
}

// DeleteTermo remove um termo (cadastrado errado, por exemplo).
func (r *Repository) DeleteTermo(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if _, err := db.Exec(ctx, `DELETE FROM sempras_casa_abrigo_termos WHERE id = $1`, id); err != nil {
		return wrap("DeleteTermo", err)
	}
	return nil
}

// ------------------------------------------------------ Casa Abrigo: PIA

const piaFichaCols = `p.acolhimento_id, p.matricula_escolar, p.serie_escolar, p.escola, p.medicamentos, p.alergias,
	p.vacinacao_atualizada, p.cartao_sus, p.contato_familiar, p.visitas_autorizadas, p.historico_familiar,
	p.atividades_manha, p.atividades_tarde, p.atividades_noite, p.atividades_fim_semana, p.atualizado_por, p.atualizado_em`

func scanPiaFicha(row pgx.Row) (domain.PiaFicha, error) {
	var p domain.PiaFicha
	err := row.Scan(&p.AcolhimentoID, &p.MatriculaEscolar, &p.SerieEscolar, &p.Escola, &p.Medicamentos, &p.Alergias,
		&p.VacinacaoAtualizada, &p.CartaoSUS, &p.ContatoFamiliar, &p.VisitasAutorizadas, &p.HistoricoFamiliar,
		&p.AtividadesManha, &p.AtividadesTarde, &p.AtividadesNoite, &p.AtividadesFimSemana, &p.AtualizadoPor, &p.AtualizadoEm)
	return p, err
}

// UpsertPiaFicha grava ou atualiza a ficha 1:1 do PIA (saúde/família/rotina).
func (r *Repository) UpsertPiaFicha(ctx context.Context, db database.DBTX, p domain.PiaFicha, autor uuid.UUID) error {
	if _, err := db.Exec(ctx, `INSERT INTO sempras_casa_abrigo_pia
		(acolhimento_id, matricula_escolar, serie_escolar, escola, medicamentos, alergias, vacinacao_atualizada,
		 cartao_sus, contato_familiar, visitas_autorizadas, historico_familiar, atividades_manha, atividades_tarde,
		 atividades_noite, atividades_fim_semana, atualizado_por, atualizado_em)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		ON CONFLICT (acolhimento_id) DO UPDATE SET matricula_escolar = $2, serie_escolar = $3, escola = $4,
			medicamentos = $5, alergias = $6, vacinacao_atualizada = $7, cartao_sus = $8, contato_familiar = $9,
			visitas_autorizadas = $10, historico_familiar = $11, atividades_manha = $12, atividades_tarde = $13,
			atividades_noite = $14, atividades_fim_semana = $15, atualizado_por = $16, atualizado_em = $17`,
		p.AcolhimentoID, p.MatriculaEscolar, p.SerieEscolar, p.Escola, p.Medicamentos, p.Alergias, p.VacinacaoAtualizada,
		p.CartaoSUS, p.ContatoFamiliar, p.VisitasAutorizadas, p.HistoricoFamiliar, p.AtividadesManha, p.AtividadesTarde,
		p.AtividadesNoite, p.AtividadesFimSemana, autor, p.AtualizadoEm); err != nil {
		return wrap("UpsertPiaFicha", err)
	}
	return nil
}

// GetPiaFicha busca a ficha do PIA de um acolhimento (ErrNotFound se ainda não foi preenchida).
func (r *Repository) GetPiaFicha(ctx context.Context, db database.DBTX, acolhimento uuid.UUID) (domain.PiaFicha, error) {
	p, err := scanPiaFicha(db.QueryRow(ctx, `SELECT `+piaFichaCols+` FROM sempras_casa_abrigo_pia p WHERE p.acolhimento_id = $1`, acolhimento))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PiaFicha{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.PiaFicha{}, wrap("GetPiaFicha", err)
	}
	return p, nil
}

const piaMetaCols = `m.id, m.acolhimento_id, m.tipo, m.descricao, m.providencia, m.responsavel, m.prazo, m.concluida, m.created_at`

func scanPiaMeta(row pgx.Row) (domain.PiaMeta, error) {
	var m domain.PiaMeta
	err := row.Scan(&m.ID, &m.AcolhimentoID, &m.Tipo, &m.Descricao, &m.Providencia, &m.Responsavel, &m.Prazo, &m.Concluida, &m.CreatedAt)
	return m, err
}

// InsertPiaMeta grava uma meta do plano ou da intervenção do PIA.
func (r *Repository) InsertPiaMeta(ctx context.Context, db database.DBTX, m domain.PiaMeta, autor uuid.UUID) error {
	if _, err := db.Exec(ctx, `INSERT INTO sempras_casa_abrigo_pia_metas
		(id, acolhimento_id, tipo, descricao, providencia, responsavel, prazo, registrado_por, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		m.ID, m.AcolhimentoID, m.Tipo, m.Descricao, m.Providencia, m.Responsavel, m.Prazo, autor, m.CreatedAt); err != nil {
		return wrap("InsertPiaMeta", err)
	}
	return nil
}

// ListPiaMetas lista as metas do PIA de um acolhimento.
func (r *Repository) ListPiaMetas(ctx context.Context, db database.DBTX, acolhimento uuid.UUID) ([]domain.PiaMeta, error) {
	rows, err := db.Query(ctx, `SELECT `+piaMetaCols+` FROM sempras_casa_abrigo_pia_metas m
		WHERE m.acolhimento_id = $1 ORDER BY m.created_at`, acolhimento)
	if err != nil {
		return nil, wrap("ListPiaMetas", err)
	}
	return collect(rows, "ListPiaMetas", scanPiaMeta)
}

// GetPiaMeta busca uma meta (confere a quem pertence antes de concluir/remover).
func (r *Repository) GetPiaMeta(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.PiaMeta, error) {
	m, err := scanPiaMeta(db.QueryRow(ctx, `SELECT `+piaMetaCols+` FROM sempras_casa_abrigo_pia_metas m WHERE m.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PiaMeta{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.PiaMeta{}, wrap("GetPiaMeta", err)
	}
	return m, nil
}

// UpdatePiaMeta grava a conclusão (ou não) de uma meta.
func (r *Repository) UpdatePiaMeta(ctx context.Context, db database.DBTX, m domain.PiaMeta) error {
	if _, err := db.Exec(ctx, `UPDATE sempras_casa_abrigo_pia_metas SET concluida = $2 WHERE id = $1`, m.ID, m.Concluida); err != nil {
		return wrap("UpdatePiaMeta", err)
	}
	return nil
}

// DeletePiaMeta remove uma meta (cadastrada errada, por exemplo).
func (r *Repository) DeletePiaMeta(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if _, err := db.Exec(ctx, `DELETE FROM sempras_casa_abrigo_pia_metas WHERE id = $1`, id); err != nil {
		return wrap("DeletePiaMeta", err)
	}
	return nil
}

// ------------------------------------------------- Casa Abrigo: documentos

const documentoCols = `d.id, d.acolhimento_id, d.tipo, d.nome_original, d.object_key, d.content_type, d.size_bytes,
	d.status, d.created_at`

func scanDocumentoCasaAbrigo(row pgx.Row) (domain.DocumentoCasaAbrigo, error) {
	var d domain.DocumentoCasaAbrigo
	err := row.Scan(&d.ID, &d.AcolhimentoID, &d.Tipo, &d.NomeOriginal, &d.ObjectKey, &d.ContentType, &d.SizeBytes,
		&d.Status, &d.CreatedAt)
	return d, err
}

// InsertDocumento grava o metadado pendente (o navegador ainda está
// enviando o conteúdo ao object storage).
func (r *Repository) InsertDocumento(ctx context.Context, db database.DBTX, d domain.DocumentoCasaAbrigo, autor uuid.UUID) error {
	if _, err := db.Exec(ctx, `INSERT INTO sempras_casa_abrigo_documentos
		(id, acolhimento_id, tipo, nome_original, object_key, content_type, size_bytes, status, registrado_por, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		d.ID, d.AcolhimentoID, d.Tipo, d.NomeOriginal, d.ObjectKey, d.ContentType, d.SizeBytes, d.Status, autor, d.CreatedAt); err != nil {
		return wrap("InsertDocumento", err)
	}
	return nil
}

// GetDocumento busca o documento (confere o dono/tamanho antes de confirmar ou remover).
func (r *Repository) GetDocumento(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.DocumentoCasaAbrigo, error) {
	d, err := scanDocumentoCasaAbrigo(db.QueryRow(ctx, `SELECT `+documentoCols+` FROM sempras_casa_abrigo_documentos d WHERE d.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DocumentoCasaAbrigo{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.DocumentoCasaAbrigo{}, wrap("GetDocumento", err)
	}
	return d, nil
}

// UpdateDocumentoStatus grava a confirmação do upload (tamanho e tipo
// reais, conferidos no object storage — o que o navegador declarou não é confiável).
func (r *Repository) UpdateDocumentoStatus(ctx context.Context, db database.DBTX, id uuid.UUID, status string, sizeBytes int64, contentType string) error {
	if _, err := db.Exec(ctx, `UPDATE sempras_casa_abrigo_documentos SET status = $2, size_bytes = $3, content_type = $4 WHERE id = $1`,
		id, status, sizeBytes, contentType); err != nil {
		return wrap("UpdateDocumentoStatus", err)
	}
	return nil
}

// ListDocumentos lista os documentos de um acolhimento.
func (r *Repository) ListDocumentos(ctx context.Context, db database.DBTX, acolhimento uuid.UUID) ([]domain.DocumentoCasaAbrigo, error) {
	rows, err := db.Query(ctx, `SELECT `+documentoCols+` FROM sempras_casa_abrigo_documentos d
		WHERE d.acolhimento_id = $1 ORDER BY d.created_at DESC`, acolhimento)
	if err != nil {
		return nil, wrap("ListDocumentos", err)
	}
	return collect(rows, "ListDocumentos", scanDocumentoCasaAbrigo)
}

// DeleteDocumento remove o metadado (a remoção do objeto no storage é
// feita pela aplicação, fora da transação, best-effort).
func (r *Repository) DeleteDocumento(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if _, err := db.Exec(ctx, `DELETE FROM sempras_casa_abrigo_documentos WHERE id = $1`, id); err != nil {
		return wrap("DeleteDocumento", err)
	}
	return nil
}

// ------------------------------------------------------------------- Centro POP

const centroPopPerfilCols = `p.cidadao_id, p.situacao, p.tempo_rua_anos, p.motivo_vulnerabilidade, p.possui_doenca,
	p.doenca_descricao, p.usa_medicacao, p.medicacao_descricao, p.uso_drogas, p.drogas_descricao, p.atualizado_por, p.atualizado_em`

func scanCentroPopPerfil(row pgx.Row) (domain.CentroPopPerfil, error) {
	var p domain.CentroPopPerfil
	err := row.Scan(&p.CidadaoID, &p.Situacao, &p.TempoRuaAnos, &p.MotivoVulnerabilidade, &p.PossuiDoenca,
		&p.DoencaDescricao, &p.UsaMedicacao, &p.MedicacaoDescricao, &p.UsoDrogas, &p.DrogasDescricao, &p.AtualizadoPor, &p.AtualizadoEm)
	return p, err
}

// UpsertCentroPopPerfil grava o perfil de rua (um por cidadão).
func (r *Repository) UpsertCentroPopPerfil(ctx context.Context, db database.DBTX, p domain.CentroPopPerfil, autor uuid.UUID) error {
	if _, err := db.Exec(ctx, `INSERT INTO sempras_centro_pop_perfis
		(cidadao_id, situacao, tempo_rua_anos, motivo_vulnerabilidade, possui_doenca, doenca_descricao,
		 usa_medicacao, medicacao_descricao, uso_drogas, drogas_descricao, atualizado_por, atualizado_em)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (cidadao_id) DO UPDATE SET situacao = EXCLUDED.situacao, tempo_rua_anos = EXCLUDED.tempo_rua_anos,
			motivo_vulnerabilidade = EXCLUDED.motivo_vulnerabilidade, possui_doenca = EXCLUDED.possui_doenca,
			doenca_descricao = EXCLUDED.doenca_descricao, usa_medicacao = EXCLUDED.usa_medicacao,
			medicacao_descricao = EXCLUDED.medicacao_descricao, uso_drogas = EXCLUDED.uso_drogas,
			drogas_descricao = EXCLUDED.drogas_descricao, atualizado_por = EXCLUDED.atualizado_por, atualizado_em = EXCLUDED.atualizado_em`,
		p.CidadaoID, p.Situacao, p.TempoRuaAnos, p.MotivoVulnerabilidade, p.PossuiDoenca, p.DoencaDescricao,
		p.UsaMedicacao, p.MedicacaoDescricao, p.UsoDrogas, p.DrogasDescricao, autor, p.AtualizadoEm); err != nil {
		return wrap("UpsertCentroPopPerfil", err)
	}
	return nil
}

// GetCentroPopPerfil busca o perfil de rua (ErrNotFound se ainda não foi preenchido).
func (r *Repository) GetCentroPopPerfil(ctx context.Context, db database.DBTX, cidadao uuid.UUID) (domain.CentroPopPerfil, error) {
	p, err := scanCentroPopPerfil(db.QueryRow(ctx, `SELECT `+centroPopPerfilCols+` FROM sempras_centro_pop_perfis p WHERE p.cidadao_id = $1`, cidadao))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CentroPopPerfil{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.CentroPopPerfil{}, wrap("GetCentroPopPerfil", err)
	}
	return p, nil
}

const encaminhamentoExternoCols = `e.id, e.cidadao_id, e.unidade_id, e.destino, e.prioridade, e.motivo, e.status,
	e.observacoes, e.tecnico_id, e.created_at, e.atualizado_em`

func scanEncaminhamentoExterno(row pgx.Row) (domain.EncaminhamentoExterno, error) {
	var e domain.EncaminhamentoExterno
	err := row.Scan(&e.ID, &e.CidadaoID, &e.UnidadeID, &e.Destino, &e.Prioridade, &e.Motivo, &e.Status,
		&e.Observacoes, &e.TecnicoID, &e.CreatedAt, &e.AtualizadoEm)
	return e, err
}

// InsertEncaminhamentoExterno grava um encaminhamento pra fora do sistema (nasce 'pendente').
func (r *Repository) InsertEncaminhamentoExterno(ctx context.Context, db database.DBTX, e domain.EncaminhamentoExterno, autor uuid.UUID) error {
	if _, err := db.Exec(ctx, `INSERT INTO sempras_encaminhamentos_externos
		(id, cidadao_id, unidade_id, destino, prioridade, motivo, status, registrado_por)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		e.ID, e.CidadaoID, e.UnidadeID, e.Destino, e.Prioridade, e.Motivo, e.Status, autor); err != nil {
		return wrap("InsertEncaminhamentoExterno", err)
	}
	return nil
}

// GetEncaminhamentoExterno busca o encaminhamento (forUpdate trava a linha na transição).
func (r *Repository) GetEncaminhamentoExterno(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (domain.EncaminhamentoExterno, error) {
	q := `SELECT ` + encaminhamentoExternoCols + ` FROM sempras_encaminhamentos_externos e WHERE e.id = $1`
	if forUpdate {
		q += ` FOR UPDATE`
	}
	e, err := scanEncaminhamentoExterno(db.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.EncaminhamentoExterno{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.EncaminhamentoExterno{}, wrap("GetEncaminhamentoExterno", err)
	}
	return e, nil
}

// UpdateEncaminhamentoExterno grava a transição de status.
func (r *Repository) UpdateEncaminhamentoExterno(ctx context.Context, db database.DBTX, e domain.EncaminhamentoExterno) error {
	if _, err := db.Exec(ctx, `UPDATE sempras_encaminhamentos_externos SET status = $2, observacoes = $3,
		tecnico_id = $4, atualizado_em = $5 WHERE id = $1`,
		e.ID, e.Status, e.Observacoes, e.TecnicoID, e.AtualizadoEm); err != nil {
		return wrap("UpdateEncaminhamentoExterno", err)
	}
	return nil
}

// EncaminhamentosExternosNaArea lista os encaminhamentos externos de um cidadão nas unidades da área.
func (r *Repository) EncaminhamentosExternosNaArea(ctx context.Context, db database.DBTX, cidadao uuid.UUID, area domain.Area) ([]domain.EncaminhamentoExterno, error) {
	rows, err := db.Query(ctx, `SELECT `+encaminhamentoExternoCols+` FROM sempras_encaminhamentos_externos e JOIN unidades u ON u.id = e.unidade_id
		WHERE e.cidadao_id = $4 AND `+areaSQL(1, 2, 3)+`
		ORDER BY e.created_at DESC LIMIT 200`, append(areaArgs(area), cidadao)...)
	if err != nil {
		return nil, wrap("EncaminhamentosExternosNaArea", err)
	}
	return collect(rows, "EncaminhamentosExternosNaArea", scanEncaminhamentoExterno)
}

const documentoCentroPopCols = `d.id, d.cidadao_id, d.unidade_id, d.tipo, d.nome_original, d.object_key, d.content_type, d.size_bytes,
	d.status, d.created_at`

func scanDocumentoCentroPop(row pgx.Row) (domain.DocumentoCentroPop, error) {
	var d domain.DocumentoCentroPop
	err := row.Scan(&d.ID, &d.CidadaoID, &d.UnidadeID, &d.Tipo, &d.NomeOriginal, &d.ObjectKey, &d.ContentType, &d.SizeBytes,
		&d.Status, &d.CreatedAt)
	return d, err
}

// InsertDocumentoCentroPop grava o metadado pendente.
func (r *Repository) InsertDocumentoCentroPop(ctx context.Context, db database.DBTX, d domain.DocumentoCentroPop, autor uuid.UUID) error {
	if _, err := db.Exec(ctx, `INSERT INTO sempras_centro_pop_documentos
		(id, cidadao_id, unidade_id, tipo, nome_original, object_key, content_type, size_bytes, status, registrado_por, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		d.ID, d.CidadaoID, d.UnidadeID, d.Tipo, d.NomeOriginal, d.ObjectKey, d.ContentType, d.SizeBytes, d.Status, autor, d.CreatedAt); err != nil {
		return wrap("InsertDocumentoCentroPop", err)
	}
	return nil
}

// GetDocumentoCentroPop busca o documento.
func (r *Repository) GetDocumentoCentroPop(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.DocumentoCentroPop, error) {
	d, err := scanDocumentoCentroPop(db.QueryRow(ctx, `SELECT `+documentoCentroPopCols+` FROM sempras_centro_pop_documentos d WHERE d.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DocumentoCentroPop{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.DocumentoCentroPop{}, wrap("GetDocumentoCentroPop", err)
	}
	return d, nil
}

// UpdateDocumentoCentroPopStatus grava a confirmação do upload.
func (r *Repository) UpdateDocumentoCentroPopStatus(ctx context.Context, db database.DBTX, id uuid.UUID, status string, sizeBytes int64, contentType string) error {
	if _, err := db.Exec(ctx, `UPDATE sempras_centro_pop_documentos SET status = $2, size_bytes = $3, content_type = $4 WHERE id = $1`,
		id, status, sizeBytes, contentType); err != nil {
		return wrap("UpdateDocumentoCentroPopStatus", err)
	}
	return nil
}

// ListDocumentosCentroPop lista os documentos de um cidadão.
func (r *Repository) ListDocumentosCentroPop(ctx context.Context, db database.DBTX, cidadao uuid.UUID) ([]domain.DocumentoCentroPop, error) {
	rows, err := db.Query(ctx, `SELECT `+documentoCentroPopCols+` FROM sempras_centro_pop_documentos d
		WHERE d.cidadao_id = $1 ORDER BY d.created_at DESC`, cidadao)
	if err != nil {
		return nil, wrap("ListDocumentosCentroPop", err)
	}
	return collect(rows, "ListDocumentosCentroPop", scanDocumentoCentroPop)
}

// DeleteDocumentoCentroPop remove o metadado.
func (r *Repository) DeleteDocumentoCentroPop(ctx context.Context, db database.DBTX, id uuid.UUID) error {
	if _, err := db.Exec(ctx, `DELETE FROM sempras_centro_pop_documentos WHERE id = $1`, id); err != nil {
		return wrap("DeleteDocumentoCentroPop", err)
	}
	return nil
}
