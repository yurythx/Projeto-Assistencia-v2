package app

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/jackc/pgx/v5"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"

	"github.com/yurythx/projeto-nexus/internal/domain/events"
	"github.com/yurythx/projeto-nexus/internal/platform/config"
	"github.com/yurythx/projeto-nexus/internal/platform/keycloakconfig"
	"github.com/yurythx/projeto-nexus/internal/platform/messaging"
	"github.com/yurythx/projeto-nexus/internal/platform/redisx"
	"github.com/yurythx/projeto-nexus/internal/platform/secretcrypto"
	"github.com/yurythx/projeto-nexus/internal/platform/ws"
)

const testConfigKey = "MDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDA="

// infraConfig monta uma configuração completa contra a infraestrutura de
// teste real (Postgres, RabbitMQ, MinIO; Redis em memória) ou pula.
func infraConfig(t *testing.T) *config.Config {
	t.Helper()
	dsn, rmq, minio := os.Getenv("TEST_DATABASE_URL"), os.Getenv("TEST_RABBITMQ_URL"), os.Getenv("TEST_MINIO_ENDPOINT")
	if dsn == "" || rmq == "" || minio == "" {
		t.Skip("TEST_DATABASE_URL/TEST_RABBITMQ_URL/TEST_MINIO_ENDPOINT não definidos")
	}
	pg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	pemKey := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
	limit := config.RateLimitConfig{WindowSeconds: 60, MaxRequests: 1000}
	return &config.Config{
		App: config.AppConfig{Env: "test", Name: "nexus-test", LogLevel: "error", LogFormat: "json"},
		Database: config.DatabaseConfig{Host: pg.Host, Port: int(pg.Port), Name: pg.Database, User: pg.User, Password: pg.Password,
			SSLMode: "disable", MaxConns: 5, MinConns: 0, MaxConnLifetime: time.Hour, MaxConnIdleTime: time.Minute, ConnectTimeout: 3 * time.Second},
		Redis:     config.RedisConfig{URL: "redis://" + miniredis.RunT(t).Addr()},
		RabbitMQ:  config.RabbitMQConfig{URL: rmq, MaxRetries: 3, PrefetchCount: 5},
		MinIO:     config.MinIOConfig{Endpoint: minio, AccessKey: os.Getenv("TEST_MINIO_ACCESS_KEY"), SecretKey: os.Getenv("TEST_MINIO_SECRET_KEY"), Bucket: "nexus-app-test"},
		LocalAuth: config.LocalAuthConfig{Enabled: true, PrivateKeyPEM: pemKey, TokenTTL: time.Hour},
		Upload:    config.UploadConfig{MaxFileBytes: 10 << 20, URLExpiry: time.Minute},
		Signum:    config.SignumConfig{ChallengeTTL: time.Minute},
		Egress:    config.EgressConfig{Timeout: time.Second, MaxAttempts: 3, BatchSize: 10, PollInterval: time.Second},
		Security:  config.SecurityConfig{ConfigEncryptionKey: testConfigKey},
		AuditWORM: config.AuditWORMConfig{Bucket: "nexus-app-worm", RetentionDays: 1},
		Worker:    config.WorkerConfig{MetricsHost: "127.0.0.1", MetricsPort: 0},

		APIRateLimit: limit, LoginRateLimit: limit, PublicRateLimit: limit, ContactRateLimit: limit,
		FrontendURL: "http://localhost:3000", MaxPageSize: 100,
	}
}

// keycloakRow grava (e remove ao fim) a configuração persistida do Keycloak.
func keycloakRow(t *testing.T, cfg *config.Config, cipherKey string, s keycloakconfig.Settings) {
	t.Helper()
	d := testDeps(t)
	cipher, err := secretcrypto.NewFromBase64Key(cipherKey)
	if err != nil {
		t.Fatal(err)
	}
	cleanup := func() { _, _ = d.DB.Exec(context.Background(), `DELETE FROM keycloak_settings WHERE id = 'default'`) }
	cleanup()
	t.Cleanup(cleanup)
	if _, err := keycloakconfig.NewPostgresStore(d.DB, cipher).Set(context.Background(), s, "teste"); err != nil {
		t.Fatal(err)
	}
}

