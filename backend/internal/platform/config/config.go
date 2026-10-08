// Package config carrega e valida a configuração do Projeto Nexus a partir
// de variáveis de ambiente. Configuração obrigatória ausente faz Load
// retornar um erro imediatamente (fail fast) em vez de deixar a aplicação
// subir num estado parcialmente configurado — é preferível o processo nem
// iniciar a iniciar e falhar de forma imprevisível no primeiro request que
// tocar a configuração faltante.
package config

import (
	"fmt"
	"math"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// AppConfig guarda as configurações gerais de identidade da aplicação.
type AppConfig struct {
	Env       string // development | staging | production
	Name      string
	LogLevel  string
	LogFormat string // json | text
}

// HTTPConfig guarda as configurações de bind e os timeouts do servidor
// HTTP. Os defaults seguem a skill (M01/A05 — prevenção de DoS/Slowloris):
// ReadHeaderTimeout 2s, ReadTimeout 5s, WriteTimeout 10s, IdleTimeout 120s.
// Uploads nunca atravessam a API (vão direto ao MinIO por URL
// pré-assinada), então 5s de leitura de corpo bastam para qualquer JSON.
type HTTPConfig struct {
	Host              string
	Port              int
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

func (c HTTPConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// DatabaseConfig guarda as configurações de conexão e pool do PostgreSQL.
type DatabaseConfig struct {
	Host            string
	Port            int
	Name            string
	User            string
	Password        string
	SSLMode         string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
	ConnectTimeout  time.Duration
}

// DSN retorna uma connection string no estilo libpq, pronta para o pgxpool.
func (c DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s connect_timeout=%d",
		c.Host, c.Port, c.Name, c.User, c.Password, c.SSLMode, int(c.ConnectTimeout.Seconds()),
	)
}

// RabbitMQConfig guarda as configurações de conexão e de retry/prefetch do
// RabbitMQ.
type RabbitMQConfig struct {
	URL           string
	MaxRetries    int
	PrefetchCount int
}

// KeycloakConfig guarda as configurações OIDC do Keycloak gerenciado
// externamente — este projeto nunca cria/administra o Keycloak em si,
// apenas consome um realm/client já existentes (§29).
type KeycloakConfig struct {
	IssuerURL    string
	Realm        string
	ClientID     string
	ClientSecret string
	Audience     string
}

// SecurityConfig guarda segredos de infraestrutura de segurança da
// própria plataforma — não credenciais de negócio.
type SecurityConfig struct {
	// ConfigEncryptionKey cifra/decifra (AES-256-GCM, ver
	// internal/platform/secretcrypto) segredos que passam a ser
	// persistidos no Postgres em vez de só existir como variável de
	// ambiente — hoje, o Client Secret do Keycloak configurado em tempo
	// de execução via /api/v1/admin/keycloak (ver
	// internal/platform/keycloakconfig). Esperada em base64 padrão,
	// decodificando para exatamente 32 bytes (ex.: `openssl rand -base64
	// 32`). Suporta o padrão "<KEY>_FILE" via loader.secret, como
	// qualquer outro segredo desta plataforma.
	ConfigEncryptionKey string

	// MetricsToken protege o endpoint /metrics do Prometheus (gap G-02 da
	// auditoria de conformidade). Vazio = endpoint aberto (comportamento
	// histórico, aceitável só em dev ou com a porta em rede interna
	// isolada); definido = exige "Authorization: Bearer <token>" no
	// scrape. Suporta o padrão "<KEY>_FILE" via loader.secret.
	MetricsToken string

	// TrustedProxies é a lista de CIDRs de proxies reversos confiáveis
	// (gap G-04). Só quando o RemoteAddr da conexão TCP cai numa dessas
	// faixas é que o cabeçalho X-Forwarded-For é lido para descobrir o IP
	// real do cliente — sem isso, qualquer cliente forja o XFF para
	// escapar do rate limiter e para poluir o ip_address gravado na
	// prova de consentimento LGPD. Vazio = nunca confiar em XFF (só o
	// RemoteAddr direto vale).
	TrustedProxies []string
}

// LocalAuthConfig guarda as configurações do login local por
// usuário/senha (§ Sistema de Login Local) — um caminho de autenticação
// PARALELO ao Keycloak (útil para dev/teste e como conta de emergência),
// nunca um substituto: nada aqui desliga ou substitui a verificação via
// Keycloak, que continua obrigatória e configurada como sempre foi. Os
// tokens locais são assinados com RSA (RS256) usando um par de chaves
// PRÓPRIO deste subsistema — nunca a mesma chave/segredo usado por
// qualquer coisa relacionada ao Keycloak/SSO, para que os dois caminhos
// de autenticação permaneçam criptograficamente independentes (ver
// internal/platform/auth/local.go e docs/adr/003-local-auth-rsa-hardening.md).
type LocalAuthConfig struct {
	Enabled bool
	// PrivateKeyPEM é o conteúdo PEM (PKCS1 ou PKCS8) da chave privada RSA
	// usada para assinar e verificar tokens locais — a chave pública é
	// derivada dela em tempo de execução (auth.NewLocalSigner), nunca
	// configurada separadamente. Suporta o padrão "<KEY>_FILE" via
	// loader.secret (LOCAL_AUTH_PRIVATE_KEY_FILE), o jeito recomendado de
	// fornecer este valor: PEM é multi-linha e não cabe bem numa variável
	// de ambiente comum.
	PrivateKeyPEM string
	TokenTTL      time.Duration
}

// RedisConfig aponta para o Redis usado como backplane do WebSocket
// (fan-out entre réplicas da API), rate limiting adaptativo distribuído,
// lockout progressivo e invalidação do cache de permissões (A07).
type RedisConfig struct {
	// URL no formato redis://[:senha@]host:porta/db ou rediss:// (TLS).
	URL string
}

// EgressConfig parametriza o plugin Egress (webhooks de saída).
type EgressConfig struct {
	Timeout      time.Duration
	MaxAttempts  int
	BatchSize    int
	PollInterval time.Duration
	// AllowPrivateNetworks desliga o bloqueio anti-SSRF de faixas privadas
	// (RFC 1918), loopback e link-local. SÓ para desenvolvimento local
	// (ex.: um n8n no mesmo docker-compose) — Load recusa true em produção.
	AllowPrivateNetworks bool
}

// AtlasConfig parametriza a IA do assistente do Atlas (fluxos e TTDD) enquanto nada
// for configurado em Configurações > Inteligência artificial (ADR 020).
type AtlasConfig struct {
	// AIEndpoint é a URL base de um provedor compatível com a API de chat
	// da OpenAI (OpenAI, vLLM, LiteLLM, Ollama). Vazio = assistente sem IA:
	// responde com a síntese canônica dos procedimentos homologados.
	AIEndpoint string
	AIAPIKey   string
	AIModel    string
	AITimeout  time.Duration
	// ChatRateLimit limita as consultas ao assistente por identidade.
	ChatRateLimit RateLimitConfig
}

// SignumConfig parametriza a cerimônia de assinatura eletrônica.
type SignumConfig struct {
	ChallengeTTL time.Duration
}

// UploadConfig limita os uploads diretos ao MinIO (URL pré-assinada).
type UploadConfig struct {
	MaxFileBytes int64
	URLExpiry    time.Duration
}

// RateLimitConfig parametriza um limitador de janela fixa (ver
// internal/platform/ratelimit.PostgresLimiter): até MaxRequests
// requisições por chave a cada WindowSeconds segundos.
type RateLimitConfig struct {
	WindowSeconds int
	MaxRequests   int
}

// AuditWORMConfig parametriza a cópia WORM (Write Once Read Many) da
// trilha de auditoria (F2.6). Bucket DEDICADO (não o de blobs da app):
// object-lock só pode ser habilitado na criação do bucket.
type AuditWORMConfig struct {
	Bucket        string // AUDIT_WORM_BUCKET (default "nexus-audit-worm")
	RetentionDays int    // AUDIT_WORM_RETENTION_DAYS (default 1825 = 5 anos)
}

// WorkerConfig guarda as configurações do listener HTTP mínimo próprio do
// cmd/worker, usado só para /health e /metrics (healthcheck do Docker +
// scrape do Prometheus) — nunca para tráfego de negócio.
type WorkerConfig struct {
	MetricsHost string
	MetricsPort int
}

func (c WorkerConfig) MetricsAddr() string {
	return fmt.Sprintf("%s:%d", c.MetricsHost, c.MetricsPort)
}

// MinIOConfig guarda as credenciais para storage de objetos (S3/MinIO).
type MinIOConfig struct {
	Endpoint  string // host:porta alcançável pelo backend (rede interna) — usado em Get/Put/Delete
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
	// PublicEndpoint / PublicUseSSL: host alcançável pelo NAVEGADOR — as
	// URLs pré-assinadas (upload/download direto do cliente) precisam ser
	// assinadas para ESTE host, senão o navegador tenta resolver o nome
	// interno do Docker ("minio:9000") e o upload falha. Default = Endpoint.
	PublicEndpoint string
	PublicUseSSL   bool
}

// Config é a configuração da aplicação já totalmente validada.
type Config struct {
	App       AppConfig
	HTTP      HTTPConfig
	Database  DatabaseConfig
	RabbitMQ  RabbitMQConfig
	Redis     RedisConfig
	Keycloak  KeycloakConfig
	Security  SecurityConfig
	LocalAuth LocalAuthConfig
	Worker    WorkerConfig
	Egress    EgressConfig
	Atlas     AtlasConfig
	Signum    SignumConfig
	Upload    UploadConfig

	AuditWORM AuditWORMConfig

	// APIRateLimit é o teto por identidade autenticada (fallback: IP)
	// aplicado a todo o grupo /api/v1 (gap G-01).
	APIRateLimit RateLimitConfig
	// LoginRateLimit limita tentativas de login local por IP; o lockout
	// progressivo por conta/IP fica em internal/platform/ratelimit.
	LoginRateLimit RateLimitConfig
	// LoginLockoutIPThreshold: falhas de login por IP antes do bloqueio
	// progressivo do IP (a conta bloqueia com 5). Alto de propósito: atrás
	// de NAT, um IP é um prédio inteiro.
	LoginLockoutIPThreshold int
	// ContactRateLimit é o limite dedicado do formulário público de
	// contato, por IP.
	ContactRateLimit RateLimitConfig
	// PublicRateLimit limita as demais rotas públicas (catálogo,
	// transparência, consentimento anônimo), por IP.
	PublicRateLimit RateLimitConfig

	MinIO MinIOConfig

	FrontendURL         string
	APIPublicURL        string
	WebSocketPublicURL  string
	OTELExporterOTLPURL string
	MaxPageSize         int
}

// loader acumula os erros de leitura para que Load reporte de uma vez toda
// variável faltante, em vez de forçar quem está operando a passar por um
// ciclo de "corrige uma, reinicia, corrige a próxima".
type loader struct {
	errs []string
}

func (l *loader) str(key string, required bool, def string) string {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		if required {
			l.errs = append(l.errs, key)
		}
		return def
	}
	return v
}

// secret funciona como str, mas primeiro verifica se existe uma variável
// "<KEY>_FILE" apontando para um arquivo — se sim, o CONTEÚDO do arquivo
// (sem espaços/quebras de linha nas pontas) é usado como valor, e a
// variável "<KEY>" direta é ignorada.
//
// Isto é o padrão universal para rotação/gestão de segredos sem precisar
// de código específico para cada backend: Docker Swarm secrets monta cada
// segredo como um arquivo em /run/secrets/<nome>; Kubernetes Secrets
// montados como volume funcionam do mesmo jeito; o Vault Agent Sidecar
// Injector escreve o segredo lido do Vault num arquivo local; AWS
// Secrets Manager com o CSI driver também. Nenhum desses precisa que a
// aplicação fale a API específica do provedor — só que ela saiba ler
// "<KEY>_FILE" em vez de "<KEY>" quando o arquivo existir. Usado para
// todo valor que é de fato um segredo (senha, client secret, chave de
// API) — nunca para configuração não sensível (host, nome de banco,
// etc.), que continua vindo direto de env var via str().
func (l *loader) secret(key string, required bool, def string) string {
	filePath, hasFileVar := os.LookupEnv(key + "_FILE")
	if hasFileVar && filePath != "" {
		// #nosec G304 -- filePath vem de "<KEY>_FILE", uma variável de
		// ambiente definida pelo operador do deploy (padrão Docker/K8s/
		// Vault secrets), nunca de entrada de requisição.
		content, err := os.ReadFile(filePath)
		if err != nil {
			l.errs = append(l.errs, fmt.Sprintf("%s_FILE (failed to read %q: %v)", key, filePath, err))
			return def
		}
		return strings.TrimSpace(string(content))
	}
	return l.str(key, required, def)
}

func (l *loader) intVal(key string, required bool, def int) int {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		if required {
			l.errs = append(l.errs, key)
		}
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		l.errs = append(l.errs, fmt.Sprintf("%s (invalid integer %q)", key, v))
		return def
	}
	return n
}

