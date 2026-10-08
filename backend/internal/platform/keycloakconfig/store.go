// Package keycloakconfig implementa a configuração do Keycloak editável
// em tempo de execução pelo nexus-admin (menu Configurações > Keycloak /
// IAM) — o mesmo espírito de internal/platform/configflags (persistida no
// Postgres, sem cache, alterável sem reiniciar o processo), mas para os
// parâmetros de conexão OIDC em vez de interruptores booleanos.
//
// A peça que torna isto seguro é que salvar NUNCA aplica um valor sem
// antes confirmar, com uma tentativa de discovery OIDC real (ver
// tester.go), que o issuer configurado responde — e o efeito só é
// aplicado ao restante da plataforma através de auth.Verifier.Reload,
// nunca lendo esta tabela diretamente de dentro do middleware de
// autenticação (isso manteria uma consulta ao Postgres em TODO request
// autenticado, o oposto do que auth.Verifier já otimiza: JWKS em cache,
// zero chamada de rede por requisição).
package keycloakconfig

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yurythx/projeto-nexus/internal/platform/config"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/secretcrypto"
)

// Settings é o estado persistido (e já decifrado, quando lido por Get)
// da configuração do Keycloak. Configured reporta se já existe uma
// configuração de fato salva (issuer_url não vazio) — antes da primeira
// vez que um admin salva algo por aqui, a plataforma continua rodando
// inteiramente a partir de variável de ambiente (config.Load), e Get
// devolve Settings{Configured: false} para o admin ver "nada configurado
// ainda, mostrando o que vem do ambiente" na tela.
type Settings struct {
	IssuerURL            string
	Realm                string
	ClientID             string
	ClientSecret         string
	Audience             string
	FrontendClientID     string
	FrontendClientSecret string
	UpdatedAt            time.Time
	UpdatedBy            string
	Configured           bool
}

// ToKeycloakConfig converte pro formato que auth.Verifier.Reload/NewVerifier
// espera — usado tanto no boot (dependencies.go, reaplicando uma
// configuração já salva antes de aceitar tráfego) quanto depois de um
// PUT bem-sucedido (transport.go), que antes duplicavam este mesmo
// literal em dois lugares.
func (s Settings) ToKeycloakConfig() config.KeycloakConfig {
	return config.KeycloakConfig{
		IssuerURL:    s.IssuerURL,
		Realm:        s.Realm,
		ClientID:     s.ClientID,
		ClientSecret: s.ClientSecret,
		Audience:     s.Audience,
	}
}

// Store persiste e consulta a configuração do Keycloak. Handlers depende
// desta interface, não de *PostgresStore diretamente, para poder ser
// testado com um fake em memória sem subir Postgres.
type Store interface {
	// Get devolve a configuração salva, já decifrada, ou
	// Settings{Configured: false} se nenhuma linha existir ainda (ou a
	// linha existir com issuer_url vazio — equivalente a "nunca
	// configurado").
	Get(ctx context.Context) (Settings, error)

	// Set faz upsert da configuração. Um ClientSecret/FrontendClientSecret
	// vazio em incoming significa "manter o segredo já salvo" (a tela
	// nunca reexibe o segredo em texto plano para o admin reenviar de
	// volta — ver transport.go) — só uma string não-vazia sobrescreve o
	// valor cifrado existente. updatedBy é o subject do administrador
	// (auditoria).
	Set(ctx context.Context, incoming Settings, updatedBy string) (Settings, error)
}

// PostgresStore implementa Store sobre a tabela keycloak_settings
// (migration 000004), cifrando/decifrando os campos de segredo com
// cipher (ver internal/platform/secretcrypto).
type PostgresStore struct {
	db     database.DBTX // o pool, em produção
	cipher *secretcrypto.Cipher
}

func NewPostgresStore(pool *pgxpool.Pool, cipher *secretcrypto.Cipher) *PostgresStore {
	return &PostgresStore{db: pool, cipher: cipher}
}

var _ Store = (*PostgresStore)(nil)