func TestBuildWiresEveryDependency(t *testing.T) {
	cfg := infraConfig(t)
	// Configuração persistida do Keycloak com discovery que falha: o boot
	// segue com o .env (só registra o erro).
	keycloakRow(t, cfg, testConfigKey, keycloakconfig.Settings{IssuerURL: "http://127.0.0.1:1/realms/nx", Realm: "nx", ClientID: "c", Audience: "c"})

	d, err := build(context.Background(), cfg, "api")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.Close)

	if d.Kernel == nil || d.Messaging == nil || d.Storage == nil || d.Transparency == nil || len(d.Kernel.Plugins()) == 0 {
		t.Fatalf("dependências incompletas: %+v", d)
	}
	router := NewRouter(d)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/ready = %d %s", rec.Code, rec.Body.String())
	}
	// o Signum recebe as credenciais salvas pela tela
	if issuer, _, _ := d.keycloakCredentials(context.Background()); issuer != "http://127.0.0.1:1/realms/nx" {
		t.Fatalf("issuer = %q", issuer)
	}

	d.Close()
	d.Close() // idempotente
}

func TestBuildWarnsButBootsWhenOptionalPiecesFail(t *testing.T) {
	cfg := infraConfig(t)
	cfg.MinIO.SecretKey = "errada" // EnsureBucket falha: só aviso
	// segredo gravado com outra chave: a leitura falha e o boot segue com o .env
	keycloakRow(t, cfg, base64.StdEncoding.EncodeToString([]byte("11111111111111111111111111111111")),
		keycloakconfig.Settings{IssuerURL: "https://sso/realms/nx", Realm: "nx", ClientID: "c", ClientSecret: "s", Audience: "c"})

	d, err := build(context.Background(), cfg, "worker")
	if err != nil {
		t.Fatal(err)
	}
	d.Close()
}

func TestBuildFailsFastOnEachRequiredDependency(t *testing.T) {
	cases := []struct {
		name, want string
		mutate     func(*testing.T, *config.Config)
	}{
		{"postgres", "connect database", func(_ *testing.T, c *config.Config) { c.Database.Port = 1 }},
		{"redis", "connect redis", func(_ *testing.T, c *config.Config) { c.Redis.URL = "http://nada" }},
		{"signer", "local auth signer", func(_ *testing.T, c *config.Config) { c.LocalAuth.PrivateKeyPEM = "não é pem" }},
		{"oidc", "OIDC verifier", func(_ *testing.T, c *config.Config) {
			c.Keycloak = config.KeycloakConfig{IssuerURL: "http://127.0.0.1:1/realms/nx", ClientID: "c", Audience: "c"}
		}},
		{"cipher", "config encryption cipher", func(_ *testing.T, c *config.Config) { c.Security.ConfigEncryptionKey = "curta" }},
		{"proxies", "TRUSTED_PROXIES", func(_ *testing.T, c *config.Config) { c.Security.TrustedProxies = []string{"não-é-cidr"} }},
		{"rabbitmq", "connect rabbitmq", func(_ *testing.T, c *config.Config) { c.RabbitMQ.URL = "http://127.0.0.1:5672/" }},
		{"minio", "connect minio", func(_ *testing.T, c *config.Config) { c.MinIO.Endpoint = "minio:9000/caminho" }},
		// banco alcançável, mas sem as tabelas (search_path vazio)
		{"kernel", "kernel start", func(_ *testing.T, c *config.Config) {
			c.Database.Name += " options='-c search_path=nx_schema_inexistente'"
		}},
		{"topology", "declare rabbitmq topology", conflictingQueue},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := infraConfig(t)
			tc.mutate(t, cfg)
			d, err := build(context.Background(), cfg, "api")
			if err == nil {
				d.Close()
				t.Fatal("build deveria falhar")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v (quer %q)", err, tc.want)
			}
		})
	}
}

