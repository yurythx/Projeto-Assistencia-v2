package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/yurythx/projeto-nexus/internal/platform/config"
)

func TestParseRSAPrivateKeyFormats(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	pkcs8, _ := x509.MarshalPKCS8PrivateKey(key)
	if _, err := parseRSAPrivateKeyPEM(string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}))); err != nil {
		t.Fatalf("PKCS8 RSA aceito: %v", err)
	}
	ec, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	ecDER, _ := x509.MarshalPKCS8PrivateKey(ec)
	if _, err := parseRSAPrivateKeyPEM(string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: ecDER}))); err == nil || !strings.Contains(err.Error(), "not an RSA") {
		t.Fatalf("chave EC recusada: %v", err)
	}
	if _, err := parseRSAPrivateKeyPEM(string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("lixo")}))); err == nil {
		t.Fatal("DER inválido recusado")
	}
}

func TestIssueTokenPropagatesSigningFailure(t *testing.T) {
	// Chave de 512 bits montada à mão: o crypto/rsa recusa assinar.
	p, _ := rand.Prime(rand.Reader, 256)
	q, _ := rand.Prime(rand.Reader, 256)
	phi := new(big.Int).Mul(new(big.Int).Sub(p, big.NewInt(1)), new(big.Int).Sub(q, big.NewInt(1)))
	weak := &rsa.PrivateKey{PublicKey: rsa.PublicKey{N: new(big.Int).Mul(p, q), E: 65537},
		D: new(big.Int).ModInverse(big.NewInt(65537), phi), Primes: []*big.Int{p, q}}
	weak.Precompute()
	s := &LocalSigner{privateKey: weak, tokenTTL: time.Minute}
	if tok, _, err := s.IssueToken(LocalAccount{ID: "x"}); err == nil || tok != "" {
		t.Fatalf("falha de assinatura não pode virar token vazio: %q %v", tok, err)
	}
}

func TestOptionalAuthentication(t *testing.T) {
	provider := newTestOIDCProvider(t)
	v := newTestVerifier(t, provider, "nexus", "nexus")
	good := provider.signToken(t, tokenOpts{subject: "s1", preferredUsername: "ana", audience: "nexus", expiresAt: time.Now().Add(time.Hour)})
	echo := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id, ok := IdentityFromContext(r.Context()); ok {
			_, _ = w.Write([]byte("user:" + id.Username))
			return
		}
		_, _ = w.Write([]byte("anon"))
	})
	call := func(enrich Enricher, header string) string {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		rec := httptest.NewRecorder()
		OptionalAuthentication(v, enrich)(echo).ServeHTTP(rec, req)
		return rec.Body.String()
	}
	fail := func(context.Context, Identity) (Identity, error) { return Identity{}, errors.New("conta desativada") }
	ok := func(_ context.Context, id Identity) (Identity, error) { return id, nil }
	for name, c := range map[string]struct {
		enrich Enricher
		header string
		want   string
	}{
		"sem token":        {nil, "", "anon"},
		"token inválido":   {nil, "Bearer lixo", "anon"},
		"conta recusada":   {fail, "Bearer " + good, "anon"},
		"com enriquecedor": {ok, "Bearer " + good, "user:ana"},
		"sem enriquecedor": {nil, "Bearer " + good, "user:ana"},
	} {
		if got := call(c.enrich, c.header); got != c.want {
			t.Errorf("%s: %s", name, got)
		}
	}
	if _, err := bearerToken(&http.Request{Header: http.Header{"Authorization": {"Bearer    "}}}); err == nil {
		t.Error("bearer vazio")
	}
}

func TestVerifierPaths(t *testing.T) {
	ctx := context.Background()
	if _, err := NewVerifier(ctx, config.KeycloakConfig{IssuerURL: "http://127.0.0.1:1/realms/x", ClientID: "c"}, nil); err == nil {
		t.Fatal("issuer inacessível no boot")
	}
	provider := newTestOIDCProvider(t)
	v := newTestVerifier(t, provider, "nexus", "nexus")
	// Token válido do Keycloak com claims em formato inesperado.
	weird := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss": provider.issuer, "sub": "s", "aud": "nexus", "exp": time.Now().Add(time.Hour).Unix(),
		"realm_access": "não é objeto",
	})
	weird.Header["kid"] = provider.kid
	raw, _ := weird.SignedString(provider.privateKey)
	if _, err := v.Verify(ctx, raw); err == nil || !strings.Contains(err.Error(), "claims") {
		t.Fatalf("claims ilegíveis: %v", err)
	}
	if _, err := v.Verify(ctx, "lixo"); err == nil || !strings.Contains(err.Error(), "token verification failed") {
		t.Fatalf("só Keycloak, token inválido: %v", err)
	}
	if _, err := (&Verifier{}).verifyWithEmptyState(ctx); err == nil {
		t.Fatal("sem verificador algum")
	}

	signer := testSigner(t, time.Hour)
	both, err := NewVerifier(ctx, config.KeycloakConfig{IssuerURL: provider.issuer, ClientID: "nexus", Audience: "nexus"}, signer)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := both.Verify(ctx, "lixo"); err == nil || !strings.Contains(err.Error(), "keycloak:") {
		t.Fatalf("os dois caminhos falham: %v", err)
	}
	localOnly, err := NewVerifier(ctx, config.KeycloakConfig{}, signer)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := localOnly.Verify(ctx, "lixo"); err == nil || !strings.Contains(err.Error(), "local token verification failed") {
		t.Fatalf("só local: %v", err)
	}
}

func (v *Verifier) verifyWithEmptyState(ctx context.Context) (Identity, error) {
	v.state.Store(&verifierState{})
	return v.Verify(ctx, "x")
}

func TestRequireAuthenticationEnrichAndPermission(t *testing.T) {
	provider := newTestOIDCProvider(t)
	v := newTestVerifier(t, provider, "nexus", "nexus")
	tok := provider.signToken(t, tokenOpts{subject: "s1", preferredUsername: "ana", audience: "nexus", expiresAt: time.Now().Add(time.Hour)})
	withPerm := func(_ context.Context, id Identity) (Identity, error) {
		id.Permissions = []string{"blog:manage"}
		return id, nil
	}
	denied := func(context.Context, Identity) (Identity, error) { return Identity{}, errors.New("desativada") }
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := Require(r.Context()); err != nil {
			t.Error("identidade no contexto")
		}
		w.WriteHeader(http.StatusNoContent)
	})
	for name, c := range map[string]struct {
		enrich Enricher
		want   int
	}{
		"permitido":      {withPerm, http.StatusNoContent},
		"sem permissão":  {nil, http.StatusForbidden},
		"conta recusada": {denied, http.StatusInternalServerError},
	} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		RequireAuthentication(v, c.enrich, quietLogger())(RequirePermission(quietLogger(), "blog:manage")(ok)).ServeHTTP(rec, req)
		if rec.Code != c.want {
			t.Errorf("%s: %d", name, rec.Code)
		}
	}
	if _, err := Require(context.Background()); err == nil {
		t.Error("sem identidade: 401")
	}
}

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }
