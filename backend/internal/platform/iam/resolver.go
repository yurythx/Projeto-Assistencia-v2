// Package iam resolve a identidade efetiva de cada requisição autenticada
// (Core — IAM & Usuários):
//
//  1. Provisionamento just-in-time: todo usuário federado (Keycloak ->
//     Active Directory via LDAP/LDAPS) ganha/atualiza sua linha em "users"
//     (nome, e-mail, grupos do AD) no primeiro acesso e a cada mudança.
//  2. Permissões efetivas: união das permissões dos perfis vindos das
//     lotações manuais (user_scopes) e do mapeamento de grupos do AD
//     (ad_group_mappings), cada um aplicado a um escopo organizacional
//     (Entidade > Unidade > Departamento).
//
// O resultado fica em cache em memória por réplica (TTL curto) e é
// invalidado em todas as réplicas por pub/sub no Redis sempre que um
// administrador altera perfis, lotações ou mapeamentos.
package iam

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/redisx"
)

const (
	cacheTTL           = 60 * time.Second
	invalidateChannel  = "iam:invalidate"
	maxCachedIdentites = 10000
)

type cached struct {
	userID      uuid.UUID
	permissions []string
	scopes      []auth.Scope
	expiresAt   time.Time
}

// Resolver implementa auth.Enricher.
type Resolver struct {
	db     database.DBTX // o pool, em produção
	redis  *redis.Client
	logger *slog.Logger
	// subscribe abre a assinatura do canal de invalidação (substituível em teste).
	subscribe func(ctx context.Context) (<-chan *redis.Message, func() error)

	mu    sync.Mutex
	cache map[string]cached
}

// NewResolver constrói o resolvedor. redis pode ser nil (sem invalidação
// entre réplicas — só o TTL).
func NewResolver(pool *pgxpool.Pool, rdb *redis.Client, logger *slog.Logger) *Resolver {
	r := &Resolver{db: pool, redis: rdb, logger: logger, cache: make(map[string]cached)}
	r.subscribe = func(ctx context.Context) (<-chan *redis.Message, func() error) {
		sub := r.redis.Subscribe(ctx, redisx.Key(invalidateChannel))
		return sub.Channel(), sub.Close
	}
	return r
}

// Enrich implementa auth.Enricher.
func (r *Resolver) Enrich(ctx context.Context, identity auth.Identity) (auth.Identity, error) {
	key := cacheKey(identity)
	r.mu.Lock()
	c, ok := r.cache[key]
	r.mu.Unlock()
	if ok && time.Now().Before(c.expiresAt) {
		return apply(identity, c), nil
	}

	userID, err := r.resolveUser(ctx, identity)
	if err != nil {
		return identity, err
	}
	perms, scopes, err := r.loadGrants(ctx, userID, identity)
	if err != nil {
		return identity, apperrors.Internal(err)
	}
	c = cached{userID: userID, permissions: perms, scopes: scopes, expiresAt: time.Now().Add(cacheTTL)}

	r.mu.Lock()
	if len(r.cache) >= maxCachedIdentites {
		r.cache = make(map[string]cached)
	}
	r.cache[key] = c
	r.mu.Unlock()
	return apply(identity, c), nil
}

func apply(identity auth.Identity, c cached) auth.Identity {
	identity.UserID = c.userID
	identity.Permissions = c.permissions
	identity.Scopes = c.scopes
	return identity
}

// cacheKey muda sempre que qualquer atributo sincronizado do token muda
// (grupos, e-mail, nome, roles) — o que força um novo provisionamento.
func cacheKey(identity auth.Identity) string {
	groups := slices.Clone(identity.Groups)
	sort.Strings(groups)
	roles := slices.Clone(identity.Roles)
	sort.Strings(roles)
	h := sha256.Sum256([]byte(strings.Join([]string{
		string(identity.Source), identity.Subject, identity.Username, identity.Email, identity.Name,
		strings.Join(groups, "\x1f"), strings.Join(roles, "\x1f"),
	}, "\x1e")))
	return hex.EncodeToString(h[:])
}

// resolveUser encontra (local) ou provisiona (Keycloak) o usuário interno
// e confere se a conta está ativa.
func (r *Resolver) resolveUser(ctx context.Context, identity auth.Identity) (uuid.UUID, error) {
	var (
		id     uuid.UUID
		active bool
	)
	switch identity.Source {
	case auth.SourceLocal:
		parsed, err := uuid.Parse(identity.Subject)
		if err != nil {
			return uuid.Nil, apperrors.Unauthorized("token local com subject inválido")
		}
		err = r.db.QueryRow(ctx, `SELECT id, active FROM users WHERE id = $1`, parsed).Scan(&id, &active)
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, apperrors.Unauthorized("conta inexistente")
		}
		if err != nil {
			return uuid.Nil, apperrors.Internal(fmt.Errorf("iam: load local user: %w", err))
		}
	default:
		var err error
		id, active, err = r.provision(ctx, identity)
		if err != nil {
			return uuid.Nil, err
		}
	}
	if !active {
		return uuid.Nil, apperrors.Forbidden("conta desativada — procure o administrador")
	}
	return id, nil
}

