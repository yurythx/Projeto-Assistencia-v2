package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setRequiredEnv(t *testing.T) {
	t.Helper()
	vars := map[string]string{
		"APP_ENV":             "development",
		"DB_HOST":             "localhost",
		"DB_PORT":             "5432",
		"DB_NAME":             "nix",
		"DB_USER":             "nix",
		"DB_PASSWORD":         "secret",
		"RABBITMQ_URL":        "amqp://guest:guest@localhost:5672/",
		"REDIS_URL":           "redis://localhost:6379/0",
		"KEYCLOAK_ISSUER_URL": "https://idp.example.com/realms/nix",
		"KEYCLOAK_REALM":      "nix",
		"KEYCLOAK_CLIENT_ID":  "projeto-nexus",
		"KEYCLOAK_AUDIENCE":   "projeto-nexus",
	}
	for k, v := range vars {
		t.Setenv(k, v)
	}
}

func TestLoad_Success(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Database.Host != "localhost" {
		t.Errorf("expected DB host localhost, got %q", cfg.Database.Host)
	}
	if cfg.HTTP.Port != 8000 {
		t.Errorf("expected default HTTP port 8000, got %d", cfg.HTTP.Port)
	}
	if cfg.RabbitMQ.MaxRetries != 3 {
		t.Errorf("expected default max retries 3, got %d", cfg.RabbitMQ.MaxRetries)
	}
}

func TestLoad_MissingRequired(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	// Deixa DB_HOST, RABBITMQ_URL, KEYCLOAK_* sem definir, de propósito.

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing required env vars, got nil")
	}
}

func TestLoad_InvalidAppEnv(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("APP_ENV", "not-a-real-env")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid APP_ENV, got nil")
	}
}

func TestLoad_SecretFromFile_TakesPrecedenceOverDirectEnvVar(t *testing.T) {
	setRequiredEnv(t)

	secretFile := filepath.Join(t.TempDir(), "db_password")
	if err := os.WriteFile(secretFile, []byte("password-from-file\n"), 0o600); err != nil {
		t.Fatalf("write secret file: %v", err)
	}
	t.Setenv("DB_PASSWORD_FILE", secretFile)
	// Continua definida (por setRequiredEnv) como "secret" — o valor do
	// arquivo deve vencer, simulando Docker/Kubernetes secrets montados
	// como arquivo tendo prioridade sobre uma env var direta.
	t.Setenv("DB_PASSWORD", "should-be-ignored")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if cfg.Database.Password != "password-from-file" {
		t.Errorf("Database.Password = %q, want %q (do arquivo, com espaços/quebra de linha removidos)", cfg.Database.Password, "password-from-file")
	}
}

func TestLoad_SecretFile_MissingFileIsAConfigError(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("DB_PASSWORD_FILE", "/does/not/exist")

	_, err := Load()
	if err == nil {
		t.Fatal("esperava um erro quando DB_PASSWORD_FILE aponta para um arquivo inexistente")
	}
}

func TestDatabaseConfig_DSN(t *testing.T) {
	db := DatabaseConfig{
		Host: "db", Port: 5432, Name: "nix", User: "nix", Password: "pw",
		SSLMode: "disable", ConnectTimeout: 5 * time.Second,
	}
	dsn := db.DSN()
	want := "host=db port=5432 dbname=nix user=nix password=pw sslmode=disable connect_timeout=5"
	if dsn != want {
		t.Errorf("DSN() = %q, want %q", dsn, want)
	}
}

// TestLoadDatabase_Success confirma que LoadDatabase lê só as variáveis
// DB_* — sem precisar de nenhuma das outras exigidas por Load()
// (RABBITMQ_URL, KEYCLOAK_*), o motivo dela existir (cmd/seedadmin não
// deveria precisar de um Keycloak/RabbitMQ configurados só pra escrever
// uma linha em "users").
func TestLoadDatabase_Success(t *testing.T) {
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_NAME", "nix")
	t.Setenv("DB_USER", "nix")
	t.Setenv("DB_PASSWORD", "secret")

	db, err := LoadDatabase()
	if err != nil {
		t.Fatalf("LoadDatabase: %v", err)
	}
	if db.Host != "localhost" || db.Port != 5432 || db.Name != "nix" || db.User != "nix" || db.Password != "secret" {
		t.Errorf("LoadDatabase() = %+v, want host/port/name/user/password from env", db)
	}
	if db.SSLMode != "disable" {
		t.Errorf("SSLMode default = %q, want disable", db.SSLMode)
	}
}

func TestLoadDatabase_MissingRequired(t *testing.T) {
	// As obrigatórias vazias (e sem _FILE): o teste não depende do ambiente
	// em que roda — num container com DB_* definidas ele passaria a falhar.
	for _, key := range []string{"DB_HOST", "DB_PORT", "DB_NAME", "DB_USER", "DB_PASSWORD"} {
		t.Setenv(key, "")
		t.Setenv(key+"_FILE", "")
	}
	_, err := LoadDatabase()
	if err == nil {
		t.Fatal("expected error for missing required DB_* env vars, got nil")
	}
}

