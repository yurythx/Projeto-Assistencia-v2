// Package notificacoes é a caixa de notificações da plataforma (ADR 027):
// cada aviso fica gravado para o usuário (lido ou não), chega na hora pelo
// WebSocket (tópico pessoal) e respeita a preferência por módulo. A chave
// torna o envio idempotente: repetir o mesmo aviso não duplica.
package notificacoes

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/ws"
)

// FrameNova é o tipo do frame de WebSocket de uma notificação nova.
const FrameNova = "notificacao.nova"

// Retencao: notificação lida há mais que isto é apagada (minimização, LGPD
// art. 6º III) quando chega um aviso novo para a mesma pessoa.
const Retencao = "180 days"

// MaxLista limita a caixa devolvida de uma vez.
const MaxLista = 50

// ErrNaoEncontrada: notificação inexistente (ou de outra pessoa).
var ErrNaoEncontrada = errors.New("notificacoes: notificação não encontrada")

// ErrInvalida: aviso fora das regras (título, link, chave).
type ErrInvalida struct{ Msg string }

func (e ErrInvalida) Error() string { return e.Msg }

// Nova é um aviso a enviar.
type Nova struct {
	Modulo   string // chave do módulo ("atlas", "tramite")
	Titulo   string
	Mensagem string
	Link     string // caminho interno ("/atlas/…"); vazio = sem link
	Chave    string // idempotência por usuário ("atlas:wf:<id>")
}

// Validar confere o aviso.
func (n Nova) Validar() error {
	switch {
	case n.Modulo == "" || utf8.RuneCountInString(n.Modulo) > 40:
		return ErrInvalida{"módulo da notificação inválido"}
	case n.Titulo == "" || utf8.RuneCountInString(n.Titulo) > 200:
		return ErrInvalida{"título da notificação obrigatório (até 200 caracteres)"}
	case utf8.RuneCountInString(n.Mensagem) > 1000:
		return ErrInvalida{"mensagem da notificação acima de 1.000 caracteres"}
	case n.Link != "" && (!strings.HasPrefix(n.Link, "/") || strings.HasPrefix(n.Link, "//") || strings.HasPrefix(n.Link, "/\\")):
		return ErrInvalida{"o link da notificação tem de ser um caminho interno"}
	case n.Chave == "" || utf8.RuneCountInString(n.Chave) > 200:
		return ErrInvalida{"chave da notificação inválida"}
	}
	return nil
}

// Notificacao é um aviso na caixa do usuário.
type Notificacao struct {
	ID        uuid.UUID `json:"id"`
	Modulo    string    `json:"modulo"`
	Titulo    string    `json:"titulo"`
	Mensagem  string    `json:"mensagem"`
	Link      string    `json:"link"`
	Lida      bool      `json:"lida"`
	CreatedAt time.Time `json:"created_at"`
}

// Caixa é a lista do usuário com o total de não lidas.
type Caixa struct {
	Itens   []Notificacao `json:"itens"`
	NaoLida int           `json:"nao_lidas"`
}

// Preferencia diz se o usuário recebe os avisos de um módulo.
type Preferencia struct {
	Modulo string `json:"modulo"`
	Ativo  bool   `json:"ativo"`
}

// Publisher entrega frames em tempo real (implementado por *ws.Hub).
type Publisher interface {
	Publish(ctx context.Context, topic, frameType string, data any) error
}

// Service grava e entrega notificações.
type Service struct {
	pool   *pgxpool.Pool
	db     database.DBTX // leituras e atualizações (o pool; substituível em teste)
	hub    Publisher     // nil: só grava (sem tempo real)
	logger *slog.Logger
}

// NewService cria o serviço.
func NewService(pool *pgxpool.Pool, hub Publisher, logger *slog.Logger) *Service {
	return &Service{pool: pool, db: pool, hub: hub, logger: logger}
}

// Enviada é o que foi gravado para cada destinatário (para entregar depois
// do commit).
type Enviada struct {
	Usuario uuid.UUID
	Item    Notificacao
}

