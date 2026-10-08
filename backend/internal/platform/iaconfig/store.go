package iaconfig

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/secretcrypto"
)

// Store persiste conexões e usos (tabelas ia_conexoes e ia_uso, migration
// 000136). Handlers e o Roteador dependem da interface (fakes nos testes).
type Store interface {
	Listar(ctx context.Context) ([]Conexao, error)
	Obter(ctx context.Context, id uuid.UUID) (Conexao, error)
	// Salvar insere (ID nil) ou atualiza. Chave vazia mantém a gravada,
	// a menos que removerChave.
	Salvar(ctx context.Context, c Conexao, removerChave bool, por string) (Conexao, error)
	Excluir(ctx context.Context, id uuid.UUID) error
	RegistrarTeste(ctx context.Context, id uuid.UUID, t Teste) error
	Uso(ctx context.Context, funcao string) (Uso, error)
	DefinirUso(ctx context.Context, u Uso, por string) (Uso, error)
}

// PostgresStore implementa Store.
type PostgresStore struct {
	db     database.DBTX
	cipher *secretcrypto.Cipher
}

// NewPostgresStore cria o store (o pool, em produção).
func NewPostgresStore(db database.DBTX, cipher *secretcrypto.Cipher) *PostgresStore {
	return &PostgresStore{db: db, cipher: cipher}
}

var _ Store = (*PostgresStore)(nil)

const colunas = `id, nome, provedor, endpoint, modelo, chave_cifrada, externo, timeout_segundos,
	teste_ok, teste_em, teste_latencia_ms, teste_erro, updated_at, updated_by`

func (s *PostgresStore) scan(row pgx.Row) (Conexao, error) {
	var (
		c       Conexao
		cifrada string
		ok      *bool
		em      *time.Time
		lat     *int
		erro    string
	)
	if err := row.Scan(&c.ID, &c.Nome, &c.Provedor, &c.Endpoint, &c.Modelo, &cifrada, &c.Externo, &c.TimeoutSegundos,
		&ok, &em, &lat, &erro, &c.UpdatedAt, &c.UpdatedBy); err != nil {
		return c, err
	}
	if ok != nil {
		c.UltimoTeste = &Teste{OK: *ok, Em: *em, LatenciaMs: *lat, Erro: erro}
	}
	chave, err := s.cipher.Decrypt(cifrada)
	if err != nil {
		return c, fmt.Errorf("iaconfig: chave da conexão %q ilegível (CONFIG_ENCRYPTION_KEY mudou?): %w", c.Nome, err)
	}
	c.Chave = chave
	return c, nil
}

func wrap(err error) error {
	switch {
	case err == nil:
		return nil
	case database.IsNoRows(err):
		return ErrNaoEncontrada
	case database.IsUniqueViolation(err):
		return ErrNomeRepetido
	case database.IsForeignKeyViolation(err):
		return ErrEmUso
	}
	return fmt.Errorf("iaconfig: %w", err)
}

func (s *PostgresStore) Listar(ctx context.Context) ([]Conexao, error) {
	rows, err := s.db.Query(ctx, `SELECT `+colunas+` FROM ia_conexoes ORDER BY nome`)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	out := []Conexao{}
	for rows.Next() {
		c, err := s.scan(rows)
		if err != nil {
			return nil, wrap(err)
		}
		out = append(out, c)
	}
	return out, wrap(rows.Err())
}

func (s *PostgresStore) Obter(ctx context.Context, id uuid.UUID) (Conexao, error) {
	c, err := s.scan(s.db.QueryRow(ctx, `SELECT `+colunas+` FROM ia_conexoes WHERE id = $1`, id))
	return c, wrap(err)
}

