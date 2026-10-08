package keycloakconfig

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
	"github.com/yurythx/projeto-nexus/internal/platform/secretcrypto"
)

func otherCipher(t *testing.T) *secretcrypto.Cipher {
	c, err := secretcrypto.NewFromBase64Key(base64.StdEncoding.EncodeToString([]byte(strings.Repeat("z", secretcrypto.KeySize))))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestStoreFailures(t *testing.T) {
	ctx := context.Background()
	down := &PostgresStore{db: dbtest.Fail{}, cipher: testCipher(t)}
	if _, err := down.Get(ctx); err == nil {
		t.Error("get com o banco fora")
	}
	if _, err := down.Set(ctx, Settings{IssuerURL: "https://x"}, "a"); err == nil {
		t.Error("set com o banco fora")
	}

	pool := testPool(t)
	resetSingleton(t, pool)
	good := NewPostgresStore(pool, testCipher(t))
	// Só o segredo do frontend cifrado com outra chave: o Get falha nele.
	if _, err := good.Set(ctx, Settings{IssuerURL: "https://sso/realms/n", Realm: "n", ClientID: "c", ClientSecret: "s", Audience: "c"}, "a"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE keycloak_settings SET frontend_client_secret_encrypted = $1 WHERE id = 'default'`, otherCipher(t).Encrypt("f")); err != nil {
		t.Fatal(err)
	}
	if _, err := good.Get(ctx); err == nil || !strings.Contains(err.Error(), "frontend_client_secret") {
		t.Fatalf("segredo do frontend indecifrável: %v", err)
	}
	// Set mantendo segredos guardados com outra chave: a leitura de volta falha em cada um.
	if _, err := good.Set(ctx, Settings{IssuerURL: "https://sso/realms/n", Realm: "n", ClientID: "c", ClientSecret: "s2", Audience: "c"}, "a"); err == nil || !strings.Contains(err.Error(), "frontend_client_secret after set") {
		t.Fatalf("segredo do frontend guardado com outra chave: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE keycloak_settings SET client_secret_encrypted = $1 WHERE id = 'default'`, otherCipher(t).Encrypt("s")); err != nil {
		t.Fatal(err)
	}
	if _, err := good.Set(ctx, Settings{IssuerURL: "https://sso/realms/n", Realm: "n", ClientID: "c", Audience: "c"}, "a"); err == nil || !strings.Contains(err.Error(), "client_secret after set") {
		t.Fatalf("segredo do backend guardado com outra chave: %v", err)
	}
}
