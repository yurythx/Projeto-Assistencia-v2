package keycloakconfig

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/platform/audit"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/config"
	"github.com/yurythx/projeto-nexus/pkg/httputil"
)

// ActionKeycloakConfigChanged é a ação registrada em audit_logs (§49)
// toda vez que a configuração do Keycloak é salva via PUT — nunca com o
// client secret no Metadata (ver Save abaixo).
const ActionKeycloakConfigChanged = "keycloak_config.changed"

// Handlers expõe a API administrativa de configuração do Keycloak — GET
// para o estado atual, POST /test para um teste de conexão sem persistir
// nada, PUT para salvar (que só persiste depois de um teste de discovery
// bem-sucedido, e então recarrega auth.Verifier em tempo real). Registrado
// atrás de auth.RequirePermission(auth.PermKeycloakManage) — só
// nexus-admin.
type Handlers struct {
	store       Store
	verifier    *auth.Verifier
	envFallback config.KeycloakConfig
	audit       *audit.Writer
	logger      *slog.Logger
}

func NewHandlers(store Store, verifier *auth.Verifier, envFallback config.KeycloakConfig, auditWriter *audit.Writer, logger *slog.Logger) *Handlers {
	return &Handlers{store: store, verifier: verifier, envFallback: envFallback, audit: auditWriter, logger: logger}
}

// statusResponse é o formato público do GET — nunca inclui um segredo em
// texto plano, só se um está definido (client_secret_set) para a UI
// mostrar um placeholder "••••••••" em vez do campo vazio.
type statusResponse struct {
	Source                  string `json:"source"` // "database" | "environment" | "unset"
	IssuerURL               string `json:"issuer_url"`
	Realm                   string `json:"realm"`
	ClientID                string `json:"client_id"`
	ClientSecretSet         bool   `json:"client_secret_set"`
	Audience                string `json:"audience"`
	FrontendClientID        string `json:"frontend_client_id"`
	FrontendClientSecretSet bool   `json:"frontend_client_secret_set"`
	UpdatedAt               string `json:"updated_at,omitempty"`
	UpdatedBy               string `json:"updated_by,omitempty"`
}

func (h *Handlers) toStatusResponse(s Settings) statusResponse {
	resp := statusResponse{
		IssuerURL:               s.IssuerURL,
		Realm:                   s.Realm,
		ClientID:                s.ClientID,
		ClientSecretSet:         s.ClientSecret != "",
		Audience:                s.Audience,
		FrontendClientID:        s.FrontendClientID,
		FrontendClientSecretSet: s.FrontendClientSecret != "",
	}
	if s.Configured {
		resp.Source = "database"
		resp.UpdatedAt = s.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")
		resp.UpdatedBy = s.UpdatedBy
		return resp
	}
	if h.envFallback.IssuerURL != "" {
		resp.Source = "environment"
		resp.IssuerURL = h.envFallback.IssuerURL
		resp.Realm = h.envFallback.Realm
		resp.ClientID = h.envFallback.ClientID
		resp.ClientSecretSet = h.envFallback.ClientSecret != ""
		resp.Audience = h.envFallback.Audience
		return resp
	}
	resp.Source = "unset"
	return resp
}

// Status trata GET /api/v1/admin/keycloak.
func (h *Handlers) Status(w http.ResponseWriter, r *http.Request) {
	settings, err := h.store.Get(r.Context())
	if err != nil {
		httputil.WriteError(w, r, h.logger, err)
		return
	}
	httputil.WriteOK(w, h.toStatusResponse(settings))
}

