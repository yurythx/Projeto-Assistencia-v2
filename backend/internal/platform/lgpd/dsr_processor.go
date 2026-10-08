package lgpd

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/yurythx/projeto-nexus/internal/platform/audit"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
)

// erasureDetail explica, na própria linha da solicitação e para o
// titular, por que a "eliminação" é uma anonimização e não um DELETE.
const erasureDetail = "Dados pessoais anonimizados (username, e-mail, nome, hash de senha, vínculo Keycloak e papéis) " +
	"e dados pessoais guardados pelos módulos (ex.: perfil do Diretório) eliminados. " +
	"A linha do usuário é mantida por integridade referencial; a trilha de auditoria, os registros de " +
	"consentimento e os atos administrativos (tramitações, assinaturas) são preservados como registro legal, " +
	"inclusive desta própria operação (LGPD art. 16, I e III)."

// erasureFailedDetail é o que o titular vê se a eliminação esgotar as tentativas.
const erasureFailedDetail = "Não foi possível concluir a eliminação. Abra uma nova solicitação ou contate o encarregado (DPO)."

// ErasureProcessor é um processor do worker: a cada 30s varre data_subject_requests pendentes do
// tipo 'erasure' e anonimiza o titular. Roda uma vez no boot também, para
// drenar o backlog sem esperar o primeiro tick. Uma falha (ex.: banco
// instável) deixa a solicitação pendente para a próxima rodada; só depois
// de maxErasureAttempts ela vira 'failed' — e o titular pode abrir outra.
func (s *Service) ErasureProcessor() func(ctx context.Context) error {
	return func(ctx context.Context) error {
		s.processErasureBatch(ctx)
		ticker := time.NewTicker(s.erasureInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				s.processErasureBatch(ctx)
			}
		}
	}
}

type erasureJob struct {
	reqID, userID uuid.UUID
	attempts      int
}

func (s *Service) processErasureBatch(ctx context.Context) {
	rows, err := s.db.Query(ctx, `
		SELECT id, user_id, attempts FROM data_subject_requests
		WHERE kind = 'erasure' AND status = 'pending'
		ORDER BY created_at ASC LIMIT 20`)
	if err != nil {
		s.logger.Error("lgpd: varredura de solicitações de exclusão falhou", slog.Any("error", err))
		return
	}
	batch, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (erasureJob, error) {
		var j erasureJob
		err := row.Scan(&j.reqID, &j.userID, &j.attempts)
		return j, err
	})
	if err != nil {
		s.logger.Error("lgpd: leitura das solicitações de exclusão falhou", slog.Any("error", err))
		return
	}
	for _, j := range batch {
		if err := s.processOneErasure(ctx, j.reqID, j.userID); err != nil {
			s.recordErasureFailure(ctx, j, err)
			continue
		}
		s.logger.Info("lgpd: exclusão (anonimização) concluída", slog.String("request_id", j.reqID.String()))
	}
}

// recordErasureFailure conta a tentativa; no limite, marca 'failed'.
func (s *Service) recordErasureFailure(ctx context.Context, j erasureJob, cause error) {
	attempts := j.attempts + 1
	final := attempts >= s.maxErasureAttempts
	s.logger.Error("lgpd: falha ao processar exclusão",
		slog.String("request_id", j.reqID.String()), slog.Int("attempts", attempts),
		slog.Bool("final", final), slog.Any("error", cause))
	q := `UPDATE data_subject_requests SET attempts = $2 WHERE id = $1 AND status = 'pending'`
	args := []any{j.reqID, attempts}
	if final {
		q = `UPDATE data_subject_requests SET attempts = $2, status = 'failed', detail = $3
			WHERE id = $1 AND status = 'pending'`
		args = append(args, erasureFailedDetail)
	}
	if _, err := s.db.Exec(ctx, q, args...); err != nil {
		s.logger.Error("lgpd: não foi possível registrar a falha da exclusão",
			slog.String("request_id", j.reqID.String()), slog.Any("error", err))
	}
}

func (s *Service) processOneErasure(ctx context.Context, reqID, userID uuid.UUID) error {
	return database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		// Trava a solicitação e confirma que ainda está pendente (duas
		// réplicas de worker não podem processar a mesma).
		var status string
		if err := tx.QueryRow(ctx,
			`SELECT status FROM data_subject_requests WHERE id = $1 FOR UPDATE`, reqID).Scan(&status); err != nil {
			return fmt.Errorf("lock request: %w", err)
		}
		if status != "pending" {
			return nil // outra réplica pegou primeiro
		}

		// Anonimização — NUNCA DELETE. Mantém id (FK de audit_logs e
		// user_consents) e desativa a conta.
		//
		// password_hash recebe o tombstone 'ANONYMIZED' (não NULL): a
		// constraint users_has_auth_method exige keycloak_subject OU
		// password_hash preenchidos, e keycloak_subject (o "sub" do
		// Gov.br) É dado pessoal e precisa sair. 'ANONYMIZED' não é um
		// hash bcrypt válido, então bcrypt.CompareHashAndPassword sempre
		// falha; além disso active=false barra o login antes disso.
		// O sufixo é o id inteiro (não um prefixo): username é UNIQUE, e uma
		// colisão travaria a eliminação para sempre.
		short := strings.ReplaceAll(userID.String(), "-", "")
		if _, err := tx.Exec(ctx, `
			UPDATE users SET
				username         = 'anon_' || $2,
				email            = 'anon_' || $2 || '@anonimizado.invalid',
				display_name     = '',
				password_hash    = 'ANONYMIZED',
				keycloak_subject = NULL,
				roles            = '{}',
				active           = false,
				updated_at       = now()
			WHERE id = $1`, userID, short); err != nil {
			return fmt.Errorf("anonymize user: %w", err)
		}

		// Dados pessoais dos módulos, na mesma transação.
		keys, providers := s.providers()
		for _, key := range keys {
			if err := providers[key].ErasePersonalData(ctx, tx, userID); err != nil {
				return fmt.Errorf("erase %s: %w", key, err)
			}
		}

		if _, err := tx.Exec(ctx, `
			UPDATE data_subject_requests
			SET status = 'completed', completed_at = now(), detail = $2
			WHERE id = $1`, reqID, erasureDetail); err != nil {
			return fmt.Errorf("mark completed: %w", err)
		}

		// Trilha da conclusão, na mesma transação.
		uid := userID
		return audit.NewWriter(tx).Record(ctx, audit.Entry{
			ActorID:      &uid,
			Action:       "lgpd.erasure.completed",
			ResourceType: "user",
			ResourceID:   userID.String(),
			Metadata:     map[string]any{"request_id": reqID.String(), "method": "anonymization", "modules": keys},
		})
	})
}
