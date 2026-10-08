package keycloakconfig

import (
	"context"
	"encoding/base64"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yurythx/projeto-nexus/internal/platform/secretcrypto"
)

// Estes testes rodam contra o PostgreSQL real usado no restante da suíte
// deste backend — pulados automaticamente se TEST_DATABASE_URL não
// estiver definida (mesmo padrão de
// internal/platform/configflags/postgres_test.go).
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL não definida; pulando teste de integração do PostgresStore")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func testCipher(t *testing.T) *secretcrypto.Cipher {
	t.Helper()
	key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("t", secretcrypto.KeySize)))
	c, err := secretcrypto.NewFromBase64Key(key)
	if err != nil {
		t.Fatalf("secretcrypto.NewFromBase64Key: %v", err)
	}
	return c
}

// keycloak_settings é um singleton (id='default', ver a migration
// 000004) — diferente de configflags (uma linha por chave), os testes
// aqui compartilham a MESMA linha, então cada teste precisa restaurar o
// estado (linha ausente = "nunca configurado") ao final, para não vazar
// para os demais testes deste pacote nem para uma suíte rodando em
// paralelo contra o mesmo banco.
func resetSingleton(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM keycloak_settings WHERE id = 'default'`)
	})
	_, err := pool.Exec(context.Background(), `DELETE FROM keycloak_settings WHERE id = 'default'`)
	if err != nil {
		t.Fatalf("limpar keycloak_settings antes do teste: %v", err)
	}
}

func TestPostgresStore_Get_NeverConfiguredReturnsZeroValue(t *testing.T) {
	pool := testPool(t)
	resetSingleton(t, pool)
	store := NewPostgresStore(pool, testCipher(t))

	got, err := store.Get(context.Background())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Configured {
		t.Fatal("esperava Configured=false quando nenhuma linha existe ainda")
	}
}

func TestPostgresStore_SetThenGet_RoundTripsAllFields(t *testing.T) {
	pool := testPool(t)
	resetSingleton(t, pool)
	store := NewPostgresStore(pool, testCipher(t))
	ctx := context.Background()

	in := Settings{
		IssuerURL:            "https://sso.orgao.gov.br/realms/nexus",
		Realm:                "nexus",
		ClientID:             "nexus-backend",
		ClientSecret:         "s3gr3d0-do-backend",
		Audience:             "nexus-backend",
		FrontendClientID:     "nexus-frontend",
		FrontendClientSecret: "s3gr3d0-do-frontend",
	}
	saved, err := store.Set(ctx, in, "admin-1")
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if !saved.Configured {
		t.Fatal("esperava Configured=true logo após Set com issuer_url não vazio")
	}
	if saved.ClientSecret != in.ClientSecret || saved.FrontendClientSecret != in.FrontendClientSecret {
		t.Fatalf("Set deveria devolver os segredos decifrados de volta: %+v", saved)
	}

	got, err := store.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != saved {
		t.Fatalf("Get após Set deveria devolver o mesmo estado: got %+v, want %+v", got, saved)
	}

	// O segredo nunca pode estar em texto plano na tabela — só a coluna
	// cifrada, e ela não pode ser igual ao plaintext original.
	var encrypted string
	if err := pool.QueryRow(ctx, `SELECT client_secret_encrypted FROM keycloak_settings WHERE id = 'default'`).Scan(&encrypted); err != nil {
		t.Fatalf("ler client_secret_encrypted: %v", err)
	}
	if encrypted == in.ClientSecret {
		t.Fatal("client_secret_encrypted está em texto plano na tabela — deveria estar cifrado")
	}
}

func TestPostgresStore_Set_EmptySecretKeepsExistingSecret(t *testing.T) {
	pool := testPool(t)
	resetSingleton(t, pool)
	store := NewPostgresStore(pool, testCipher(t))
	ctx := context.Background()

	_, err := store.Set(ctx, Settings{
		IssuerURL:    "https://sso.orgao.gov.br/realms/nexus",
		Realm:        "nexus",
		ClientID:     "nexus-backend",
		ClientSecret: "segredo-original",
		Audience:     "nexus-backend",
	}, "admin-1")
	if err != nil {
		t.Fatalf("Set inicial: %v", err)
	}

	// Segunda chamada só muda o Realm, client_secret vazio — deve MANTER
	// "segredo-original", nunca apagar.
	updated, err := store.Set(ctx, Settings{
		IssuerURL:    "https://sso.orgao.gov.br/realms/nexus",
		Realm:        "nexus-v2",
		ClientID:     "nexus-backend",
		ClientSecret: "",
		Audience:     "nexus-backend",
	}, "admin-1")
	if err != nil {
		t.Fatalf("Set de atualização: %v", err)
	}

	if updated.Realm != "nexus-v2" {
		t.Errorf("Realm deveria ter sido atualizado, got %q", updated.Realm)
	}
	if updated.ClientSecret != "segredo-original" {
		t.Errorf("ClientSecret deveria ter sido preservado quando enviado vazio, got %q", updated.ClientSecret)
	}
}

func TestPostgresStore_Get_WrongCipherKeyFailsLoudly(t *testing.T) {
	pool := testPool(t)
	resetSingleton(t, pool)
	store := NewPostgresStore(pool, testCipher(t))
	ctx := context.Background()

	if _, err := store.Set(ctx, Settings{
		IssuerURL:    "https://sso.orgao.gov.br/realms/nexus",
		Realm:        "nexus",
		ClientID:     "nexus-backend",
		ClientSecret: "segredo",
		Audience:     "nexus-backend",
	}, "admin-1"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	otherKey := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("z", secretcrypto.KeySize)))
	otherCipher, err := secretcrypto.NewFromBase64Key(otherKey)
	if err != nil {
		t.Fatalf("secretcrypto.NewFromBase64Key: %v", err)
	}
	storeWithWrongKey := NewPostgresStore(pool, otherCipher)

	if _, err := storeWithWrongKey.Get(ctx); err == nil {
		t.Fatal("Get com CONFIG_ENCRYPTION_KEY diferente da usada no Set deveria falhar, não devolver um segredo decifrado errado silenciosamente")
	}
}
