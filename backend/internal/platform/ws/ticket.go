package ws

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/yurythx/projeto-nexus/internal/platform/redisx"
)

// TicketTTL é a validade de um ticket de conexão: curto, de uso único.
const TicketTTL = 30 * time.Second

// ErrInvalidTicket indica ticket inexistente, expirado ou já usado.
var ErrInvalidTicket = errors.New("ws: ticket inválido, expirado ou já utilizado")

// Ticket autoriza UMA abertura de conexão WebSocket. O navegador não pode
// mandar o bearer token num upgrade de WebSocket (sem headers
// customizados), então ele troca o token por um ticket opaco via
// POST /api/v1/ws/ticket e o apresenta na query string do /ws.
type Ticket struct {
	Value     string
	ExpiresAt time.Time
}

// TicketStore guarda os tickets no Redis — um ticket emitido por uma
// réplica pode ser resgatado por qualquer outra (GETDEL é atômico: uso
// único garantido mesmo sob corrida).
type TicketStore struct {
	redis *redis.Client
	ttl   time.Duration
}

// NewTicketStore cria o armazenamento de tickets.
func NewTicketStore(rdb *redis.Client, ttl time.Duration) *TicketStore {
	return &TicketStore{redis: rdb, ttl: ttl}
}

// Issue emite um ticket para info.
func (s *TicketStore) Issue(ctx context.Context, info ClientInfo) (*Ticket, error) {
	raw := make([]byte, 32)
	_, _ = rand.Read(raw) // não falha (Go 1.24+: aborta o processo se o SO não tiver entropia)
	value := hex.EncodeToString(raw)
	body, _ := json.Marshal(info) // ClientInfo só tem tipos serializáveis
	if err := s.redis.Set(ctx, redisx.Key("ws", "ticket", value), body, s.ttl).Err(); err != nil {
		return nil, fmt.Errorf("ws: gravar ticket: %w", err)
	}
	return &Ticket{Value: value, ExpiresAt: time.Now().Add(s.ttl)}, nil
}

// Redeem consome o ticket (uso único) e devolve a identidade associada.
func (s *TicketStore) Redeem(ctx context.Context, value string) (ClientInfo, error) {
	if len(value) != 64 {
		return ClientInfo{}, ErrInvalidTicket
	}
	body, err := s.redis.GetDel(ctx, redisx.Key("ws", "ticket", value)).Bytes()
	if errors.Is(err, redis.Nil) {
		return ClientInfo{}, ErrInvalidTicket
	}
	if err != nil {
		return ClientInfo{}, fmt.Errorf("ws: resgatar ticket: %w", err)
	}
	var info ClientInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return ClientInfo{}, ErrInvalidTicket
	}
	return info, nil
}