func TestLoad_ProductionRejectsInsecureDefaultSecrets(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("APP_ENV", "production")
	// Não define MINIO_* -> ficam no default inseguro.

	_, err := Load()
	if err == nil {
		t.Fatal("Load() deveria recusar produção com segredos no default inseguro")
	}
	for _, want := range []string{"MINIO_ACCESS_KEY", "MINIO_SECRET_KEY"} {
		if !contains(err.Error(), want) {
			t.Errorf("erro deveria citar %s: %v", want, err)
		}
	}
}

func TestLoad_ProductionAcceptsStrongSecrets(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("MINIO_ACCESS_KEY", "nexus-prod-access")
	t.Setenv("MINIO_SECRET_KEY", "nexus-prod-secret-strong-value")
	t.Setenv("CONFIG_ENCRYPTION_KEY", "cHJvZC1zdHJvbmcta2V5LTMyLWJ5dGVzLWxvbmchIQ==")

	if _, err := Load(); err != nil {
		t.Fatalf("Load() com segredos fortes em produção não deveria falhar: %v", err)
	}
}

func TestLoad_DevelopmentAllowsDefaultSecrets(t *testing.T) {
	setRequiredEnv(t) // APP_ENV=development
	if _, err := Load(); err != nil {
		t.Fatalf("Load() em development com defaults deveria passar: %v", err)
	}
}

// TestLoad_ProductionRejectsExampleDBPassword cobre o achado de auditoria:
// DB_PASSWORD=dev-change-this-db-password é o valor literal commitado em
// .env.example (público), não um default de código — mas precisa ser
// recusado em produção do mesmo jeito que os defaults do MinIO.
func TestLoad_ProductionRejectsExampleDBPassword(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("MINIO_ACCESS_KEY", "nexus-prod-access")
	t.Setenv("MINIO_SECRET_KEY", "nexus-prod-secret-strong-value")
	t.Setenv("DB_PASSWORD", exampleDBPassword)

	_, err := Load()
	if err == nil {
		t.Fatal("Load() deveria recusar produção com DB_PASSWORD igual ao valor de .env.example")
	}
	if !contains(err.Error(), "DB_PASSWORD") {
		t.Errorf("erro deveria citar DB_PASSWORD: %v", err)
	}
}

// TestLoad_ProductionRejectsExampleRabbitMQPassword cobre o mesmo achado
// para RABBITMQ_URL — a senha de .env.example fica embutida na URL de
// conexão (amqp://user:pass@host), então a checagem é por substring, não
// igualdade exata.
func TestLoad_ProductionRejectsExampleRabbitMQPassword(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("MINIO_ACCESS_KEY", "nexus-prod-access")
	t.Setenv("MINIO_SECRET_KEY", "nexus-prod-secret-strong-value")
	t.Setenv("RABBITMQ_URL", "amqp://nexus:"+exampleRabbitMQPassword+"@rabbitmq:5672/nexus")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() deveria recusar produção com a senha de RabbitMQ de .env.example")
	}
	if !contains(err.Error(), "RABBITMQ_URL") {
		t.Errorf("erro deveria citar RABBITMQ_URL: %v", err)
	}
}

// A senha de Redis de .env.example (usada pelo docker-compose) também fica
// embutida na URL; mesma checagem por substring do RabbitMQ.
func TestLoad_ProductionRejectsExampleRedisPassword(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("MINIO_ACCESS_KEY", "nexus-prod-access")
	t.Setenv("MINIO_SECRET_KEY", "nexus-prod-secret-strong-value")
	t.Setenv("REDIS_URL", "redis://:"+exampleRedisPassword+"@redis:6379/0")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() deveria recusar produção com a senha de Redis de .env.example")
	}
	if !contains(err.Error(), "REDIS_URL") {
		t.Errorf("erro deveria citar REDIS_URL: %v", err)
	}
}

// TestLoad_ProductionRejectsInsecureConfigEncryptionKey cobre a nova chave
// de cifragem (ver internal/platform/secretcrypto e keycloakconfig): sem
// CONFIG_ENCRYPTION_KEY definida, o processo cairia no default público
// insecureConfigEncryptionKey — inaceitável em produção, já que qualquer
// pessoa com o código-fonte conseguiria decifrar um Client Secret do
// Keycloak salvo pelo menu Configurações > Keycloak.
func TestLoad_ProductionRejectsInsecureConfigEncryptionKey(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("MINIO_ACCESS_KEY", "nexus-prod-access")
	t.Setenv("MINIO_SECRET_KEY", "nexus-prod-secret-strong-value")
	// Não define CONFIG_ENCRYPTION_KEY -> fica no default inseguro.

	_, err := Load()
	if err == nil {
		t.Fatal("Load() deveria recusar produção com CONFIG_ENCRYPTION_KEY no default inseguro")
	}
	if !contains(err.Error(), "CONFIG_ENCRYPTION_KEY") {
		t.Errorf("erro deveria citar CONFIG_ENCRYPTION_KEY: %v", err)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
