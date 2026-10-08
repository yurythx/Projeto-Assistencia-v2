// Package audit grava e consulta a trilha de auditoria imutável do Nexus
// (Core — nunca desativável).
//
// Cada registro carrega a proveniência completa exigida pela skill (§5):
// actor_id, actor_roles, ip, user_agent, entity_context, action, resource,
// diff_before, diff_after e o carimbo de tempo UTC do servidor de banco. A
// tabela é append-only (gatilhos bloqueiam UPDATE/DELETE/TRUNCATE) e cada
// linha é encadeada à anterior por SHA-256 — o hash é calculado pelo
// próprio Postgres (migration 000101), então nenhuma escrita escapa da
// cadeia. Quem chama nunca deve colocar senha, token ou segredo em
// Metadata/Before/After: a imutabilidade protege contra alteração
// posterior, não contra um segredo indevidamente gravado.
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// Nomes de ação bem conhecidos. Módulos usam "<modulo>.<entidade>.<acao>".
const (
	ActionLogin            = "login"
	ActionLogout           = "logout"
	ActionUserCreated      = "user.created"
	ActionUserUpdated      = "user.updated"
	ActionModuleToggled    = "kernel.module.toggled"
	ActionLGPDConsentGiven = "lgpd_consent_given"
)

// Entry é um registro de auditoria.
type Entry struct {
	ActorID       *uuid.UUID
	ActorSubject  string
	ActorRoles    []string
	IPAddress     string
	UserAgent     string
	EntityContext map[string]any
	Action        string
	ResourceType  string
	ResourceID    string
	// Before/After são o estado do recurso antes e depois da mudança
	// (qualquer valor serializável em JSON; nil = não se aplica).
	Before        any
	After         any
	Metadata      map[string]any
	CorrelationID *uuid.UUID
}

// execer é satisfeita tanto por *pgxpool.Pool quanto por pgx.Tx, para que
// Record possa ser atômico com a escrita de negócio da mesma transação.
type execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Writer insere linhas em audit_logs.
type Writer struct {
	db execer
}

// NewWriter constrói um Writer sobre um *pgxpool.Pool (escrita isolada) ou
// um pgx.Tx (atômica com a transação de negócio).
func NewWriter(db execer) *Writer {
	return &Writer{db: db}
}

// Record insere entry. chain_pos, prev_hash, hash e created_at são
// preenchidos pelo gatilho do banco.
func (w *Writer) Record(ctx context.Context, entry Entry) error {
	metadata, err := marshalObject(entry.Metadata)
	if err != nil {
		return fmt.Errorf("audit: marshal metadata: %w", err)
	}
	entityCtx, err := marshalObject(entry.EntityContext)
	if err != nil {
		return fmt.Errorf("audit: marshal entity_context: %w", err)
	}
	before, err := marshalOptional(entry.Before)
	if err != nil {
		return fmt.Errorf("audit: marshal diff_before: %w", err)
	}
	after, err := marshalOptional(entry.After)
	if err != nil {
		return fmt.Errorf("audit: marshal diff_after: %w", err)
	}
	roles := entry.ActorRoles
	if roles == nil {
		roles = []string{}
	}

	const q = `
		INSERT INTO audit_logs (
			id, actor_id, actor_subject, actor_roles, ip_address, user_agent, entity_context,
			action, resource_type, resource_id, diff_before, diff_after, metadata, correlation_id
		) VALUES (
			$1, $2, NULLIF($3, ''), $4, NULLIF($5, '')::inet, NULLIF($6, ''), $7,
			$8, NULLIF($9, ''), NULLIF($10, ''), $11, $12, $13, $14
		)`
	_, err = w.db.Exec(ctx, q,
		uuid.New(), entry.ActorID, entry.ActorSubject, roles, sanitizeIP(entry.IPAddress), truncate(entry.UserAgent, 512), entityCtx,
		entry.Action, entry.ResourceType, entry.ResourceID, before, after, metadata, entry.CorrelationID,
	)
	if err != nil {
		return fmt.Errorf("audit: insert entry for action %s: %w", entry.Action, err)
	}
	return nil
}

func marshalObject(m map[string]any) ([]byte, error) {
	if m == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(m)
}

func marshalOptional(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

// truncate limita s a max bytes sem partir um caractere UTF-8 e troca
// bytes inválidos/NUL — o User-Agent vem do cliente, e um texto que o
// Postgres recuse (UTF-8 inválido, \x00) derrubaria a transação de negócio
// que está sendo auditada.
func truncate(s string, max int) string {
	s = strings.ReplaceAll(strings.ToValidUTF8(s, "\uFFFD"), "\x00", "")
	if len(s) <= max {
		return s
	}
	for max > 0 && !utf8.RuneStart(s[max]) {
		max--
	}
	return s[:max]
}

// sanitizeIP normaliza Entry.IPAddress para algo que a coluna `inet`
// aceite (IP puro; tira a porta de "host:porta"); valor inválido vira ""
// (NULL) — um IP malformado nunca derruba a transação de negócio.
func sanitizeIP(raw string) string {
	if raw == "" {
		return ""
	}
	if ip := net.ParseIP(raw); ip != nil {
		return ip.String()
	}
	if host, _, err := net.SplitHostPort(raw); err == nil {
		if ip := net.ParseIP(host); ip != nil {
			return ip.String()
		}
	}
	return ""
}
