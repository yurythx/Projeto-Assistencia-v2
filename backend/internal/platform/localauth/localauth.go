// Package localauth implementa o login local por usuário/senha
// (§ Sistema de Login Local): um caminho de autenticação ADICIONAL ao
// Keycloak, nunca um substituto — pensado para desenvolvimento/teste, onde
// depender de um Keycloak sempre acessível é atrito, e como conta de
// emergência caso o Keycloak externo fique fora do ar. A verificação do
// token resultante mora em internal/platform/auth (que já sabia validar
// tokens do Keycloak); este pacote só cuida do lado "entrada": conferir a
// senha, aplicar o bloqueio de conta e emitir o token.
//
// Só existe uma tabela de usuários — a mesma que já espelha contas do
// Keycloak (migration 000011 tornou keycloak_subject opcional e adicionou
// password_hash/roles; migration 000012 adicionou o bloqueio por
// tentativas). Uma conta local é só uma linha de "users" sem
// keycloak_subject e com password_hash preenchido.
package localauth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yurythx/projeto-nexus/internal/platform/database"
)

// maxFailedAttempts é a partir de quantas tentativas seguidas de senha
// errada a conta local é bloqueada. O bloqueio é PROGRESSIVO (A07): 1 min
// na 5ª falha, dobrando a cada falha seguinte (2, 4, 8 min...), até
// maxLockout — frustra força bruta sem exigir desbloqueio manual.
const maxFailedAttempts = 5

// maxLockout é o teto do bloqueio progressivo.
const maxLockout = 24 * time.Hour

// Account é o subconjunto de uma linha de "users" necessário para
// autenticar um login local — nunca é serializado de volta ao cliente
// (PasswordHash em especial não pode vazar em resposta HTTP nenhuma).
type Account struct {
	ID                  uuid.UUID
	Username            string
	Email               string
	DisplayName         string
	PasswordHash        string
	Roles               []string
	Groups              []string
	Active              bool
	FailedLoginAttempts int
	LockedUntil         *time.Time
}

// Locked reporta se a conta está bloqueada NESTE momento — LockedUntil
// pode estar preenchido no passado (um bloqueio antigo, já expirado, cuja
// linha só será limpa no próximo login bem-sucedido); só um LockedUntil no
// futuro conta como bloqueado de verdade.
func (a *Account) Locked() bool {
	return a.LockedUntil != nil && a.LockedUntil.After(time.Now())
}

// Store persiste e consulta contas locais.
type Store interface {
	// GetByUsername busca uma conta local pelo username. Retorna
	// pgx.ErrNoRows (envolto) se não existir NENHUM usuário com esse
	// username, ou se existir mas for uma conta só-Keycloak
	// (password_hash NULL) — as duas situações são tratadas da mesma
	// forma pelo handler de login (credenciais inválidas), para não
	// revelar a um atacante qual delas ocorreu.
	GetByUsername(ctx context.Context, username string) (*Account, error)

	// TouchLastSeen atualiza last_seen_at após um login bem-sucedido — o
	// mesmo campo que o fluxo do Keycloak atualiza a cada upsert (ver
	// users.Repository.UpsertByKeycloakSubject), para que a lista de
	// usuários no dashboard reflita atividade recente independente de
	// qual caminho de login foi usado.
	TouchLastSeen(ctx context.Context, id uuid.UUID) error

	// RegisterFailedAttempt incrementa failed_login_attempts em uma
	// operação atômica e, ao atingir maxFailedAttempts, define
	// locked_until = now() + lockoutDuration — chamado a cada senha
	// errada aceita como tal (ou seja, não chamado quando a conta já
	// estava bloqueada, ver Handlers.Login).
	RegisterFailedAttempt(ctx context.Context, id uuid.UUID) error

	// UpdatePasswordHash regrava o hash (rehash transparente bcrypt ->
	// Argon2id após login bem-sucedido).
	UpdatePasswordHash(ctx context.Context, id uuid.UUID, hash string) error

	// ResetFailedAttempts zera o contador e limpa locked_until — chamado
	// em todo login bem-sucedido, para que um bloqueio antigo não
	// continue contando tentativas de uma janela de tempo já encerrada.
	ResetFailedAttempts(ctx context.Context, id uuid.UUID) error
}

// PostgresStore implementa Store sobre a tabela users compartilhada com o
// módulo users.
type PostgresStore struct {
	db database.DBTX // o pool, em produção
}

// NewPostgresStore cria o store.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{db: pool}
}

var _ Store = (*PostgresStore)(nil)

func (s *PostgresStore) GetByUsername(ctx context.Context, username string) (*Account, error) {
	const q = `
		SELECT id, username, email, display_name, password_hash, roles, COALESCE(groups, '{}'), active,
		       failed_login_attempts, locked_until
		FROM users
		WHERE username = $1 AND password_hash IS NOT NULL
	`
	var a Account
	err := s.db.QueryRow(ctx, q, username).Scan(
		&a.ID, &a.Username, &a.Email, &a.DisplayName, &a.PasswordHash, &a.Roles, &a.Groups, &a.Active,
		&a.FailedLoginAttempts, &a.LockedUntil,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("localauth: get by username: %w", err)
	}
	return &a, nil
}

func (s *PostgresStore) TouchLastSeen(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE users SET last_seen_at = now() WHERE id = $1`
	if _, err := s.db.Exec(ctx, q, id); err != nil {
		return fmt.Errorf("localauth: touch last seen: %w", err)
	}
	return nil
}

func (s *PostgresStore) RegisterFailedAttempt(ctx context.Context, id uuid.UUID) error {
	// Uma única instrução: incrementa o contador e, se o novo valor
	// atingir o limite, carimba locked_until — sem round-trip
	// leia-depois-escreva (que abriria uma corrida entre duas tentativas
	// concorrentes de login).
	const q = `
		UPDATE users
		SET failed_login_attempts = failed_login_attempts + 1,
		    locked_until = CASE
		        WHEN failed_login_attempts + 1 >= $2 THEN now() + LEAST(
		            interval '1 minute' * power(2, LEAST(failed_login_attempts + 1 - $2, 20)),
		            make_interval(secs => $3))
		        ELSE locked_until
		    END
		WHERE id = $1
	`
	if _, err := s.db.Exec(ctx, q, id, maxFailedAttempts, maxLockout.Seconds()); err != nil {
		return fmt.Errorf("localauth: register failed attempt: %w", err)
	}
	return nil
}

func (s *PostgresStore) ResetFailedAttempts(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE users SET failed_login_attempts = 0, locked_until = NULL WHERE id = $1`
	if _, err := s.db.Exec(ctx, q, id); err != nil {
		return fmt.Errorf("localauth: reset failed attempts: %w", err)
	}
	return nil
}

func (s *PostgresStore) UpdatePasswordHash(ctx context.Context, id uuid.UUID, hash string) error {
	if _, err := s.db.Exec(ctx, `UPDATE users SET password_hash = $2 WHERE id = $1`, id, hash); err != nil {
		return fmt.Errorf("localauth: update password hash: %w", err)
	}
	return nil
}
