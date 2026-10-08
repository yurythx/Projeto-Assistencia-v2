// Package domain define o barramento de saída Egress: destinos de webhook
// (n8n, Zabbix, Grafana ou genérico) e as entregas de eventos.
package domain

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
)

var (
	ErrNotFound = errors.New("egress: registro não encontrado")
)

// Target é um destino de webhook.
type Target struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	Kind          string    `json:"kind"`
	URL           string    `json:"url"`
	HasSecret     bool      `json:"has_secret"`
	Secret        string    `json:"-"` // decifrado só no worker de entrega
	EventPatterns []string  `json:"event_patterns"`
	Active        bool      `json:"active"`
	CreatedBy     string    `json:"created_by"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Delivery é uma tentativa de entrega de um evento a um destino.
type Delivery struct {
	ID             uuid.UUID       `json:"id"`
	TargetID       uuid.UUID       `json:"target_id"`
	TargetName     string          `json:"target_name,omitempty"`
	EventID        uuid.UUID       `json:"event_id"`
	EventType      string          `json:"event_type"`
	Payload        json.RawMessage `json:"payload,omitempty"`
	Status         string          `json:"status"`
	Attempts       int             `json:"attempts"`
	NextAttemptAt  time.Time       `json:"next_attempt_at"`
	LastStatusCode *int            `json:"last_status_code,omitempty"`
	LastError      string          `json:"last_error,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	DeliveredAt    *time.Time      `json:"delivered_at,omitempty"`
}

// MatchPattern aplica a semântica de routing key do RabbitMQ: palavras
// separadas por ".", "*" casa exatamente uma palavra, "#" casa zero ou
// mais palavras.
func MatchPattern(pattern, key string) bool {
	return match(strings.Split(pattern, "."), strings.Split(key, "."))
}

func match(p, k []string) bool {
	for len(p) > 0 {
		switch p[0] {
		case "#":
			if len(p) == 1 {
				return true
			}
			for i := 0; i <= len(k); i++ {
				if match(p[1:], k[i:]) {
					return true
				}
			}
			return false
		case "*":
			if len(k) == 0 {
				return false
			}
		default:
			if len(k) == 0 || p[0] != k[0] {
				return false
			}
		}
		p, k = p[1:], k[1:]
	}
	return len(k) == 0
}

// Matches reporta se o destino quer receber eventType.
func (t Target) Matches(eventType string) bool {
	for _, p := range t.EventPatterns {
		if MatchPattern(p, eventType) {
			return true
		}
	}
	return false
}

// Backoff calcula o atraso da próxima tentativa (exponencial, teto 6h).
func Backoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 12 {
		return 6 * time.Hour
	}
	d := 15 * time.Second * time.Duration(1<<(attempt-1))
	if d > 6*time.Hour {
		return 6 * time.Hour
	}
	return d
}

// Repository é a porta de persistência.
type Repository interface {
	Targets(ctx context.Context, db database.DBTX, onlyActive bool) ([]Target, error)
	Target(ctx context.Context, db database.DBTX, id uuid.UUID) (Target, error)
	SaveTarget(ctx context.Context, db database.DBTX, t Target, secretEncrypted *string) (Target, error)
	DeleteTarget(ctx context.Context, db database.DBTX, id uuid.UUID) error
	EncryptedSecret(ctx context.Context, db database.DBTX, id uuid.UUID) (string, error)

	EnqueueDelivery(ctx context.Context, db database.DBTX, d Delivery) error
	ClaimDue(ctx context.Context, db database.DBTX, limit int) ([]Delivery, error)
	MarkDelivered(ctx context.Context, db database.DBTX, id uuid.UUID, statusCode int) error
	MarkFailed(ctx context.Context, db database.DBTX, id uuid.UUID, attempts int, statusCode *int, errMsg string, next time.Time, dead bool) error
	Deliveries(ctx context.Context, db database.DBTX, targetID *uuid.UUID, status string, p pagination.Params) ([]Delivery, int64, error)
	Redeliver(ctx context.Context, db database.DBTX, id uuid.UUID) error
}