// conflictingQueue redeclara a fila de notificações com outros argumentos:
// a declaração da topologia recebe PRECONDITION_FAILED. A fila volta ao
// normal ao fim (o próximo build a declara de novo).
func conflictingQueue(t *testing.T, c *config.Config) {
	t.Helper()
	conn, err := amqp.Dial(c.RabbitMQ.URL)
	if err != nil {
		t.Fatal(err)
	}
	ch, err := conn.Channel()
	if err != nil {
		t.Fatal(err)
	}
	name := messaging.QueueNotificationWebsocket.Name
	if _, err := ch.QueueDelete(name, false, false, false); err != nil {
		t.Fatal(err)
	}
	if _, err := ch.QueueDeclare(name, false, false, false, false, nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = ch.QueueDelete(name, false, false, false)
		_ = conn.Close()
		// devolve a fila ao formato certo para os próximos testes
		if d, err := build(context.Background(), infraConfig(t), "api"); err == nil {
			d.Close()
		}
	})
}

func TestDeclareTopologyNeedsAnOpenConnection(t *testing.T) {
	cfg := infraConfig(t)
	conn, err := messaging.Connect(context.Background(), cfg.RabbitMQ.URL, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
	if err := declareTopology(conn, messaging.PlatformQueues()); err == nil || !strings.Contains(err.Error(), "open channel") {
		t.Fatalf("err = %v", err)
	}
}

func TestWorkerRunsAllProcessorsUntilCancelled(t *testing.T) {
	cfg := infraConfig(t)
	d, err := build(context.Background(), cfg, "worker")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.Close)
	w := NewWorker(d)
	if len(w.processors) != 7 {
		t.Fatalf("processors = %d", len(w.processors))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := w.Run(ctx); err != nil {
		t.Fatal(err)
	}

	// consume liga uma fila de plugin a um handler
	cctx, ccancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer ccancel()
	_ = d.consume(cctx, messaging.QueueNotificationWebsocket.Name, func(context.Context, events.Event) error { return nil })

	// ... e o processo da API também sobe e desce limpo
	actx, acancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer acancel()
	RunAPIBackground(actx, d)
}

func TestWorkerMetricsServer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// porta livre: sobe e desce com o ctx
	port := freePort(t)
	w := &Worker{deps: &Dependencies{Logger: logger, Config: &config.Config{Worker: config.WorkerConfig{MetricsHost: "127.0.0.1", MetricsPort: port}}}}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.RunMetricsServer(ctx) }()
	url := "http://127.0.0.1:" + strconv.Itoa(port)
	var resp *http.Response
	var err error
	for i := 0; i < 50; i++ {
		if resp, err = http.Get(url + "/metrics"); err == nil { // #nosec G107 -- servidor local do teste
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !strings.Contains(string(body), "go_goroutines") {
		t.Fatal("/metrics sem métricas")
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	// porta ocupada: devolve o erro do listener
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	w.deps.Config.Worker.MetricsPort = l.Addr().(*net.TCPAddr).Port
	if err := w.RunMetricsServer(context.Background()); err == nil {
		t.Fatal("porta ocupada deveria falhar")
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func TestSupervisedRestartsWithBackoff(t *testing.T) {
	origMin, origMax := supervisedMinBackoff, supervisedMaxBackoff
	supervisedMinBackoff, supervisedMaxBackoff = time.Millisecond, 2*time.Millisecond
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	var calls atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	p := supervised("teste", logger, func(context.Context) error {
		switch calls.Add(1) {
		case 1:
			return errors.New("caiu")
		case 2, 3:
			return nil // voltou antes do shutdown
		default:
			cancel()
			return errors.New("fim")
		}
	})
	// ctx já cancelado durante o backoff
	blocked := supervised("parado", logger, func(context.Context) error { return errors.New("x") })
	supervisedMinBackoff, supervisedMaxBackoff = time.Hour, time.Hour
	stuck := supervised("espera", logger, func(context.Context) error { return errors.New("x") })
	supervisedMinBackoff, supervisedMaxBackoff = origMin, origMax

	if err := p(ctx); err != nil || calls.Load() != 4 {
		t.Fatalf("err=%v calls=%d", err, calls.Load())
	}
	cctx, ccancel := context.WithCancel(context.Background())
	ccancel()
	if err := blocked(cctx); err != nil {
		t.Fatal(err)
	}
	sctx, scancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer scancel()
	if err := stuck(sctx); err != nil {
		t.Fatal(err)
	}
}

func TestNotificationHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	hub := ws.NewHub(logger, rdb)
	h := NotificationHandler(hub, logger)
	ctx := context.Background()

	sub := rdb.Subscribe(ctx, redisx.Key("ws"))
	defer sub.Close()
	if _, err := sub.Receive(ctx); err != nil { // confirmação da assinatura
		t.Fatal(err)
	}

	// agenda privada e conteúdo com público-alvo (ADR 014) nunca são difundidos
	for _, ev := range []events.Event{
		{Type: "calendar.event.created", Payload: []byte(`{"visibility":"private"}`)},
		{Type: "blog.post.published", Payload: []byte(`{"restrito":true}`)},
	} {
		if err := h(ctx, ev); err != nil {
			t.Fatal(err)
		}
	}
	for _, ev := range []events.Event{
		{Type: "calendar.event.created", Payload: []byte(`{"visibility":"public"}`)},
		{Type: "blog.post.published", Payload: []byte(`{}`)},
	} {
		if err := h(ctx, ev); err != nil {
			t.Fatalf("%s: %v", ev.Type, err)
		}
	}
	// só os dois públicos chegaram ao backplane
	for _, want := range []string{"calendar.event.created", "blog.post.published"} {
		msg, err := sub.ReceiveMessage(ctx)
		if err != nil || !strings.Contains(msg.Payload, want) {
			t.Fatalf("mensagem = %v, %v (quer %s)", msg, err, want)
		}
	}

	// evento que não serializa: o erro volta ao consumidor (nack/retry)
	if err := h(ctx, events.Event{Type: "blog.post.published", Payload: []byte(`ilegível`)}); err == nil {
		t.Fatal("falha engolida")
	}
}

func TestNewDependenciesLoadsConfigFromEnv(t *testing.T) {
	cfg := infraConfig(t)
	t.Setenv("APP_ENV", "")
	if _, err := NewDependencies(context.Background(), "api"); err == nil || !strings.Contains(err.Error(), "load config") {
		t.Fatalf("sem APP_ENV: %v", err)
	}

	for k, v := range map[string]string{
		"APP_ENV": "test", "APP_LOG_LEVEL": "error",
		"DB_HOST": cfg.Database.Host, "DB_PORT": strconv.Itoa(cfg.Database.Port), "DB_NAME": cfg.Database.Name,
		"DB_USER": cfg.Database.User, "DB_PASSWORD": cfg.Database.Password, "DB_SSLMODE": "disable",
		"REDIS_URL": cfg.Redis.URL, "RABBITMQ_URL": cfg.RabbitMQ.URL,
		"MINIO_ENDPOINT": cfg.MinIO.Endpoint, "MINIO_ACCESS_KEY": cfg.MinIO.AccessKey, "MINIO_SECRET_KEY": cfg.MinIO.SecretKey,
		"LOCAL_AUTH_ENABLED": "true", "LOCAL_AUTH_PRIVATE_KEY": cfg.LocalAuth.PrivateKeyPEM,
	} {
		t.Setenv(k, v)
	}
	d, err := NewDependencies(context.Background(), "api")
	if err != nil {
		t.Fatal(err)
	}
	d.Close()
}
