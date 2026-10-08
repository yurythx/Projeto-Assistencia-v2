package kernel

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yurythx/projeto-nexus/internal/platform/audit"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
)

// Store persiste o estado de ativação dos plugins.
type Store interface {
	// Ensure cria a linha do módulo com o estado default, se ainda não existir.
	Ensure(ctx context.Context, key string, enabled bool) error
	// LoadAll devolve o estado persistido de todos os módulos.
	LoadAll(ctx context.Context) (map[string]bool, error)
	// Set grava o novo estado e a entrada de auditoria na MESMA transação.
	Set(ctx context.Context, key string, enabled bool, actor string, entry audit.Entry) error
	// Listen bloqueia até ctx acabar, chamando onChange a cada NOTIFY.
	Listen(ctx context.Context, onChange func()) error
}

// ModulesChannel é o canal LISTEN/NOTIFY disparado pelo gatilho de
// system_modules (migration 000100).
const ModulesChannel = "nexus_modules_channel"

// PostgresStore implementa Store sobre system_modules.
type PostgresStore struct {
	pool   *pgxpool.Pool
	db     database.DBTX // leituras/escritas avulsas (o pool, em produção)
	logger *slog.Logger
	// backoff inicial e máximo da reconexão do LISTEN.
	minBackoff, maxBackoff time.Duration
	channel                string
}

// NewPostgresStore cria o Store. logger recebe as quedas da conexão de
// LISTEN (antes eram descartadas em silêncio).
func NewPostgresStore(pool *pgxpool.Pool, logger *slog.Logger) *PostgresStore {
	return &PostgresStore{pool: pool, db: pool, logger: logger, minBackoff: time.Second, maxBackoff: 30 * time.Second, channel: ModulesChannel}
}

func (s *PostgresStore) Ensure(ctx context.Context, key string, enabled bool) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO system_modules (key, enabled) VALUES ($1, $2) ON CONFLICT (key) DO NOTHING`, key, enabled)
	if err != nil {
		return fmt.Errorf("kernel: ensure module %s: %w", key, err)
	}
	return nil
}

func (s *PostgresStore) LoadAll(ctx context.Context) (map[string]bool, error) {
	rows, err := s.db.Query(ctx, `SELECT key, enabled FROM system_modules`)
	if err != nil {
		return nil, fmt.Errorf("kernel: load modules: %w", err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var k string
		var e bool
		if err := rows.Scan(&k, &e); err != nil {
			return nil, fmt.Errorf("kernel: load modules: %w", err)
		}
		out[k] = e
	}
	return out, rows.Err()
}

func (s *PostgresStore) Set(ctx context.Context, key string, enabled bool, actor string, entry audit.Entry) error {
	return database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO system_modules (key, enabled, updated_at, updated_by) VALUES ($1, $2, now(), $3)
			ON CONFLICT (key) DO UPDATE SET enabled = EXCLUDED.enabled, updated_at = now(), updated_by = EXCLUDED.updated_by`,
			key, enabled, actor); err != nil {
			return fmt.Errorf("kernel: set module %s: %w", key, err)
		}
		return audit.NewWriter(tx).Record(ctx, entry)
	})
}

// Listen mantém uma conexão dedicada em LISTEN e reconecta com backoff
// exponencial. O backoff volta ao mínimo sempre que o LISTEN chegou a ser
// estabelecido — uma queda depois de horas conectado não espera 30s.
func (s *PostgresStore) Listen(ctx context.Context, onChange func()) error {
	backoff := s.minBackoff
	for {
		listening, err := s.listenOnce(ctx, onChange)
		if ctx.Err() != nil {
			return nil
		}
		if listening {
			backoff = s.minBackoff
		}
		s.logger.Warn("kernel: conexão de LISTEN caiu, reconectando",
			slog.Any("error", err), slog.Duration("retry_in", backoff))
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
		}
		if backoff < s.maxBackoff {
			backoff = min(backoff*2, s.maxBackoff)
		}
	}
}

// listenOnce devolve listening=true se o LISTEN foi estabelecido antes do erro.
func (s *PostgresStore) listenOnce(ctx context.Context, onChange func()) (bool, error) {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "LISTEN "+s.channel); err != nil {
		return false, err
	}
	// Recarrega logo após (re)conectar: pode ter havido mudança enquanto
	// a conexão estava fora.
	onChange()
	for {
		if _, err := conn.Conn().WaitForNotification(ctx); err != nil {
			return true, err
		}
		onChange()
	}
}