// int32Val lê um inteiro que precisa caber em int32 (ex.: tamanhos de
// pool do pgxpool). Um valor fora da faixa é erro de configuração — o
// processo não sobe — em vez de estourar silenciosamente na conversão.
func (l *loader) int32Val(key string, def int32) int32 {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		l.errs = append(l.errs, fmt.Sprintf("%s (invalid integer %q)", key, v))
		return def
	}
	if n < 0 || n > math.MaxInt32 {
		l.errs = append(l.errs, fmt.Sprintf("%s (%d fora da faixa [0, %d])", key, n, math.MaxInt32))
		return def
	}
	// #nosec G109 -- n é validado contra [0, math.MaxInt32] na linha acima.
	return int32(n)
}

func (l *loader) durationVal(key string, def time.Duration) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		l.errs = append(l.errs, fmt.Sprintf("%s (invalid duration %q)", key, v))
		return def
	}
	return d
}

func (l *loader) boolVal(key string, def bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		l.errs = append(l.errs, fmt.Sprintf("%s (invalid boolean %q)", key, v))
		return def
	}
	return b
}

// Load lê a configuração a partir do ambiente do processo. Retorna um erro
// nomeando toda variável obrigatória ausente/inválida, caso a validação
// falhe.
func Load() (*Config, error) {
	l := &loader{}

	cfg := &Config{
		App: AppConfig{
			Env:       l.str("APP_ENV", true, ""),
			Name:      l.str("APP_NAME", false, "projeto-nexus"),
			LogLevel:  l.str("APP_LOG_LEVEL", false, "info"),
			LogFormat: l.str("LOG_FORMAT", false, "json"),
		},
		HTTP: HTTPConfig{
			Host:              l.str("HTTP_HOST", false, "0.0.0.0"),
			Port:              l.intVal("HTTP_PORT", false, 8000),
			ReadHeaderTimeout: l.durationVal("HTTP_READ_HEADER_TIMEOUT", 2*time.Second),
			ReadTimeout:       l.durationVal("HTTP_READ_TIMEOUT", 5*time.Second),
			WriteTimeout:      l.durationVal("HTTP_WRITE_TIMEOUT", 10*time.Second),
			IdleTimeout:       l.durationVal("HTTP_IDLE_TIMEOUT", 120*time.Second),
		},
		Database: DatabaseConfig{
			Host:            l.str("DB_HOST", true, ""),
			Port:            l.intVal("DB_PORT", true, 0),
			Name:            l.str("DB_NAME", true, ""),
			User:            l.str("DB_USER", true, ""),
			Password:        l.secret("DB_PASSWORD", true, ""),
			SSLMode:         l.str("DB_SSLMODE", false, "disable"),
			MaxConns:        l.int32Val("DB_MAX_CONNS", 20),
			MinConns:        l.int32Val("DB_MIN_CONNS", 2),
			MaxConnLifetime: l.durationVal("DB_MAX_CONN_LIFETIME", time.Hour),
			MaxConnIdleTime: l.durationVal("DB_MAX_CONN_IDLE_TIME", 15*time.Minute),
			ConnectTimeout:  l.durationVal("DB_CONNECT_TIMEOUT", 5*time.Second),
		},
		RabbitMQ: RabbitMQConfig{
			URL:           l.secret("RABBITMQ_URL", true, ""),
			MaxRetries:    l.intVal("RABBITMQ_MAX_RETRIES", false, 3),
			PrefetchCount: l.intVal("RABBITMQ_PREFETCH_COUNT", false, 10),
		},
		Redis: RedisConfig{
			URL: l.secret("REDIS_URL", true, ""),
		},
		Keycloak: KeycloakConfig{
			IssuerURL:    l.str("KEYCLOAK_ISSUER_URL", false, ""),
			Realm:        l.str("KEYCLOAK_REALM", false, ""),
			ClientID:     l.str("KEYCLOAK_CLIENT_ID", false, ""),
			ClientSecret: l.secret("KEYCLOAK_CLIENT_SECRET", false, ""),
			Audience:     l.str("KEYCLOAK_AUDIENCE", false, ""),
		},
		Security: SecurityConfig{
			ConfigEncryptionKey: l.secret("CONFIG_ENCRYPTION_KEY", false, insecureConfigEncryptionKey),
			MetricsToken:        l.secret("METRICS_SCRAPE_TOKEN", false, ""),
			TrustedProxies:      splitCSV(l.str("TRUSTED_PROXIES", false, "")),
		},
		LocalAuth: LocalAuthConfig{
			Enabled:       l.boolVal("LOCAL_AUTH_ENABLED", false),
			PrivateKeyPEM: l.secret("LOCAL_AUTH_PRIVATE_KEY", false, ""),
			TokenTTL:      l.durationVal("LOCAL_AUTH_TOKEN_TTL", time.Hour),
		},
		APIRateLimit: RateLimitConfig{
			WindowSeconds: l.intVal("API_RATE_LIMIT_WINDOW_SECONDS", false, 60),
			MaxRequests:   l.intVal("API_RATE_LIMIT_MAX", false, 600),
		},
		LoginRateLimit: RateLimitConfig{
			WindowSeconds: l.intVal("LOGIN_RATE_LIMIT_WINDOW_SECONDS", false, 60),
			MaxRequests:   l.intVal("LOGIN_RATE_LIMIT_MAX", false, 10),
		},
		LoginLockoutIPThreshold: l.intVal("LOGIN_LOCKOUT_IP_THRESHOLD", false, 30),
		ContactRateLimit: RateLimitConfig{
			WindowSeconds: l.intVal("CONTACT_RATE_LIMIT_WINDOW_SECONDS", false, 3600),
			MaxRequests:   l.intVal("CONTACT_RATE_LIMIT_MAX", false, 5),
		},
		PublicRateLimit: RateLimitConfig{
			WindowSeconds: l.intVal("PUBLIC_RATE_LIMIT_WINDOW_SECONDS", false, 60),
			MaxRequests:   l.intVal("PUBLIC_RATE_LIMIT_MAX", false, 120),
		},
		Egress: EgressConfig{
			Timeout:              l.durationVal("EGRESS_TIMEOUT", 10*time.Second),
			MaxAttempts:          l.intVal("EGRESS_MAX_ATTEMPTS", false, 8),
			BatchSize:            l.intVal("EGRESS_BATCH_SIZE", false, 20),
			PollInterval:         l.durationVal("EGRESS_POLL_INTERVAL", 5*time.Second),
			AllowPrivateNetworks: l.boolVal("EGRESS_ALLOW_PRIVATE_NETWORKS", false),
		},
		Atlas: AtlasConfig{
			AIEndpoint: l.str("ATLAS_AI_ENDPOINT", false, ""),
			AIAPIKey:   l.secret("ATLAS_AI_API_KEY", false, ""),
			AIModel:    l.str("ATLAS_AI_MODEL", false, "llama3.2"),
			AITimeout:  l.durationVal("ATLAS_AI_TIMEOUT", 20*time.Second),
			ChatRateLimit: RateLimitConfig{
				WindowSeconds: l.intVal("ATLAS_CHAT_RATE_LIMIT_WINDOW_SECONDS", false, 60),
				MaxRequests:   l.intVal("ATLAS_CHAT_RATE_LIMIT_MAX", false, 10),
			},
		},
		Signum: SignumConfig{
			ChallengeTTL: l.durationVal("SIGNUM_CHALLENGE_TTL", 5*time.Minute),
		},
		Upload: UploadConfig{
			MaxFileBytes: int64(l.intVal("UPLOAD_MAX_FILE_MB", false, 100)) * 1024 * 1024,
			URLExpiry:    l.durationVal("UPLOAD_URL_EXPIRY", 15*time.Minute),
		},
		AuditWORM: AuditWORMConfig{
			Bucket:        l.str("AUDIT_WORM_BUCKET", false, "nexus-audit-worm"),
			RetentionDays: l.intVal("AUDIT_WORM_RETENTION_DAYS", false, 1825),
		},
		Worker: WorkerConfig{
			MetricsHost: l.str("WORKER_METRICS_HOST", false, "0.0.0.0"),
			MetricsPort: l.intVal("WORKER_METRICS_PORT", false, 9100),
		},
		MinIO:               minioConfig(l),
		FrontendURL:         l.str("FRONTEND_URL", false, "http://localhost:3000"),
		APIPublicURL:        l.str("API_PUBLIC_URL", false, "http://localhost:8000"),
		WebSocketPublicURL:  l.str("WEBSOCKET_PUBLIC_URL", false, "ws://localhost:8000/ws"),
		OTELExporterOTLPURL: l.str("OTEL_EXPORTER_OTLP_ENDPOINT", false, ""),
		MaxPageSize:         l.intVal("MAX_PAGE_SIZE", false, 100),
	}

	if len(l.errs) > 0 {
		return nil, fmt.Errorf("config: missing or invalid required environment variables: %s", strings.Join(l.errs, ", "))
	}

	if raw := cfg.Atlas.AIEndpoint; raw != "" {
		if u, err := url.Parse(raw); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
			return nil, fmt.Errorf("config: ATLAS_AI_ENDPOINT deve ser uma URL http(s) sem credenciais embutidas (use ATLAS_AI_API_KEY), veio %q", raw)
		}
	}

	if cfg.App.Env != "development" && cfg.App.Env != "staging" && cfg.App.Env != "production" && cfg.App.Env != "test" {
		return nil, fmt.Errorf("config: APP_ENV must be one of development|staging|production|test, got %q", cfg.App.Env)
	}

	// LOCAL_AUTH_PRIVATE_KEY (ou _FILE) só é obrigatória quando o login
	// local está ligado — não faz sentido validá-la sempre (ela não é
	// usada em nenhum outro caminho), mas um deploy com
	// LOCAL_AUTH_ENABLED=true e sem chave configurada precisa falhar no
	// startup, não emitir tokens sem assinatura válida. A validação de que
	// o PEM de fato parseia como uma chave RSA válida (e do tamanho
	// mínimo) acontece em auth.NewLocalSigner, não aqui — este pacote só
	// confere presença, não a validade criptográfica do conteúdo.
	if cfg.LocalAuth.Enabled && cfg.LocalAuth.PrivateKeyPEM == "" {
		return nil, fmt.Errorf("config: LOCAL_AUTH_PRIVATE_KEY is required when LOCAL_AUTH_ENABLED=true")
	}

	// Segredos com default de conveniência para dev/test que NUNCA podem ir
	// para produção com esse valor. Em APP_ENV=production o startup falha se
	// qualquer um ainda estiver no default inseguro — defesa contra deploy
	// que esqueceu de sobrescrever a env (ex.: subir o docker-compose.yml
	// como está, cujo default `:-` não força nada).
	//
	// A mesma checagem cobre também os valores literais de .env.example
	// (achado de auditoria de segurança): DB_PASSWORD=nexus_pass e
	// RABBITMQ_DEFAULT_PASS=rabbit_pass não são "defaults" no sentido de
	// código (são required=true, sem fallback em loader.secret) — mas o
	// arquivo em si é público (committed no git), então um operador que
	// fizer `cp .env.example .env` num ambiente real e esquecer de trocar
	// esses dois valores estaria, na prática, publicando a senha do banco e
	// do broker. exampleSecretValues centraliza essa lista para não
	// duplicá-la a cada novo segredo textual adicionado a .env.example.
	if cfg.App.Env == "production" {
		var weak []string
		if cfg.MinIO.AccessKey == insecureMinioAccessKey {
			weak = append(weak, "MINIO_ACCESS_KEY")
		}
		if cfg.MinIO.SecretKey == insecureMinioSecretKey {
			weak = append(weak, "MINIO_SECRET_KEY")
		}
		if cfg.Database.Password == exampleDBPassword {
			weak = append(weak, "DB_PASSWORD")
		}
		if strings.Contains(cfg.RabbitMQ.URL, exampleRabbitMQPassword) {
			weak = append(weak, "RABBITMQ_URL (RABBITMQ_DEFAULT_PASS)")
		}
		if strings.Contains(cfg.Redis.URL, exampleRedisPassword) {
			weak = append(weak, "REDIS_URL (REDIS_PASSWORD)")
		}
		if cfg.Security.ConfigEncryptionKey == insecureConfigEncryptionKey {
			weak = append(weak, "CONFIG_ENCRYPTION_KEY")
		}
		if cfg.Egress.AllowPrivateNetworks {
			weak = append(weak, "EGRESS_ALLOW_PRIVATE_NETWORKS (proteção anti-SSRF desligada)")
		}
		if len(weak) > 0 {
			return nil, fmt.Errorf(
				"config: recusando iniciar em produção com segredo(s) no valor default inseguro ou de .env.example: %s — defina uma env var forte para cada um",
				strings.Join(weak, ", "))
		}
	}

	return cfg, nil
}

