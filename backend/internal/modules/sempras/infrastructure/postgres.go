// Package infrastructure implementa a persistência do SEMPRAS em
// PostgreSQL (queries parametrizadas via pgx). As listagens recebem a área
// de quem consulta e a aplicam no SQL (ADR 013: CoverageOf).
package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
	COALESCE(to_char(c.data_nascimento, 'YYYY-MM-DD'), ''), c.nome_mae, c.telefone, c.endereco, c.created_at, c.updated_at`

func scanCidadao(row pgx.Row) (domain.Cidadao, error) {
	var c domain.Cidadao
	err := row.Scan(&c.ID, &c.Nome, &c.NomeSocial, &c.CPF, &c.NIS, &c.DataNascimento, &c.NomeMae, &c.Telefone,
		&c.Endereco, &c.CreatedAt, &c.UpdatedAt)
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
		(id, nome, nome_social, cpf, nis, data_nascimento, nome_mae, telefone, endereco, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, '')::date, $7, $8, $9, $10, $11, $11)`,
		c.ID, c.Nome, c.NomeSocial, c.CPF, c.NIS, c.DataNascimento, c.NomeMae, c.Telefone, c.Endereco, autor, c.CreatedAt)
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
		nis = NULLIF($5, ''), data_nascimento = NULLIF($6, '')::date, nome_mae = $7, telefone = $8, endereco = $9
		WHERE id = $1`,
		c.ID, c.Nome, c.NomeSocial, c.CPF, c.NIS, c.DataNascimento, c.NomeMae, c.Telefone, c.Endereco)
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