// Registrar grava o aviso na transação do chamador para os usuários que não
// o desligaram e ainda não o receberam; devolve o que gravou — entregue com
// Entregar depois do commit.
func Registrar(ctx context.Context, db database.DBTX, usuarios []uuid.UUID, n Nova) ([]Enviada, error) {
	if err := n.Validar(); err != nil {
		return nil, err
	}
	if len(usuarios) == 0 {
		return nil, nil
	}
	rows, err := db.Query(ctx, `INSERT INTO notificacoes (user_id, modulo, titulo, mensagem, link, chave)
		SELECT u, $2, $3, $4, $5, $6 FROM unnest($1::uuid[]) AS u
		WHERE EXISTS (SELECT 1 FROM users WHERE id = u)
			AND NOT EXISTS (SELECT 1 FROM notificacoes_preferencias p WHERE p.user_id = u AND p.modulo = $2 AND NOT p.ativo)
		ON CONFLICT (user_id, chave) DO NOTHING
		RETURNING user_id, id, created_at`, usuarios, n.Modulo, n.Titulo, n.Mensagem, n.Link, n.Chave)
	if err != nil {
		return nil, fmt.Errorf("notificacoes: %w", err)
	}
	defer rows.Close()
	var out []Enviada
	for rows.Next() {
		e := Enviada{Item: Notificacao{Modulo: n.Modulo, Titulo: n.Titulo, Mensagem: n.Mensagem, Link: n.Link}}
		if err := rows.Scan(&e.Usuario, &e.Item.ID, &e.Item.CreatedAt); err != nil {
			return nil, fmt.Errorf("notificacoes: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("notificacoes: %w", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM notificacoes WHERE user_id = ANY($1) AND lida_em < now() - $2::interval`,
		usuarios, Retencao); err != nil {
		return nil, fmt.Errorf("notificacoes: %w", err)
	}
	return out, nil
}

// Entregar publica cada aviso gravado no tópico pessoal do destinatário.
// É o melhor esforço: o aviso já está na caixa; quem estiver fora vê ao
// abrir o sino.
func (s *Service) Entregar(ctx context.Context, enviadas []Enviada) {
	if s.hub == nil {
		return
	}
	for _, e := range enviadas {
		if err := s.hub.Publish(ctx, ws.UserTopic(e.Usuario.String()), FrameNova, e.Item); err != nil {
			s.logger.WarnContext(ctx, "notificacoes: entrega em tempo real falhou (o aviso está na caixa)", "error", err)
		}
	}
}

// Enviar grava (transação própria) e entrega.
func (s *Service) Enviar(ctx context.Context, usuarios []uuid.UUID, n Nova) (int, error) {
	var enviadas []Enviada
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) (err error) {
		enviadas, err = Registrar(ctx, tx, usuarios, n)
		return err
	})
	if err != nil {
		return 0, err
	}
	s.Entregar(ctx, enviadas)
	return len(enviadas), nil
}

// Caixa devolve as notificações mais recentes do usuário e quantas não
// foram lidas.
func (s *Service) Caixa(ctx context.Context, usuario uuid.UUID, limite int) (Caixa, error) {
	if limite <= 0 || limite > MaxLista {
		limite = MaxLista
	}
	out := Caixa{Itens: []Notificacao{}}
	rows, err := s.db.Query(ctx, `SELECT id, modulo, titulo, mensagem, link, lida_em IS NOT NULL, created_at
		FROM notificacoes WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2`, usuario, limite)
	if err != nil {
		return out, fmt.Errorf("notificacoes: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var n Notificacao
		if err := rows.Scan(&n.ID, &n.Modulo, &n.Titulo, &n.Mensagem, &n.Link, &n.Lida, &n.CreatedAt); err != nil {
			return out, fmt.Errorf("notificacoes: %w", err)
		}
		out.Itens = append(out.Itens, n)
	}
	if err := rows.Err(); err != nil {
		return out, fmt.Errorf("notificacoes: %w", err)
	}
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM notificacoes WHERE user_id = $1 AND lida_em IS NULL`, usuario).
		Scan(&out.NaoLida); err != nil {
		return out, fmt.Errorf("notificacoes: %w", err)
	}
	return out, nil
}

// MarcarLida marca uma notificação do usuário como lida (repetir é
// inofensivo).
func (s *Service) MarcarLida(ctx context.Context, usuario, id uuid.UUID) error {
	tag, err := s.db.Exec(ctx, `UPDATE notificacoes SET lida_em = COALESCE(lida_em, now()) WHERE id = $1 AND user_id = $2`, id, usuario)
	if err != nil {
		return fmt.Errorf("notificacoes: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNaoEncontrada
	}
	return nil
}

// MarcarTodas marca todas as notificações do usuário como lidas.
func (s *Service) MarcarTodas(ctx context.Context, usuario uuid.UUID) error {
	if _, err := s.db.Exec(ctx, `UPDATE notificacoes SET lida_em = now() WHERE user_id = $1 AND lida_em IS NULL`, usuario); err != nil {
		return fmt.Errorf("notificacoes: %w", err)
	}
	return nil
}

// Preferencias devolve as preferências gravadas (sem linha = recebe).
func (s *Service) Preferencias(ctx context.Context, usuario uuid.UUID) ([]Preferencia, error) {
	out := []Preferencia{}
	rows, err := s.db.Query(ctx, `SELECT modulo, ativo FROM notificacoes_preferencias WHERE user_id = $1 ORDER BY modulo`, usuario)
	if err != nil {
		return out, fmt.Errorf("notificacoes: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var p Preferencia
		if err := rows.Scan(&p.Modulo, &p.Ativo); err != nil {
			return out, fmt.Errorf("notificacoes: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return out, fmt.Errorf("notificacoes: %w", err)
	}
	return out, nil
}

// DefinirPreferencia liga ou desliga os avisos de um módulo para o usuário.
func (s *Service) DefinirPreferencia(ctx context.Context, usuario uuid.UUID, p Preferencia) error {
	if p.Modulo == "" || utf8.RuneCountInString(p.Modulo) > 40 {
		return ErrInvalida{"módulo inválido"}
	}
	_, err := s.db.Exec(ctx, `INSERT INTO notificacoes_preferencias (user_id, modulo, ativo) VALUES ($1, $2, $3)
		ON CONFLICT (user_id, modulo) DO UPDATE SET ativo = EXCLUDED.ativo, updated_at = now()`, usuario, p.Modulo, p.Ativo)
	if err != nil {
		return fmt.Errorf("notificacoes: %w", err)
	}
	return nil
}