// Defaults inseguros: só existem para o fluxo dev/test funcionar sem
// configuração. Ver a validação de produção em Load().
const (
	insecureMinioAccessKey = "admin"
	insecureMinioSecretKey = "password123"

	// exampleDBPassword/exampleRabbitMQPassword são os valores literais
	// commitados em .env.example — nunca defaults de código (DB_PASSWORD e
	// RABBITMQ_URL são required=true, sem fallback), mas o arquivo em si é
	// público, então valem a mesma checagem de produção que os defaults do
	// MinIO acima. Ver o comentário em Load().
	exampleDBPassword       = "dev-change-this-db-password"
	exampleRabbitMQPassword = "dev-change-this-rabbitmq-password"
	exampleRedisPassword    = "dev-change-this-redis-password"

	// insecureConfigEncryptionKey é uma chave AES-256 FIXA e PÚBLICA (só
	// para o processo conseguir subir em dev/test sem exigir mais uma
	// variável de ambiente) — 32 bytes de "0" em base64. Nunca pode ir
	// para produção: ver a checagem em Load(). Gere uma chave real com
	// `openssl rand -base64 32`.
	insecureConfigEncryptionKey = "MDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDA="
)

// splitCSV divide uma lista separada por vírgulas, descartando espaços em
// branco e itens vazios. Usado para TRUSTED_PROXIES.
func splitCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// minioConfig monta MinIOConfig, derivando o endpoint público (para URLs
// pré-assinadas — alcançável pelo navegador) de MINIO_PUBLIC_URL. Sem essa
// variável, usa o mesmo host interno (comportamento antigo).
func minioConfig(l *loader) MinIOConfig {
	c := MinIOConfig{
		Endpoint:  l.str("MINIO_ENDPOINT", false, "minio:9000"),
		AccessKey: l.str("MINIO_ACCESS_KEY", false, insecureMinioAccessKey),
		SecretKey: l.secret("MINIO_SECRET_KEY", false, insecureMinioSecretKey),
		Bucket:    l.str("MINIO_BUCKET", false, "nexus"),
		UseSSL:    l.boolVal("MINIO_USE_SSL", false),
	}
	c.PublicEndpoint = c.Endpoint
	c.PublicUseSSL = c.UseSSL
	if raw := strings.TrimSpace(os.Getenv("MINIO_PUBLIC_URL")); raw != "" {
		if u, err := url.Parse(raw); err == nil && u.Host != "" {
			c.PublicEndpoint = u.Host
			c.PublicUseSSL = u.Scheme == "https"
		} else {
			l.errs = append(l.errs, fmt.Sprintf("MINIO_PUBLIC_URL (URL inválida: %q)", raw))
		}
	}
	return c
}

