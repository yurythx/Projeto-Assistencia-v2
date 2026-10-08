package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yurythx/projeto-nexus/internal/modules/signum"
	tramiteDomain "github.com/yurythx/projeto-nexus/internal/modules/tramite/domain"
)

func TestSignaturePortPropagatesSignumErrors(t *testing.T) {
	d := testDeps(t)
	var mod *signum.Module
	for _, p := range d.Kernel.Plugins() {
		if m, ok := p.(*signum.Module); ok {
			mod = m
		}
	}
	port := &signaturePort{svc: mod.Service(), enabled: func() bool { return true }}
	if !port.Available() {
		t.Fatal("porta indisponível")
	}
	tx, err := d.DB.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	// sem signatários: o Signum recusa e o erro chega ao Trâmite
	if _, err := port.OpenEnvelope(context.Background(), tx, tramiteDomain.SignatureRequest{Title: "x"}); err == nil {
		t.Fatal("envelope sem signatários aceito")
	}
}

func TestKeycloakCredentialsFallBackToEnv(t *testing.T) {
	d := testDeps(t)
	_, _ = d.DB.Exec(context.Background(), `DELETE FROM keycloak_settings WHERE id = 'default'`)
	d.Config.Keycloak.IssuerURL, d.Config.Keycloak.ClientID, d.Config.Keycloak.ClientSecret = "https://env/realms/n", "cli", "seg"
	if i, c, s := d.keycloakCredentials(context.Background()); i != "https://env/realms/n" || c != "cli" || s != "seg" {
		t.Fatalf("credenciais = %q %q %q", i, c, s)
	}
}

func TestIdentityRateLimitKeyFallsBackToIP(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "198.51.100.7:4000"
	if k := identityRateLimitKey(r); !strings.HasPrefix(k, "ip:") {
		t.Fatalf("chave = %q", k)
	}
}

func TestOpenAPIAndSwaggerUIAreServed(t *testing.T) {
	h := newHarness(t)
	t.Chdir("../..") // o binário roda da raiz do backend (docs/openapi.json)
	rec := h.do(http.MethodGet, "/openapi.json", "", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"openapi"`) {
		t.Fatalf("/openapi.json = %d", rec.Code)
	}
	rec = h.do(http.MethodGet, "/docs", "", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") ||
		!strings.Contains(rec.Body.String(), "swagger-ui") {
		t.Fatalf("/docs = %d %v", rec.Code, rec.Header())
	}
}