type testRequest struct {
	IssuerURL    string `json:"issuer_url" validate:"required,url"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Audience     string `json:"audience"`
}

// Test trata POST /api/v1/admin/keycloak/test — nunca persiste nada. Um
// client_secret vazio no corpo é tratado como "usar o segredo já salvo,
// se houver" (para o admin poder testar depois de mudar só a Issuer
// URL/Realm, sem precisar redigitar o segredo a cada teste).
func (h *Handlers) Test(w http.ResponseWriter, r *http.Request) {
	var req testRequest
	if err := httputil.DecodeJSON(w, r, &req); err != nil {
		httputil.WriteError(w, r, h.logger, err)
		return
	}
	if err := httputil.Validate(req); err != nil {
		httputil.WriteError(w, r, h.logger, err)
		return
	}

	secret := req.ClientSecret
	if secret == "" {
		// O segredo guardado só é reaproveitado para o MESMO issuer e client:
		// senão um teste contra um issuer qualquer entregaria o client
		// secret real ao token_endpoint dele.
		if current, err := h.store.Get(r.Context()); err == nil && current.Configured {
			if current.ClientID == req.ClientID && sameIssuer(current.IssuerURL, req.IssuerURL) {
				secret = current.ClientSecret
			}
		} else if h.envFallback.ClientID == req.ClientID && sameIssuer(h.envFallback.IssuerURL, req.IssuerURL) {
			secret = h.envFallback.ClientSecret
		}
	}

	result := TestConnection(r.Context(), req.IssuerURL, req.ClientID, secret, req.Audience)
	httputil.WriteOK(w, result)
}

type saveRequest struct {
	IssuerURL            string `json:"issuer_url" validate:"required,url"`
	Realm                string `json:"realm" validate:"required"`
	ClientID             string `json:"client_id" validate:"required"`
	ClientSecret         string `json:"client_secret"`
	Audience             string `json:"audience" validate:"required"`
	FrontendClientID     string `json:"frontend_client_id"`
	FrontendClientSecret string `json:"frontend_client_secret"`
}

// Save trata PUT /api/v1/admin/keycloak. Sempre testa o discovery OIDC
// contra o issuer informado ANTES de persistir (§ pedido explícito de
// "testar antes de salvar") — um issuer inalcançável nunca chega a virar
// configuração ativa, e auth.Verifier.Reload só é chamado depois que o
// novo estado já está gravado no Postgres, então uma falha no Reload
// (nunca deveria acontecer, já que acabamos de confirmar o discovery,
// mas defesa em profundidade) não deixa a configuração salva e o
// Verifier em runtime dessincronizados por muito tempo — o próximo
// restart do processo relê do Postgres e converge de qualquer forma (ver
// dependencies.go).
func (h *Handlers) Save(w http.ResponseWriter, r *http.Request) {
	var req saveRequest
	if err := httputil.DecodeJSON(w, r, &req); err != nil {
		httputil.WriteError(w, r, h.logger, err)
		return
	}
	if err := httputil.Validate(req); err != nil {
		httputil.WriteError(w, r, h.logger, err)
		return
	}

	// Resolve os segredos "vazio = manter o atual" ANTES do teste, para
	// que o teste de conexão valide exatamente o secret que vai ser
	// persistido — nunca um secret diferente do que efetivamente fica
	// salvo. Se ainda não existe nada salvo no Postgres (current não
	// Configured — primeira vez que este admin usa a tela), cai de volta
	// para o secret vindo de variável de ambiente: sem isto, "adotar" uma
	// configuração que já rodava via env var obrigaria a redigitar o
	// Client Secret mesmo sem ele ter mudado.
	current, err := h.store.Get(r.Context())
	if err != nil {
		httputil.WriteError(w, r, h.logger, err)
		return
	}
	clientSecret := req.ClientSecret
	if clientSecret == "" {
		// "Vazio = manter o atual" só vale para o MESMO issuer: trocar o
		// issuer mantendo o segredo o entregaria ao novo token_endpoint (e
		// o persistiria apontando para outro servidor).
		keep, keptIssuer := current.ClientSecret, current.IssuerURL
		if !current.Configured {
			keep, keptIssuer = h.envFallback.ClientSecret, h.envFallback.IssuerURL
		}
		if keep != "" && !sameIssuer(keptIssuer, req.IssuerURL) {
			httputil.WriteError(w, r, h.logger, apperrors.Validation(
				"ao trocar o Issuer URL, informe o Client Secret de novo (o atual não é reaproveitado para outro servidor)"))
			return
		}
		clientSecret = keep
	}
	// O "vazio = manter o atual" do frontend_client_secret é aplicado por
	// Store.Set (mesma regra do client_secret, ver o comentário em
	// `incoming` abaixo) — não há teste de discovery do lado frontend que
	// precise resolvê-lo aqui.

	test := TestConnection(r.Context(), req.IssuerURL, req.ClientID, clientSecret, req.Audience)
	if !test.DiscoveryOK {
		httputil.WriteError(w, r, h.logger, apperrors.Validation(
			"Não foi possível salvar: "+test.DiscoveryMessage,
		))
		return
	}

	incoming := Settings{
		IssuerURL:            req.IssuerURL,
		Realm:                req.Realm,
		ClientID:             req.ClientID,
		ClientSecret:         req.ClientSecret, // "vazio = manter" — Store.Set aplica a mesma regra
		Audience:             req.Audience,
		FrontendClientID:     req.FrontendClientID,
		FrontendClientSecret: req.FrontendClientSecret,
	}

	if !current.Configured {
		// Primeiro salvamento "adotando" a configuração do .env: grava o
		// segredo que acabou de ser testado — antes ia vazio, e a
		// reautenticação federada (Signum) passava a ler um segredo vazio.
		incoming.ClientSecret = clientSecret
	}

	var updatedBy string
	if identity, ok := auth.IdentityFromContext(r.Context()); ok {
		updatedBy = identity.Subject
	}

	saved, err := h.store.Set(r.Context(), incoming, updatedBy)
	if err != nil {
		httputil.WriteError(w, r, h.logger, err)
		return
	}

	reloadErr := h.verifier.Reload(r.Context(), saved.ToKeycloakConfig())
	if reloadErr != nil {
		// Já persistiu (o admin vê a configuração salva corretamente),
		// mas o processo atual não conseguiu recarregar o verifier — o
		// mais provável é o discovery ter funcionado no teste e falhado
		// agora por uma instabilidade transitória de rede. Reportamos o
		// aviso mas não desfazemos o save: outra réplica da API (ou esta
		// mesma, na próxima chamada de Reload) pode ter sucesso, e o
		// próximo restart do processo relê do Postgres de qualquer jeito.
		h.logger.Error("keycloakconfig: configuração salva, mas falha ao recarregar o verifier em runtime",
			slog.String("erro", reloadErr.Error()))
	}

	if h.audit != nil {
		// audit.FromRequest preenche ip_address e correlation_id (gap G-07
		// da auditoria de conformidade — trocar o issuer OIDC afeta a
		// autenticação de TODA a plataforma; §49 exige o IP de origem em
		// toda operação de escrita).
		entry := audit.FromRequest(r)
		entry.Action = ActionKeycloakConfigChanged
		entry.ResourceType = "keycloak_config"
		entry.ResourceID = "default"
		entry.Metadata = map[string]any{
			// NUNCA client_secret/frontend_client_secret aqui.
			"issuer_url":         saved.IssuerURL,
			"realm":              saved.Realm,
			"client_id":          saved.ClientID,
			"audience":           saved.Audience,
			"frontend_client_id": saved.FrontendClientID,
			"updated_by":         updatedBy,
			"verifier_reloaded":  reloadErr == nil,
		}
		_ = h.audit.Record(r.Context(), entry)
	}

	httputil.WriteOK(w, map[string]any{
		"settings": h.toStatusResponse(saved),
		"test":     test,
	})
}

// RegisterRoutes monta as rotas administrativas de configuração do
// Keycloak. r já deve estar atrás de auth.RequireAuthentication; este
// método adiciona por cima a exigência de auth.PermKeycloakManage
// (nexus-admin) para toda rota — mesmo o teste de conexão, já que ele
// aceita um client_secret no corpo e faz uma chamada de rede de saída
// com ele.
func RegisterRoutes(r chi.Router, h *Handlers, logger *slog.Logger) {
	r.Route("/admin/keycloak", func(admin chi.Router) {
		admin.Use(auth.RequirePermission(logger, auth.PermKeycloakManage))
		admin.Get("/", h.Status)
		admin.Post("/test", h.Test)
		admin.Put("/", h.Save)
	})
}

// sameIssuer compara issuers ignorando espaços e a barra final.
func sameIssuer(a, b string) bool {
	norm := func(s string) string { return strings.TrimRight(strings.TrimSpace(s), "/") }
	return norm(a) == norm(b)
}
