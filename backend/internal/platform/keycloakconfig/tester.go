package keycloakconfig

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

// testTimeout limita quanto tempo TestConnection espera pelo discovery
// OIDC e pela tentativa de token — mesmo raciocínio de
// auth.discoveryTimeout: um issuer mal configurado (DNS que não resolve,
// firewall que faz a chamada cair no buraco negro) não pode travar o
// endpoint de teste indefinidamente.
const testTimeout = 10 * time.Second

// CheckStatus resume o resultado de TestConnection para a UI decidir
// como colorir o painel (verde/amarelo/vermelho).
type CheckStatus string

const (
	StatusOK      CheckStatus = "ok"
	StatusWarning CheckStatus = "warning"
	StatusFailed  CheckStatus = "failed"
)

// TestResult é o resultado de um teste de conexão contra um issuer
// Keycloak — nunca persiste nada, só reporta.
type TestResult struct {
	Status CheckStatus `json:"status"`

	// DiscoveryOK é a única checagem que de fato BLOQUEIA o salvamento
	// (ver Handlers.Save em transport.go): se o documento de discovery
	// OIDC não responde, a plataforma não teria como validar token
	// nenhum contra esse issuer.
	DiscoveryOK      bool   `json:"discovery_ok"`
	DiscoveryMessage string `json:"discovery_message"`

	// CredentialsChecked/CredentialsOK são best-effort e NUNCA bloqueiam
	// o salvamento: uma tentativa de client_credentials falhando é
	// esperada para um client Keycloak configurado só para Authorization
	// Code (o caso comum de um client "público" de frontend, ou de um
	// client de backend sem "Service Accounts" habilitado) — não é,
	// sozinho, evidência de client_id/client_secret errados.
	CredentialsChecked bool   `json:"credentials_checked"`
	CredentialsOK      bool   `json:"credentials_ok"`
	CredentialsMessage string `json:"credentials_message"`
}

// TestConnection tenta o discovery OIDC contra issuerURL e, se
// clientID+clientSecret forem informados, uma troca client_credentials
// contra o token_endpoint descoberto — puramente para diagnóstico, nunca
// grava nada. audience é aceito só para simetria com o restante da
// configuração (não participa de nenhuma checagem: quem valida audiência
// é o próprio auth.Verifier a cada token real, via oidc.Config.ClientID).
func TestConnection(ctx context.Context, issuerURL, clientID, clientSecret, audience string) TestResult {
	issuerURL = strings.TrimSpace(issuerURL)
	if issuerURL == "" {
		return TestResult{Status: StatusFailed, DiscoveryMessage: "Issuer URL não pode ser vazio"}
	}
	parsed, err := url.Parse(issuerURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return TestResult{Status: StatusFailed, DiscoveryMessage: "Issuer URL inválida — precisa ser uma URL absoluta (ex.: https://sso.orgao.gov.br/realms/nexus)"}
	}

	discoveryCtx, cancel := context.WithTimeout(ctx, testTimeout)
	defer cancel()

	provider, err := oidc.NewProvider(discoveryCtx, issuerURL)
	if err != nil {
		return TestResult{
			Status:           StatusFailed,
			DiscoveryOK:      false,
			DiscoveryMessage: "Falha no discovery OIDC (" + issuerURL + "/.well-known/openid-configuration): " + err.Error(),
		}
	}

	result := TestResult{
		Status:           StatusOK,
		DiscoveryOK:      true,
		DiscoveryMessage: "Discovery OIDC respondeu com sucesso — issuer alcançável e documento de configuração válido.",
	}

	clientID = strings.TrimSpace(clientID)
	if clientID == "" || clientSecret == "" {
		return result
	}

	result.CredentialsChecked = true
	tokenURL := provider.Endpoint().TokenURL
	if tokenURL == "" {
		result.CredentialsMessage = "Documento de discovery não informou um token_endpoint — não foi possível testar as credenciais."
		result.Status = StatusWarning
		return result
	}

	reqCtx, reqCancel := context.WithTimeout(ctx, testTimeout)
	defer reqCancel()

	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
	}
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		result.CredentialsMessage = "Não foi possível montar a requisição de teste: " + err.Error()
		result.Status = StatusWarning
		return result
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		result.CredentialsMessage = "Não foi possível contatar o token_endpoint: " + err.Error()
		result.Status = StatusWarning
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		result.CredentialsOK = true
		result.CredentialsMessage = "Client ID/Secret validados com sucesso via client_credentials."
		return result
	}

	result.CredentialsOK = false
	result.Status = StatusWarning
	result.CredentialsMessage = "O token_endpoint recusou client_credentials (HTTP " + resp.Status + "). " +
		"Isto é ESPERADO para um client configurado só para Authorization Code (login via navegador) " +
		"ou sem 'Service Accounts' habilitado no Keycloak — não bloqueia o salvamento, mas se você " +
		"esperava que isto funcionasse, confira Client ID/Secret e se 'Service Accounts Enabled' está ligado."
	return result
}
