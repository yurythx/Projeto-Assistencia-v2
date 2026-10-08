// Package modkit reúne as dependências de plataforma que o Kernel injeta
// nos plugins e pequenos utilitários comuns à camada de aplicação dos
// módulos. Nada aqui conhece regra de negócio de nenhum módulo.
package modkit

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/text/unicode/norm"

	"github.com/yurythx/projeto-nexus/internal/platform/config"
	"github.com/yurythx/projeto-nexus/internal/platform/httpserver"
	"github.com/yurythx/projeto-nexus/internal/platform/notificacoes"
	"github.com/yurythx/projeto-nexus/internal/platform/outbox"
	"github.com/yurythx/projeto-nexus/internal/platform/secretcrypto"
	"github.com/yurythx/projeto-nexus/internal/platform/storage"
	"github.com/yurythx/projeto-nexus/internal/platform/ws"
)

// Deps são os recursos compartilhados disponíveis a todo plugin.
type Deps struct {
	Config  *config.Config
	Logger  *slog.Logger
	Pool    *pgxpool.Pool
	Outbox  *outbox.Writer
	Storage storage.Provider
	Hub     *ws.Hub
	// Notificacoes é a caixa de notificações da plataforma (ADR 027).
	Notificacoes *notificacoes.Service
	Cipher       *secretcrypto.Cipher
	// PublicLimiter limita rotas públicas por IP.
	PublicLimiter httpserver.Limiter
	// ContactLimiter é o limite dedicado do formulário de contato.
	ContactLimiter httpserver.Limiter
	// AtlasChatLimiter limita as consultas ao assistente do Atlas por
	// identidade (cada uma pode acionar o modelo de linguagem).
	AtlasChatLimiter httpserver.Limiter
	// InvalidatePermissions avisa o IAM (todas as réplicas) que perfis,
	// lotações ou mapeamentos mudaram.
	InvalidatePermissions func(ctx context.Context)
	// ResetLoginLockout limpa o bloqueio progressivo de login (Redis) de um
	// usuário — usado pelo desbloqueio administrativo do IAM.
	ResetLoginLockout func(ctx context.Context, username string) error
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify gera um slug ASCII ("Relatório Anual 2026" -> "relatorio-anual-2026").
func Slugify(s string) string {
	t := norm.NFD.String(strings.ToLower(strings.TrimSpace(s)))
	var b strings.Builder
	for _, r := range t {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(r)
	}
	slug := strings.Trim(nonSlug.ReplaceAllString(b.String(), "-"), "-")
	if len(slug) > 80 {
		slug = strings.Trim(slug[:80], "-")
	}
	return slug
}

// Snippet corta texto para exibição em resultados de busca/listagens.
func Snippet(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