func (s *PostgresStore) Get(ctx context.Context) (Settings, error) {
	const q = `
		SELECT issuer_url, realm, client_id, client_secret_encrypted, audience,
		       frontend_client_id, frontend_client_secret_encrypted, updated_at, updated_by
		FROM keycloak_settings WHERE id = 'default'
	`
	var (
		out                                Settings
		clientSecretEnc, frontendSecretEnc string
	)
	err := s.db.QueryRow(ctx, q).Scan(
		&out.IssuerURL, &out.Realm, &out.ClientID, &clientSecretEnc, &out.Audience,
		&out.FrontendClientID, &frontendSecretEnc, &out.UpdatedAt, &out.UpdatedBy,
	)
	switch {
	case err == nil:
		// segue abaixo
	case errors.Is(err, pgx.ErrNoRows):
		return Settings{Configured: false}, nil
	default:
		return Settings{}, fmt.Errorf("keycloakconfig: get: %w", err)
	}

	if out.ClientSecret, err = s.cipher.Decrypt(clientSecretEnc); err != nil {
		return Settings{}, fmt.Errorf("keycloakconfig: decrypt client_secret (CONFIG_ENCRYPTION_KEY mudou desde que foi salvo?): %w", err)
	}
	if out.FrontendClientSecret, err = s.cipher.Decrypt(frontendSecretEnc); err != nil {
		return Settings{}, fmt.Errorf("keycloakconfig: decrypt frontend_client_secret (CONFIG_ENCRYPTION_KEY mudou desde que foi salvo?): %w", err)
	}

	out.Configured = out.IssuerURL != ""
	return out, nil
}

func (s *PostgresStore) Set(ctx context.Context, incoming Settings, updatedBy string) (Settings, error) {
	clientSecretEnc := s.cipher.Encrypt(incoming.ClientSecret)
	frontendSecretEnc := s.cipher.Encrypt(incoming.FrontendClientSecret)

	// client_secret_encrypted = CASE WHEN EXCLUDED... = '' THEN <valor já
	// salvo> ELSE EXCLUDED... END: um secret vazio recebido aqui significa
	// "admin deixou o campo em branco = manter o que já estava salvo",
	// nunca "apagar o segredo" — apagar de propósito não é um caso de uso
	// desta tela (a integração ficaria quebrada); ver o comentário de Set
	// na interface Store.
	const q = `
		INSERT INTO keycloak_settings (
			id, issuer_url, realm, client_id, client_secret_encrypted, audience,
			frontend_client_id, frontend_client_secret_encrypted, updated_at, updated_by
		) VALUES ('default', $1, $2, $3, $4, $5, $6, $7, now(), $8)
		ON CONFLICT (id) DO UPDATE SET
			issuer_url = EXCLUDED.issuer_url,
			realm = EXCLUDED.realm,
			client_id = EXCLUDED.client_id,
			client_secret_encrypted = CASE WHEN EXCLUDED.client_secret_encrypted = ''
				THEN keycloak_settings.client_secret_encrypted ELSE EXCLUDED.client_secret_encrypted END,
			audience = EXCLUDED.audience,
			frontend_client_id = EXCLUDED.frontend_client_id,
			frontend_client_secret_encrypted = CASE WHEN EXCLUDED.frontend_client_secret_encrypted = ''
				THEN keycloak_settings.frontend_client_secret_encrypted ELSE EXCLUDED.frontend_client_secret_encrypted END,
			updated_at = now(),
			updated_by = EXCLUDED.updated_by
		RETURNING issuer_url, realm, client_id, client_secret_encrypted, audience,
		          frontend_client_id, frontend_client_secret_encrypted, updated_at, updated_by
	`
	var (
		out                                Settings
		outClientSecretEnc, outFrontendEnc string
	)
	err := s.db.QueryRow(ctx, q,
		incoming.IssuerURL, incoming.Realm, incoming.ClientID, clientSecretEnc, incoming.Audience,
		incoming.FrontendClientID, frontendSecretEnc, updatedBy,
	).Scan(
		&out.IssuerURL, &out.Realm, &out.ClientID, &outClientSecretEnc, &out.Audience,
		&out.FrontendClientID, &outFrontendEnc, &out.UpdatedAt, &out.UpdatedBy,
	)
	if err != nil {
		return Settings{}, fmt.Errorf("keycloakconfig: set: %w", err)
	}

	if out.ClientSecret, err = s.cipher.Decrypt(outClientSecretEnc); err != nil {
		return Settings{}, fmt.Errorf("keycloakconfig: decrypt client_secret after set: %w", err)
	}
	if out.FrontendClientSecret, err = s.cipher.Decrypt(outFrontendEnc); err != nil {
		return Settings{}, fmt.Errorf("keycloakconfig: decrypt frontend_client_secret after set: %w", err)
	}
	out.Configured = out.IssuerURL != ""
	return out, nil
}