// LoadDatabase lê só as variáveis DB_* — usado por ferramentas standalone
// (ex.: cmd/seedadmin) que precisam de uma conexão Postgres mas não do
// resto da configuração da plataforma (Keycloak, RabbitMQ, ...), que
// Load() exigiria sem necessidade. Mesmo loader/mesmas variáveis/mesmos
// defaults que Load() usa pra popular Config.Database — as duas nunca
// podem divergir em como DB_* é lido, então isto não duplica a lógica de
// parsing, só o subconjunto de chamadas.
func LoadDatabase() (DatabaseConfig, error) {
	l := &loader{}
	db := DatabaseConfig{
		Host:            l.str("DB_HOST", true, ""),
		Port:            l.intVal("DB_PORT", true, 0),
		Name:            l.str("DB_NAME", true, ""),
		User:            l.str("DB_USER", true, ""),
		Password:        l.secret("DB_PASSWORD", true, ""),
		SSLMode:         l.str("DB_SSLMODE", false, "disable"),
		MaxConns:        l.int32Val("DB_MAX_CONNS", 20),
		MinConns:        l.int32Val("DB_MIN_CONNS", 2),
		MaxConnLifetime: l.durationVal("DB_MAX_CONN_LIFETIME", time.Hour),
		MaxConnIdleTime: l.durationVal("DB_MAX_CONN_IDLE_TIME", 15*time.Minute),
		ConnectTimeout:  l.durationVal("DB_CONNECT_TIMEOUT", 5*time.Second),
	}
	if len(l.errs) > 0 {
		return DatabaseConfig{}, fmt.Errorf("config: missing or invalid required environment variables: %s", strings.Join(l.errs, ", "))
	}
	return db, nil
}
