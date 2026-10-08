package iam

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
)

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func status(err error) int {
	if ae, ok := apperrors.As(err); ok {
		return ae.Status
	}
	return 0
}

func TestResolverLocalAccounts(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	r := NewResolver(pool, nil, quiet())
	uid := dbtest.User(t, pool)

	id, err := r.Enrich(ctx, auth.Identity{Subject: uid.String(), Source: auth.SourceLocal, Roles: []string{auth.RoleAdmin}})
	if err != nil || id.UserID != uid || len(id.Permissions) != 1 || id.Permissions[0] != "*" {
		t.Fatalf("admin local: %+v %v", id, err)
	}
	for name, c := range map[string]struct {
		subject string
		want    int
	}{
		"subject inválido":  {"nao-uuid", http.StatusUnauthorized},
		"conta inexistente": {uuid.NewString(), http.StatusUnauthorized},
	} {
		if _, err := r.Enrich(ctx, auth.Identity{Subject: c.subject, Source: auth.SourceLocal}); status(err) != c.want {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := pool.Exec(ctx, `UPDATE users SET active = false WHERE id = $1`, uid); err != nil {
		t.Fatal(err)
	}
	// Ainda em cache: a desativação só vale depois da invalidação.
	if _, err := r.Enrich(ctx, auth.Identity{Subject: uid.String(), Source: auth.SourceLocal, Roles: []string{auth.RoleAdmin}}); err != nil {
		t.Fatalf("cache: %v", err)
	}
	r.Invalidate(ctx)
	if _, err := r.Enrich(ctx, auth.Identity{Subject: uid.String(), Source: auth.SourceLocal, Roles: []string{auth.RoleAdmin}}); status(err) != http.StatusForbidden {
		t.Fatalf("conta desativada: %v", err)
	}
}

func TestResolverProvisionsFederatedUsersAndMapsADGroups(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	r := NewResolver(pool, nil, quiet())
	sfx := uuid.NewString()[:8]
	group := "CN=Obras-" + sfx + ",OU=Grupos,DC=org"
	var perfil uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO perfis (slug, nome, permissoes) VALUES ($1, $1, ARRAY['blog:manage']) RETURNING id`, "p-"+sfx).Scan(&perfil); err != nil {
		t.Fatal(err)
	}
	ent := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO entidades (id, nome, slug) VALUES ($1, $2, $2)`, ent, "ent-"+sfx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO ad_group_mappings (ad_group, perfil_id, entidade_id) VALUES ($1, $2, $3)`, auth.NormalizeGroup(group), perfil, ent); err != nil {
		t.Fatal(err)
	}
	sub := "kc-" + sfx
	fed := auth.Identity{Subject: sub, Source: auth.SourceKeycloak, Groups: []string{group}, Name: "Maria"}
	id, err := r.Enrich(ctx, fed)
	if err != nil || id.UserID == uuid.Nil || len(id.Permissions) != 1 || id.Permissions[0] != "blog:manage" || len(id.Scopes) != 1 || id.Scopes[0].Origem != "ad" {
		t.Fatalf("provisionado com o perfil do grupo do AD: %+v %v", id, err)
	}
	var username, email string
	_ = pool.QueryRow(ctx, `SELECT username, email FROM users WHERE id = $1`, id.UserID).Scan(&username, &email)
	if username != sub || email != sub+"@sem-email.invalid" {
		t.Fatalf("sem username/e-mail no token: usa o subject (%s, %s)", username, email)
	}
	// Mudou uma claim: novo provisionamento (mesma conta, dados atualizados).
	fed.Username, fed.Email, fed.Name = "maria."+sfx, "maria."+sfx+"@org.gov", ""
	again, err := r.Enrich(ctx, fed)
	if err != nil || again.UserID != id.UserID {
		t.Fatalf("mesma conta: %v %v", again.UserID, err)
	}
	var display string
	_ = pool.QueryRow(ctx, `SELECT display_name FROM users WHERE id = $1`, id.UserID).Scan(&display)
	if display != "Maria" {
		t.Fatalf("nome vazio no token não apaga o existente: %q", display)
	}
	// Conta local com o mesmo usuário: nunca vincula automaticamente.
	local := dbtest.User(t, pool)
	var localName string
	_ = pool.QueryRow(ctx, `SELECT username FROM users WHERE id = $1`, local).Scan(&localName)
	if _, err := r.Enrich(ctx, auth.Identity{Subject: "kc-" + uuid.NewString(), Source: auth.SourceKeycloak, Username: localName}); status(err) != http.StatusConflict {
		t.Fatalf("tomada de conta local: %v", err)
	}
	// Federado desativado.
	if _, err := pool.Exec(ctx, `UPDATE users SET active = false WHERE id = $1`, id.UserID); err != nil {
		t.Fatal(err)
	}
	fed.Name = "Outra"
	if _, err := r.Enrich(ctx, fed); status(err) != http.StatusForbidden {
		t.Fatalf("federado desativado: %v", err)
	}
}

func TestResolverDatabaseFailures(t *testing.T) {
	ctx := context.Background()
	r := NewResolver(nil, nil, quiet())
	r.db = dbtest.Fail{}
	for _, id := range []auth.Identity{
		{Subject: uuid.NewString(), Source: auth.SourceLocal},
		{Subject: "kc-x", Source: auth.SourceKeycloak},
	} {
		if _, err := r.Enrich(ctx, id); status(err) != http.StatusInternalServerError {
			t.Errorf("%s com o banco fora: %v", id.Source, err)
		}
	}
	for name, db := range map[string]*dbtest.Seq{
		"consulta":             {},
		"linha ilegível":       {Queries: []dbtest.QueryResult{{Rows: dbtest.BadRows()}}},
		"leitura interrompida": {Queries: []dbtest.QueryResult{{Rows: dbtest.ErrRows()}}},
	} {
		r.db = db
		if _, _, err := r.loadGrants(ctx, uuid.New(), auth.Identity{}); err == nil {
			t.Errorf("%s: permissões com falha", name)
		}
	}
	// Enrich devolve 500 se o cálculo das permissões falhar.
	pool := dbtest.Pool(t)
	uid := dbtest.User(t, pool)
	r.db = &grantsFail{DBTX: pool}
	if _, err := r.Enrich(ctx, auth.Identity{Subject: uid.String(), Source: auth.SourceLocal}); status(err) != http.StatusInternalServerError {
		t.Errorf("permissões com falha no Enrich: %v", err)
	}
}

// grantsFail lê a conta de verdade, mas a consulta de permissões falha.
type grantsFail struct{ database.DBTX }

func (grantsFail) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, dbtest.ErrInjected
}

func TestResolverCacheEvictionAndInvalidation(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	a := NewResolver(pool, rdb, quiet())
	b := NewResolver(pool, rdb, quiet())
	lctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- b.RunInvalidationListener(lctx) }()

	uid := dbtest.User(t, pool)
	id := auth.Identity{Subject: uid.String(), Source: auth.SourceLocal}
	if _, err := b.Enrich(ctx, id); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return mr.PubSubNumSub(redisKey())[redisKey()] == 1 })
	a.Invalidate(ctx) // outra réplica avisa
	waitFor(t, func() bool { b.mu.Lock(); defer b.mu.Unlock(); return len(b.cache) == 0 })
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	// Cache cheio recomeça do zero (sem crescer sem limite).
	for i := 0; i < maxCachedIdentites; i++ {
		a.cache[uuid.NewString()] = cached{expiresAt: time.Now().Add(time.Hour)}
	}
	if _, err := a.Enrich(ctx, id); err != nil || len(a.cache) != 1 {
		t.Fatalf("cache cheio: %d %v", len(a.cache), err)
	}

	// Redis fora: invalida localmente e só avisa (réplicas convergem pelo TTL).
	mr.Close()
	a.Invalidate(ctx)
	if len(a.cache) != 0 {
		t.Fatal("invalidação local mesmo com o Redis fora")
	}

	// Sem Redis: o listener só espera o shutdown.
	c2, cancel2 := context.WithCancel(ctx)
	cancel2()
	if err := NewResolver(pool, nil, quiet()).RunInvalidationListener(c2); err != nil {
		t.Fatal(err)
	}
	// Canal fechado pelo Redis: erro (o supervisor reinicia o listener).
	closed := make(chan *redis.Message)
	close(closed)
	a.subscribe = func(context.Context) (<-chan *redis.Message, func() error) {
		return closed, func() error { return nil }
	}
	if err := a.RunInvalidationListener(ctx); err == nil {
		t.Fatal("canal fechado deveria ser erro")
	}
}

func redisKey() string { return "nexus:" + invalidateChannel }

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	for i := 0; i < 300; i++ {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condição não atingida a tempo")
}
