// Package infrastructure implementa o repositório do Trâmite.
package infrastructure

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/tramite/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
)

// Repository implementa domain.Repository.
type Repository struct{}

// NewRepository cria o repositório.
func NewRepository() *Repository { return &Repository{} }

var _ domain.Repository = (*Repository)(nil)

func wrap(err error) error {
	if err == nil {
		return nil
	}
	if database.IsNoRows(err) {
		return domain.ErrNotFound
	}
	return fmt.Errorf("tramite: %w", err)
}

func (r *Repository) Tipos(ctx context.Context, db database.DBTX) ([]domain.Tipo, error) {
	rows, err := db.Query(ctx, `SELECT id, slug, nome, descricao, ativo FROM tramite_tipos WHERE ativo ORDER BY nome`)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	out := []domain.Tipo{}
	for rows.Next() {
		var t domain.Tipo
		if err := rows.Scan(&t.ID, &t.Slug, &t.Nome, &t.Descricao, &t.Ativo); err != nil {
			return nil, wrap(err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// TipoAtivo reporta se o tipo existe e aceita novos processos. FOR SHARE:
// ninguém exclui o tipo entre esta checagem e a gravação do processo.
func (r *Repository) TipoAtivo(ctx context.Context, db database.DBTX, id uuid.UUID) (bool, error) {
	var ok bool
	err := db.QueryRow(ctx, `SELECT COALESCE((SELECT ativo FROM tramite_tipos WHERE id = $1 FOR SHARE), false)`, id).Scan(&ok)
	return ok, wrap(err)
}

// UnidadeAtiva reporta se a unidade existe e ela e a entidade estão ativas
// (docs/REGRAS_DE_NEGOCIO.md §2). FOR SHARE: ninguém exclui nem desativa a
// unidade entre esta checagem e a gravação.
func (r *Repository) UnidadeAtiva(ctx context.Context, db database.DBTX, id uuid.UUID) (bool, error) {
	var ok bool
	err := db.QueryRow(ctx, `SELECT COALESCE((SELECT u.ativo AND e.ativo FROM unidades u JOIN entidades e ON e.id = u.entidade_id
		WHERE u.id = $1 FOR SHARE OF u, e), false)`, id).Scan(&ok)
	return ok, wrap(err)
}

// NextNumero reserva o próximo sequencial do ano sem lacunas (a linha do
// ano fica travada até o fim da transação de abertura).
func (r *Repository) NextNumero(ctx context.Context, db database.DBTX, ano int) (int, error) {
	var n int
	err := db.QueryRow(ctx, `
		INSERT INTO tramite_numeracao (ano, ultimo) VALUES ($1, 1)
		ON CONFLICT (ano) DO UPDATE SET ultimo = tramite_numeracao.ultimo + 1
		RETURNING ultimo`, ano).Scan(&n)
	return n, wrap(err)
}

func (r *Repository) Insert(ctx context.Context, db database.DBTX, p domain.Processo) error {
	_, err := db.Exec(ctx, `INSERT INTO tramite_processos (id, numero, tipo_id, assunto, interessado, descricao, sigilo, status,
		unidade_origem_id, unidade_atual_id, created_by, atlas_procedimento_id, codigo_ttdd)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$9,$10,$11,NULLIF($12,''))`,
		p.ID, p.Numero, p.TipoID, p.Assunto, p.Interessado, p.Descricao, p.Sigilo, p.Status, p.UnidadeOrigemID, p.CreatedBy,
		p.AtlasProcedimentoID, p.CodigoTTDD)
	return wrap(err)
}

const procCols = `p.id, p.numero, p.tipo_id, t.nome, p.assunto, p.interessado, p.descricao, p.sigilo, p.status,
	p.unidade_origem_id, uo.nome, p.unidade_atual_id, ua.nome, p.created_by, COALESCE(NULLIF(u.display_name,''), u.username, ''),
	p.created_at, p.updated_at, p.concluido_at, p.atlas_procedimento_id, COALESCE(p.codigo_ttdd, '')`

const procFrom = ` FROM tramite_processos p
	JOIN tramite_tipos t ON t.id = p.tipo_id
	JOIN unidades uo ON uo.id = p.unidade_origem_id
	JOIN unidades ua ON ua.id = p.unidade_atual_id
	LEFT JOIN users u ON u.id = p.created_by `

func scanProc(row interface{ Scan(...any) error }) (domain.Processo, error) {
	var p domain.Processo
	err := row.Scan(&p.ID, &p.Numero, &p.TipoID, &p.Tipo, &p.Assunto, &p.Interessado, &p.Descricao, &p.Sigilo, &p.Status,
		&p.UnidadeOrigemID, &p.UnidadeOrigem, &p.UnidadeAtualID, &p.UnidadeAtual, &p.CreatedBy, &p.CreatedByName,
		&p.CreatedAt, &p.UpdatedAt, &p.ConcluidoAt, &p.AtlasProcedimentoID, &p.CodigoTTDD)
	return p, err
}

func (r *Repository) Get(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (domain.Processo, error) {
	lock := ""
	if forUpdate {
		lock = " FOR UPDATE OF p"
	}
	p, err := scanProc(db.QueryRow(ctx, `SELECT `+procCols+procFrom+` WHERE p.id = $1`+lock, id))
	return p, wrap(err)
}

func (r *Repository) HasGrant(ctx context.Context, db database.DBTX, processoID, userID uuid.UUID) (bool, error) {
	var ok bool
	err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM tramite_acessos WHERE processo_id = $1 AND user_id = $2)`, processoID, userID).Scan(&ok)
	return ok, wrap(err)
}

func uuidStrings(ids []uuid.UUID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.String())
	}
	return out
}

func unidadesOf(identity auth.Identity) []string {
	out := []string{}
	for _, s := range identity.Scopes {
		if s.UnidadeID != nil {
			out = append(out, s.UnidadeID.String())
		}
	}
	return out
}

// ListVisible aplica no SQL a mesma regra de sigilo de domain.CanRead.
func (r *Repository) ListVisible(ctx context.Context, db database.DBTX, identity auth.Identity, f domain.Filter, p pagination.Params) ([]domain.Processo, int64, error) {
	const where = ` WHERE (
			p.created_by = $1
			OR EXISTS (SELECT 1 FROM tramite_acessos a WHERE a.processo_id = p.id AND a.user_id = $1)
			OR p.sigilo = 'publico'
			OR (p.sigilo = 'restrito' AND ($2 OR p.unidade_atual_id = ANY($3::uuid[]) OR p.unidade_origem_id = ANY($3::uuid[])
				OR p.unidade_atual_id = ANY($8::uuid[]) OR p.unidade_origem_id = ANY($8::uuid[])))
		)
		AND ($4 = '' OR p.status = $4)
		AND ($5::uuid IS NULL OR p.unidade_atual_id = $5)
		AND (NOT $6 OR p.unidade_atual_id = ANY($3::uuid[]))
		AND ($7 = '' OR p.search @@ nexus_search_tsquery('portuguese', $7) OR starts_with(p.numero, $7))`
	// $2/$8: onde o gestor (tramite:manage) alcança — tudo, ou a árvore de
	// unidades das concessões dele (ADR 013).
	gestao := auth.CoverageOf(identity, auth.PermTramiteManage)
	args := []any{identity.UserID, gestao.All, unidadesOf(identity),
		f.Status, f.UnidadeID, f.Mine, f.Query, uuidStrings(gestao.Unidades)}
	var total int64
	if err := db.QueryRow(ctx, `SELECT count(*)`+procFrom+where, args...).Scan(&total); err != nil {
		return nil, 0, wrap(err)
	}
	rows, err := db.Query(ctx, `SELECT `+procCols+procFrom+where+` ORDER BY p.updated_at DESC LIMIT $9 OFFSET $10`,
		append(args, p.Limit(), p.Offset())...)
	if err != nil {
		return nil, 0, wrap(err)
	}
	defer rows.Close()
	out := []domain.Processo{}
	for rows.Next() {
		proc, err := scanProc(rows)
		if err != nil {
			return nil, 0, wrap(err)
		}
		out = append(out, proc)
	}
	return out, total, rows.Err()
}

func (r *Repository) Update(ctx context.Context, db database.DBTX, p domain.Processo) error {
	tag, err := db.Exec(ctx, `UPDATE tramite_processos SET assunto=$2, interessado=$3, descricao=$4, sigilo=$5, status=$6,
		unidade_atual_id=$7, concluido_at=$8, atlas_procedimento_id=$9, codigo_ttdd=NULLIF($10,'') WHERE id=$1`,
		p.ID, p.Assunto, p.Interessado, p.Descricao, p.Sigilo, p.Status, p.UnidadeAtualID, p.ConcluidoAt,
		p.AtlasProcedimentoID, p.CodigoTTDD)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return wrap(err)
}

func (r *Repository) Grant(ctx context.Context, db database.DBTX, processoID, userID, grantedBy uuid.UUID) error {
	_, err := db.Exec(ctx, `INSERT INTO tramite_acessos (processo_id, user_id, granted_by) VALUES ($1,$2,$3)
		ON CONFLICT DO NOTHING`, processoID, userID, grantedBy)
	return wrap(err)
}

func (r *Repository) Revoke(ctx context.Context, db database.DBTX, processoID, userID uuid.UUID) (bool, error) {
	tag, err := db.Exec(ctx, `DELETE FROM tramite_acessos WHERE processo_id = $1 AND user_id = $2`, processoID, userID)
	if err != nil {
		return false, wrap(err)
	}
	return tag.RowsAffected() > 0, nil
}

func (r *Repository) Grants(ctx context.Context, db database.DBTX, processoID uuid.UUID) ([]domain.Grant, error) {
	rows, err := db.Query(ctx, `SELECT a.user_id, COALESCE(NULLIF(u.display_name,''), u.username, ''), a.granted_by, a.granted_at
		FROM tramite_acessos a LEFT JOIN users u ON u.id = a.user_id WHERE a.processo_id = $1 ORDER BY a.granted_at`, processoID)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	out := []domain.Grant{}
	for rows.Next() {
		var g domain.Grant
		if err := rows.Scan(&g.UserID, &g.Name, &g.GrantedBy, &g.GrantedAt); err != nil {
			return nil, wrap(err)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

const docCols = `id, processo_id, tipo, titulo, origem, conteudo, object_key, content_type, size_bytes, sha256, status,
	envelope_id, created_by, created_at, updated_at`

func scanDoc(row interface{ Scan(...any) error }) (domain.Documento, error) {
	var d domain.Documento
	err := row.Scan(&d.ID, &d.ProcessoID, &d.Tipo, &d.Titulo, &d.Origem, &d.Conteudo, &d.ObjectKey, &d.ContentType, &d.SizeBytes,
		&d.SHA256, &d.Status, &d.EnvelopeID, &d.CreatedBy, &d.CreatedAt, &d.UpdatedAt)
	return d, err
}

func (r *Repository) Documentos(ctx context.Context, db database.DBTX, processoID uuid.UUID) ([]domain.Documento, error) {
	rows, err := db.Query(ctx, `SELECT `+docCols+` FROM tramite_documentos WHERE processo_id = $1 ORDER BY created_at`, processoID)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	out := []domain.Documento{}
	for rows.Next() {
		d, err := scanDoc(rows)
		if err != nil {
			return nil, wrap(err)
		}
		d.Conteudo = ""
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *Repository) GetDocumento(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Documento, error) {
	d, err := scanDoc(db.QueryRow(ctx, `SELECT `+docCols+` FROM tramite_documentos WHERE id = $1`, id))
	return d, wrap(err)
}

func (r *Repository) DocumentoByEnvelope(ctx context.Context, db database.DBTX, envelopeID uuid.UUID) (domain.Documento, error) {
	d, err := scanDoc(db.QueryRow(ctx, `SELECT `+docCols+` FROM tramite_documentos WHERE envelope_id = $1`, envelopeID))
	return d, wrap(err)
}

func (r *Repository) PendingSignatures(ctx context.Context, db database.DBTX, processoID uuid.UUID) (int, error) {
	var n int
	err := db.QueryRow(ctx, `SELECT count(*) FROM tramite_documentos WHERE processo_id = $1 AND status = 'aguardando_assinatura'`,
		processoID).Scan(&n)
	return n, wrap(err)
}

func (r *Repository) InsertDocumento(ctx context.Context, db database.DBTX, d domain.Documento) error {
	_, err := db.Exec(ctx, `INSERT INTO tramite_documentos (id, processo_id, tipo, titulo, origem, conteudo, object_key, content_type,
		size_bytes, sha256, status, created_by) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		d.ID, d.ProcessoID, d.Tipo, d.Titulo, d.Origem, d.Conteudo, d.ObjectKey, d.ContentType, d.SizeBytes, d.SHA256, d.Status, d.CreatedBy)
	return wrap(err)
}

func (r *Repository) UpdateDocumento(ctx context.Context, db database.DBTX, d domain.Documento) error {
	tag, err := db.Exec(ctx, `UPDATE tramite_documentos SET tipo=$2, titulo=$3, conteudo=$4, sha256=$5, status=$6, envelope_id=$7
		WHERE id=$1`, d.ID, d.Tipo, d.Titulo, d.Conteudo, d.SHA256, d.Status, d.EnvelopeID)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return wrap(err)
}

func (r *Repository) AddMovimento(ctx context.Context, db database.DBTX, m domain.Movimento) error {
	_, err := db.Exec(ctx, `INSERT INTO tramite_movimentos (processo_id, acao, de_unidade_id, para_unidade_id, despacho, actor_id)
		VALUES ($1,$2,$3,$4,$5,$6)`, m.ProcessoID, m.Acao, m.DeUnidadeID, m.ParaUnidadeID, m.Despacho, m.ActorID)
	return wrap(err)
}

func (r *Repository) Movimentos(ctx context.Context, db database.DBTX, processoID uuid.UUID) ([]domain.Movimento, error) {
	rows, err := db.Query(ctx, `SELECT m.id, m.processo_id, m.acao, m.de_unidade_id, COALESCE(de.nome,''), m.para_unidade_id,
		COALESCE(pa.nome,''), m.despacho, m.actor_id, COALESCE(NULLIF(u.display_name,''), u.username, ''), m.created_at
		FROM tramite_movimentos m
		LEFT JOIN unidades de ON de.id = m.de_unidade_id
		LEFT JOIN unidades pa ON pa.id = m.para_unidade_id
		LEFT JOIN users u ON u.id = m.actor_id
		WHERE m.processo_id = $1 ORDER BY m.created_at`, processoID)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	out := []domain.Movimento{}
	for rows.Next() {
		var m domain.Movimento
		if err := rows.Scan(&m.ID, &m.ProcessoID, &m.Acao, &m.DeUnidadeID, &m.DeUnidade, &m.ParaUnidadeID, &m.ParaUnidade,
			&m.Despacho, &m.ActorID, &m.ActorName, &m.CreatedAt); err != nil {
			return nil, wrap(err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *Repository) ProcessosDoProcedimento(ctx context.Context, db database.DBTX, atlasProcedimentoID uuid.UUID) ([]domain.Processo, error) {
	rows, err := db.Query(ctx, `SELECT `+procCols+procFrom+` WHERE p.atlas_procedimento_id = $1 AND p.status IN ('aberto', 'em_tramitacao')
		ORDER BY p.created_at`, atlasProcedimentoID)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	out := []domain.Processo{}
	for rows.Next() {
		p, err := scanProc(rows)
		if err != nil {
			return nil, wrap(err)
		}
		out = append(out, p)
	}
	return out, wrap(rows.Err())
}
