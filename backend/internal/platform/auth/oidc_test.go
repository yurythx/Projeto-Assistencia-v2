package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yurythx/projeto-nexus/internal/platform/config"
)

// newFakeOIDCIssuer sobe um servidor HTTP local que responde ao
// discovery OIDC (/.well-known/openid-configuration) como um Keycloak de
// verdade responderia — o suficiente para exercitar NewVerifier/Reload
// sem depender de rede externa ou de um Keycloak real rodando.
func newFakeOIDCIssuer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 srv.URL,
			"authorization_endpoint": srv.URL + "/auth",
			"token_endpoint":         srv.URL + "/token",
			"jwks_uri":               srv.URL + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys":[]}`))
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestNewVerifier_EmptyIssuerAndNoLocalSignerFails(t *testing.T) {
	if _, err := NewVerifier(t.Context(), config.KeycloakConfig{}, nil); err == nil {
		t.Fatal("esperava erro: sem issuer Keycloak e sem local signer, não haveria como verificar token nenhum")
	}
}

func TestNewVerifier_EmptyIssuerWithLocalSignerFallsBackToLocalOnly(t *testing.T) {
	signer := testSigner(t, time.Hour)

	v, err := NewVerifier(t.Context(), config.KeycloakConfig{}, signer)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}

	account := LocalAccount{ID: "user-1", Username: "nexus-user-1", Roles: []string{RoleUser}}
	token, _, err := signer.IssueToken(account)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	got, err := v.Verify(t.Context(), token)
	if err != nil {
		t.Fatalf("Verify deveria aceitar um token local válido sem verifier Keycloak: %v", err)
	}
	if got.Subject != account.ID {
		t.Errorf("Subject = %q, want %q", got.Subject, account.ID)
	}
}

// TestVerifier_Reload_AppliesNewIssuerWithoutRestart cobre o recurso
// central da configuração dinâmica do Keycloak (ver
// internal/platform/keycloakconfig): salvar uma configuração nova precisa
// valer para a PRÓXIMA verificação de token, no mesmo processo, sem
// reconstruir o Verifier inteiro.
func TestVerifier_Reload_AppliesNewIssuerWithoutRestart(t *testing.T) {
	srv := newFakeOIDCIssuer(t)

	// Começa sem nenhum issuer Keycloak (só local) — exatamente o estado
	// de um ambiente que nunca configurou Keycloak via env var.
	signer := testSigner(t, time.Hour)
	v, err := NewVerifier(t.Context(), config.KeycloakConfig{}, signer)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}

	stateBefore := v.state.Load()
	if stateBefore.idTokenVerifier != nil {
		t.Fatal("estado inicial não deveria ter um verifier Keycloak")
	}

	if err := v.Reload(t.Context(), config.KeycloakConfig{
		IssuerURL: srv.URL,
		ClientID:  "nexus-backend",
		Audience:  "nexus-backend",
	}); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	stateAfter := v.state.Load()
	if stateAfter.idTokenVerifier == nil {
		t.Fatal("depois de Reload com um issuer válido, o estado deveria ter um verifier Keycloak")
	}
	if stateAfter == stateBefore {
		t.Fatal("Reload deveria substituir o ponteiro de estado, não mutar o antigo in-place")
	}

	// O caminho local continua funcionando depois do Reload — ele nunca
	// muda o localSigner, só o lado Keycloak.
	account := LocalAccount{ID: "user-2", Username: "nexus-user-2", Roles: []string{RoleUser}}
	token, _, err := signer.IssueToken(account)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	if _, err := v.Verify(t.Context(), token); err != nil {
		t.Fatalf("Verify de um token local deveria continuar funcionando após Reload: %v", err)
	}
}

// TestVerifier_Reload_FailureLeavesPreviousStateIntact cobre o requisito
// de segurança do fluxo de salvamento: um Reload que falha (issuer
// inalcançável) nunca pode deixar o processo sem NENHUM verifier
// Keycloak — o estado anterior, que já estava funcionando, continua em
// uso até um Reload bem-sucedido.
func TestVerifier_Reload_FailureLeavesPreviousStateIntact(t *testing.T) {
	srv := newFakeOIDCIssuer(t)

	v, err := NewVerifier(t.Context(), config.KeycloakConfig{
		IssuerURL: srv.URL,
		ClientID:  "nexus-backend",
		Audience:  "nexus-backend",
	}, nil)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	stateBefore := v.state.Load()

	err = v.Reload(t.Context(), config.KeycloakConfig{
		IssuerURL: "http://127.0.0.1:1/realms/nao-existe",
		ClientID:  "nexus-backend",
		Audience:  "nexus-backend",
	})
	if err == nil {
		t.Fatal("esperava erro ao recarregar contra um issuer inalcançável")
	}

	if v.state.Load() != stateBefore {
		t.Fatal("um Reload que falha não pode substituir o estado anterior (deixaria o processo sem verificar Keycloak nenhum)")
	}
}
