package keycloakconfig

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newFakeKeycloak sobe um servidor HTTP local que responde ao discovery
// OIDC (/.well-known/openid-configuration) como um Keycloak de verdade
// responderia, e ao token_endpoint com tokenStatus — o suficiente para
// exercitar TestConnection de ponta a ponta sem depender de rede externa
// ou de um Keycloak real rodando.
func newFakeKeycloak(t *testing.T, tokenStatus int) *httptest.Server {
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
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(tokenStatus)
		if tokenStatus == http.StatusOK {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"fake","token_type":"Bearer","expires_in":300}`))
		}
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestTestConnection_DiscoveryOK_NoCredentials(t *testing.T) {
	srv := newFakeKeycloak(t, http.StatusUnauthorized)

	result := TestConnection(t.Context(), srv.URL, "", "", "")

	if !result.DiscoveryOK {
		t.Fatalf("esperava discovery OK, got %+v", result)
	}
	if result.Status != StatusOK {
		t.Errorf("sem client_id/secret informados, credenciais não deveriam ser checadas: %+v", result)
	}
	if result.CredentialsChecked {
		t.Error("CredentialsChecked deveria ser false sem client_id/secret")
	}
}

func TestTestConnection_DiscoveryFails_UnreachableIssuer(t *testing.T) {
	result := TestConnection(t.Context(), "http://127.0.0.1:1/realms/nao-existe", "", "", "")

	if result.DiscoveryOK {
		t.Fatal("esperava falha de discovery contra um issuer inalcançável")
	}
	if result.Status != StatusFailed {
		t.Errorf("Status deveria ser StatusFailed, got %q", result.Status)
	}
}

func TestTestConnection_DiscoveryFails_EmptyIssuer(t *testing.T) {
	result := TestConnection(t.Context(), "", "client", "secret", "aud")
	if result.DiscoveryOK || result.Status != StatusFailed {
		t.Fatalf("issuer vazio deveria falhar rápido, got %+v", result)
	}
}

func TestTestConnection_CredentialsOK(t *testing.T) {
	srv := newFakeKeycloak(t, http.StatusOK)

	result := TestConnection(t.Context(), srv.URL, "nexus-backend", "s3gr3d0", "nexus-backend")

	if !result.DiscoveryOK {
		t.Fatalf("esperava discovery OK: %+v", result)
	}
	if !result.CredentialsChecked || !result.CredentialsOK {
		t.Fatalf("esperava credenciais OK: %+v", result)
	}
	if result.Status != StatusOK {
		t.Errorf("Status deveria ser StatusOK, got %q", result.Status)
	}
}

func TestTestConnection_CredentialsFail_IsWarningNotFailure(t *testing.T) {
	srv := newFakeKeycloak(t, http.StatusUnauthorized)

	result := TestConnection(t.Context(), srv.URL, "nexus-backend", "senha-errada", "nexus-backend")

	if !result.DiscoveryOK {
		t.Fatalf("discovery deveria continuar OK mesmo com credenciais inválidas: %+v", result)
	}
	if !result.CredentialsChecked || result.CredentialsOK {
		t.Fatalf("esperava credenciais reportadas como falhas: %+v", result)
	}
	// Falha de credenciais é só aviso — client_credentials pode
	// legitimamente não estar habilitado no client (ver comentário em
	// tester.go); não pode virar StatusFailed nem bloquear o Save.
	if result.Status != StatusWarning {
		t.Errorf("Status deveria ser StatusWarning (não bloqueante), got %q", result.Status)
	}
}

func TestTestConnectionEdgeCases(t *testing.T) {
	ctx := context.Background()
	if r := TestConnection(ctx, "sso.orgao.gov.br/realms/x", "c", "s", ""); r.Status != StatusFailed {
		t.Fatalf("URL relativa: %+v", r)
	}
	for name, tokenEndpoint := range map[string]string{"sem token_endpoint": "", "token_endpoint inválido": "http://[::1", "token_endpoint inacessível": "http://127.0.0.1:1/token"} {
		var srv *httptest.Server
		mux := http.NewServeMux()
		mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
			doc := map[string]string{"issuer": srv.URL, "authorization_endpoint": srv.URL + "/auth", "jwks_uri": srv.URL + "/jwks"}
			if tokenEndpoint != "" {
				doc["token_endpoint"] = tokenEndpoint
			}
			_ = json.NewEncoder(w).Encode(doc)
		})
		srv = httptest.NewServer(mux)
		r := TestConnection(ctx, srv.URL, "c", "s", "")
		srv.Close()
		if !r.DiscoveryOK || r.Status != StatusWarning || r.CredentialsOK || r.CredentialsMessage == "" {
			t.Errorf("%s: %+v", name, r)
		}
	}
}