func (s *PostgresStore) Salvar(ctx context.Context, c Conexao, removerChave bool, por string) (Conexao, error) {
	cifrada := s.cipher.Encrypt(c.Chave)
	if c.ID == uuid.Nil {
		return s.scanWrap(s.db.QueryRow(ctx, `INSERT INTO ia_conexoes (nome, provedor, endpoint, modelo, chave_cifrada, externo,
			timeout_segundos, updated_by) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING `+colunas,
			c.Nome, c.Provedor, c.Endpoint, c.Modelo, cifrada, c.Externo, c.TimeoutSegundos, por))
	}
	// Chave vazia = manter a gravada (a tela nunca recebe a chave de volta).
	return s.scanWrap(s.db.QueryRow(ctx, `UPDATE ia_conexoes SET nome = $2, provedor = $3, endpoint = $4, modelo = $5,
			chave_cifrada = CASE WHEN $9 THEN '' WHEN $6 = '' THEN chave_cifrada ELSE $6 END,
			externo = $7, timeout_segundos = $8, updated_at = now(), updated_by = $10
		WHERE id = $1 RETURNING `+colunas,
		c.ID, c.Nome, c.Provedor, c.Endpoint, c.Modelo, cifrada, c.Externo, c.TimeoutSegundos, removerChave, por))
}

func (s *PostgresStore) scanWrap(row pgx.Row) (Conexao, error) {
	c, err := s.scan(row)
	return c, wrap(err)
}

func (s *PostgresStore) Excluir(ctx context.Context, id uuid.UUID) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM ia_conexoes WHERE id = $1`, id)
	if err != nil {
		return wrap(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNaoEncontrada
	}
	return nil
}

func (s *PostgresStore) RegistrarTeste(ctx context.Context, id uuid.UUID, t Teste) error {
	_, err := s.db.Exec(ctx, `UPDATE ia_conexoes SET teste_ok = $2, teste_em = $3, teste_latencia_ms = $4, teste_erro = $5
		WHERE id = $1`, id, t.OK, t.Em, t.LatenciaMs, t.Erro)
	return wrap(err)
}

const colunasUso = `funcao, principal_id, reserva_id, mascarar_dados_pessoais, externo_autorizado_por,
	externo_autorizado_em, updated_at, updated_by`

func scanUso(row pgx.Row) (Uso, error) {
	u := Uso{Configurado: true}
	err := row.Scan(&u.Funcao, &u.PrincipalID, &u.ReservaID, &u.MascararDadosPessoais, &u.ExternoAutorizadoPor,
		&u.ExternoAutorizadoEm, &u.UpdatedAt, &u.UpdatedBy)
	return u, err
}

// Uso devolve a configuração da função (Configurado=false se nunca salva:
// mascarar ligado, como na tela).
func (s *PostgresStore) Uso(ctx context.Context, funcao string) (Uso, error) {
	u, err := scanUso(s.db.QueryRow(ctx, `SELECT `+colunasUso+` FROM ia_uso WHERE funcao = $1`, funcao))
	if errors.Is(err, pgx.ErrNoRows) {
		return Uso{Funcao: funcao, MascararDadosPessoais: true}, nil
	}
	return u, wrap(err)
}

func (s *PostgresStore) DefinirUso(ctx context.Context, u Uso, por string) (Uso, error) {
	out, err := scanUso(s.db.QueryRow(ctx, `INSERT INTO ia_uso (funcao, principal_id, reserva_id, mascarar_dados_pessoais,
			externo_autorizado_por, externo_autorizado_em, updated_at, updated_by)
		VALUES ($1,$2,$3,$4,$5,$6, now(), $7)
		ON CONFLICT (funcao) DO UPDATE SET principal_id = EXCLUDED.principal_id, reserva_id = EXCLUDED.reserva_id,
			mascarar_dados_pessoais = EXCLUDED.mascarar_dados_pessoais, externo_autorizado_por = EXCLUDED.externo_autorizado_por,
			externo_autorizado_em = EXCLUDED.externo_autorizado_em, updated_at = now(), updated_by = EXCLUDED.updated_by
		RETURNING `+colunasUso,
		u.Funcao, u.PrincipalID, u.ReservaID, u.MascararDadosPessoais, u.ExternoAutorizadoPor, u.ExternoAutorizadoEm, por))
	return out, wrap(err)
}