// provision faz o upsert do usuário federado a partir das claims do
// Keycloak (espelho do AD).
func (r *Resolver) provision(ctx context.Context, identity auth.Identity) (uuid.UUID, bool, error) {
	username := identity.Username
	if username == "" {
		username = identity.Subject
	}
	email := identity.Email
	if email == "" {
		email = identity.Subject + "@sem-email.invalid"
	}
	groups := identity.Groups
	if groups == nil {
		groups = []string{}
	}
	const q = `
		INSERT INTO users (id, keycloak_subject, username, email, display_name, groups, active,
		                   created_at, updated_at, last_seen_at, ad_synced_at)
		VALUES ($1, $2, $3, $4, $5, $6, true, now(), now(), now(), now())
		ON CONFLICT (keycloak_subject) DO UPDATE
		SET username = EXCLUDED.username,
		    email = EXCLUDED.email,
		    display_name = CASE WHEN EXCLUDED.display_name <> '' THEN EXCLUDED.display_name ELSE users.display_name END,
		    groups = EXCLUDED.groups,
		    last_seen_at = now(),
		    ad_synced_at = now()
		RETURNING id, active`
	var (
		id     uuid.UUID
		active bool
	)
	err := r.db.QueryRow(ctx, q, uuid.New(), identity.Subject, username, email, identity.Name, groups).Scan(&id, &active)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			// username/e-mail já pertencem a uma conta LOCAL: nunca
			// vinculamos automaticamente (seria tomada de conta).
			return uuid.Nil, false, apperrors.Conflict(
				"já existe uma conta local com o mesmo usuário ou e-mail — um administrador precisa vinculá-la ao Keycloak")
		}
		return uuid.Nil, false, apperrors.Internal(fmt.Errorf("iam: provision user: %w", err))
	}
	return id, active, nil
}

// loadGrants calcula permissões e lotações efetivas.
func (r *Resolver) loadGrants(ctx context.Context, userID uuid.UUID, identity auth.Identity) ([]string, []auth.Scope, error) {
	groups := make([]string, 0, len(identity.Groups)*2)
	for _, g := range identity.Groups {
		groups = append(groups, strings.ToLower(strings.TrimSpace(g)), auth.NormalizeGroup(g))
	}
	const q = `
		SELECT p.slug, p.permissoes, s.entidade_id, s.unidade_id, s.departamento_id, 'manual',
		       nexus_unidades_cobertas(s.entidade_id, s.unidade_id, s.departamento_id), nexus_unidades_acima(s.unidade_id)
		  FROM user_scopes s JOIN perfis p ON p.id = s.perfil_id AND p.ativo
		 WHERE s.user_id = $1
		   AND nexus_scope_active(s.entidade_id, s.unidade_id, s.departamento_id)
		UNION ALL
		SELECT p.slug, p.permissoes, m.entidade_id, m.unidade_id, m.departamento_id, 'ad',
		       nexus_unidades_cobertas(m.entidade_id, m.unidade_id, m.departamento_id), nexus_unidades_acima(m.unidade_id)
		  FROM ad_group_mappings m JOIN perfis p ON p.id = m.perfil_id AND p.ativo
		 WHERE lower(m.ad_group) = ANY($2)
		   AND nexus_scope_active(m.entidade_id, m.unidade_id, m.departamento_id)`
	rows, err := r.db.Query(ctx, q, userID, groups)
	if err != nil {
		return nil, nil, fmt.Errorf("iam: load grants: %w", err)
	}
	defer rows.Close()

	// Cada concessão guarda as permissões do perfil e a árvore de unidades
	// que cobre (ADR 013). A lista achatada é o que a pessoa pode fazer em
	// ALGUM lugar: concessão com escopo só conta nas permissões que valem
	// com escopo (auth.EffectivePermissions).
	scopes := []auth.Scope{}
	for rows.Next() {
		var s auth.Scope
		if err := rows.Scan(&s.Perfil, &s.Permissions, &s.EntidadeID, &s.UnidadeID, &s.DepartamentoID, &s.Origem, &s.Unidades, &s.Acima); err != nil {
			return nil, nil, fmt.Errorf("iam: scan grant: %w", err)
		}
		scopes = append(scopes, s)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	perms := auth.EffectivePermissions(scopes)
	// Role de realm nexus-admin equivale a "*" (evita trancar o
	// administrador fora por um mapeamento mal configurado).
	if identity.HasRole(auth.RoleAdmin) && !slices.Contains(perms, "*") {
		perms = append([]string{"*"}, perms...)
	}
	return perms, scopes, nil
}

// Invalidate descarta o cache desta réplica e avisa as demais pelo Redis.
// Chamado depois de toda alteração em perfis, lotações e mapeamentos.
func (r *Resolver) Invalidate(ctx context.Context) {
	r.clear()
	if r.redis == nil {
		return
	}
	if err := r.redis.Publish(ctx, redisx.Key(invalidateChannel), "1").Err(); err != nil {
		r.logger.Warn("iam: falha ao propagar invalidação de cache — réplicas convergem pelo TTL", slog.Any("error", err))
	}
}

func (r *Resolver) clear() {
	r.mu.Lock()
	r.cache = make(map[string]cached)
	r.mu.Unlock()
}

// RunInvalidationListener assina o canal de invalidação e limpa o cache
// local a cada aviso. Bloqueia até ctx ser cancelado.
func (r *Resolver) RunInvalidationListener(ctx context.Context) error {
	if r.redis == nil {
		<-ctx.Done()
		return nil
	}
	ch, closeSub := r.subscribe(ctx)
	defer func() { _ = closeSub() }()
	for {
		select {
		case <-ctx.Done():
			return nil
		case _, ok := <-ch:
			if !ok {
				return fmt.Errorf("iam: canal de invalidação fechado")
			}
			r.clear()
		}
	}
}
