package auth

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"

	"github.com/yurythx/projeto-nexus/internal/platform/config"
)

// discoveryTimeout limita quanto tempo NewVerifier espera pelo documento
// de discovery OIDC. Sem um teto, um issuer configurado errado (DNS que
// nunca resolve, uma rota que faz o pacote cair no buraco negro) faria o
// processo travar indefinidamente no startup, em vez de "falhar rápido"
// como o restante deste construtor promete.
const discoveryTimeout = 10 * time.Second

// verifierState é o que muda a cada Reload — deliberadamente separado de
// Verifier para poder ser trocado atomicamente (atomic.Pointer) sem lock:
// um Reload em andamento nunca deixa uma requisição concorrente ver um
// estado parcialmente construído, e Verify nunca bloqueia esperando um
// Reload terminar (lê o ponteiro antigo até o novo ser publicado).
type verifierState struct {
	idTokenVerifier *oidc.IDTokenVerifier
	clientID        string
}

// Verifier valida access tokens contra o realm do Keycloak configurado
// e, opcionalmente, contra tokens locais assinados pelo próprio backend
// (§ Sistema de Login Local — sempre um caminho ADICIONAL, nunca um
// substituto do Keycloak). Faz o discovery OIDC no startup e mantém o
// JWKS em cache no próprio processo (só é atualizado quando aparece um
// key id desconhecido, conforme a implementação de remote key set do
// go-oidc) — sem nenhuma chamada ao Keycloak por requisição (§29).
//
// O lado Keycloak é recarregável em tempo de execução via Reload —
// usado por internal/platform/keycloakconfig quando um administrador
// salva uma nova configuração pelo menu Configurações > Keycloak, para
// que a mudança valha imediatamente, sem reiniciar o processo. localSigner
// nunca muda depois de NewVerifier (o login local não faz parte deste
// recurso de configuração dinâmica).
type Verifier struct {
	state       atomic.Pointer[verifierState]
	localSigner *LocalSigner
}

// buildState faz o discovery OIDC contra cfg.IssuerURL e monta o
// verifierState correspondente. cfg.IssuerURL vazio é um estado válido
// (nenhum verifier Keycloak — só o local, se localSigner existir), nunca
// um erro por si só; ver a checagem de "Keycloak inalcançável" mais
// abaixo, feita por quem chama (NewVerifier no boot, Reload em runtime).
func buildState(ctx context.Context, cfg config.KeycloakConfig) (*verifierState, error) {
	if cfg.IssuerURL == "" {
		return &verifierState{idTokenVerifier: nil, clientID: cfg.ClientID}, nil
	}

	discoveryCtx, cancel := context.WithTimeout(ctx, discoveryTimeout)
	defer cancel()

	provider, err := oidc.NewProvider(discoveryCtx, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("auth: OIDC discovery failed for issuer %q: %w", cfg.IssuerURL, err)
	}

	verifier := provider.VerifierContext(ctx, &oidc.Config{
		ClientID:             cfg.Audience,
		SupportedSigningAlgs: []string{oidc.RS256, oidc.RS384, oidc.RS512, oidc.ES256},
	})

	return &verifierState{idTokenVerifier: verifier, clientID: cfg.ClientID}, nil
}

// NewVerifier faz o discovery OIDC contra cfg.IssuerURL. Falha rápido
// (retorna um erro) se o issuer estiver inalcançável ou malformado, para
// que uma configuração errada apareça no startup em vez de na primeira
// requisição que precisar validar um token. localSigner pode ser nil —
// nesse caso Verify nunca tenta o caminho local, só Keycloak, exatamente
// como antes deste recurso existir. Note que localSigner é *outra* chave,
// independente de qualquer coisa relacionada a cfg (Keycloak): os dois
// caminhos de autenticação nunca compartilham material criptográfico.
func NewVerifier(ctx context.Context, cfg config.KeycloakConfig, localSigner *LocalSigner) (*Verifier, error) {
	if cfg.IssuerURL == "" && localSigner == nil {
		return nil, fmt.Errorf("auth: Keycloak IssuerURL is empty and LocalAuth is disabled")
	}

	state, err := buildState(ctx, cfg)
	if err != nil {
		return nil, err
	}

	v := &Verifier{localSigner: localSigner}
	v.state.Store(state)
	return v, nil
}

// Reload refaz o discovery OIDC contra a nova cfg e, se bem-sucedido,
// substitui atomicamente o verifier Keycloak em uso — toda requisição em
// andamento termina de validar contra o estado antigo (nunca um valor
// parcialmente trocado), e toda requisição nova já vê o estado novo.
// Chamado por internal/platform/keycloakconfig depois que um admin salva
// uma configuração nova (só depois que ela já passou por um teste de
// conexão bem-sucedido — ver keycloakconfig.TestConnection) — um Reload
// que falhasse deixaria o processo sem verificar tokens Keycloak
// nenhuns, então o chamador nunca troca a configuração persistida sem
// primeiro confirmar, por este mesmo caminho, que o novo issuer responde.
func (v *Verifier) Reload(ctx context.Context, cfg config.KeycloakConfig) error {
	state, err := buildState(ctx, cfg)
	if err != nil {
		return err
	}
	v.state.Store(state)
	return nil
}

// Verify valida a assinatura, o issuer, a audiência, a expiração e o
// algoritmo de rawToken, e então extrai a Identity da plataforma a partir
// das suas claims. Nunca chama o Keycloak — a verificação é inteiramente
// local, contra o JWKS em cache.
//
// Tenta primeiro o Keycloak (o caminho principal e sempre ativo); se isso
// falhar e o login local estiver habilitado, tenta a verificação RS256
// local (com a chave própria de LocalSigner) antes de desistir. Um token
// de verdade do Keycloak nunca passa na verificação local por acidente, e
// vice-versa — as duas chaves RSA são inteiramente independentes (uma
// pertence ao realm do Keycloak, a outra só existe neste processo), então
// não há ambiguidade possível sobre qual token "pertence" a qual caminho.
func (v *Verifier) Verify(ctx context.Context, rawToken string) (Identity, error) {
	// Lido uma única vez no início da chamada: mesmo que um Reload troque
	// v.state entre esta leitura e o fim da função, esta requisição
	// termina de validar contra um estado inteiro e consistente (nunca um
	// idTokenVerifier de uma geração com o clientID de outra).
	state := v.state.Load()

	var keycloakErr error
	if state.idTokenVerifier != nil {
		token, err := state.idTokenVerifier.Verify(ctx, rawToken)
		if err == nil {
			var claims accessTokenClaims
			if err := token.Claims(&claims); err != nil {
				return Identity{}, fmt.Errorf("auth: decode token claims: %w", err)
			}
			return claims.toIdentity(state.clientID), nil
		}
		keycloakErr = err
	}

	if v.localSigner == nil {
		if keycloakErr != nil {
			return Identity{}, fmt.Errorf("auth: token verification failed: %w", keycloakErr)
		}
		return Identity{}, fmt.Errorf("auth: no token verifiers available")
	}

	identity, localErr := v.localSigner.verifyToken(rawToken)
	if localErr != nil {
		if keycloakErr != nil {
			return Identity{}, fmt.Errorf("auth: token verification failed (keycloak: %v, local: %w)", keycloakErr, localErr)
		}
		return Identity{}, fmt.Errorf("auth: local token verification failed: %w", localErr)
	}
	return identity, nil
}
