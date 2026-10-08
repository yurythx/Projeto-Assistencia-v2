package config

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestLoadRejectsMalformedValues(t *testing.T) {
	for key, val := range map[string]string{
		"DB_MAX_CONNS":      "muitos",
		"DB_MIN_CONNS":      "-1",
		"HTTP_READ_TIMEOUT": "rápido",
		"MINIO_USE_SSL":     "talvez",
		"MINIO_PUBLIC_URL":  "://sem-host",
	} {
		t.Run(key, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv(key, val)
			if _, err := Load(); err == nil || !strings.Contains(err.Error(), key) {
				t.Fatalf("%s=%q deveria ser erro de configuração: %v", key, val, err)
			}
		})
	}
}

func TestLoadDerivesPublicMinioEndpointAndAddr(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("MINIO_PUBLIC_URL", "https://arquivos.orgao.gov.br")
	t.Setenv("HTTP_HOST", "127.0.0.1")
	t.Setenv("HTTP_PORT", "8080")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MinIO.PublicEndpoint != "arquivos.orgao.gov.br" || !cfg.MinIO.PublicUseSSL {
		t.Fatalf("endpoint público derivado da URL: %+v", cfg.MinIO)
	}
	if cfg.HTTP.Addr() != "127.0.0.1:8080" {
		t.Fatalf("addr: %s", cfg.HTTP.Addr())
	}
}

func TestProductionRejectsWeakKeyAndOpenEgress(t *testing.T) {
	for name, env := range map[string]map[string]string{
		"chave de cifragem padrão": {"CONFIG_ENCRYPTION_KEY": insecureConfigEncryptionKey},
		"anti-SSRF desligado":      {"EGRESS_ALLOW_PRIVATE_NETWORKS": "true"},
	} {
		t.Run(name, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("APP_ENV", "production")
			t.Setenv("MINIO_ACCESS_KEY", "nexus-prod-access")
			t.Setenv("MINIO_SECRET_KEY", "nexus-prod-secret-strong-value")
			t.Setenv("CONFIG_ENCRYPTION_KEY", "cHJvZC1zdHJvbmcta2V5LTMyLWJ5dGVzLWxvbmchIQ==")
			for k, v := range env {
				t.Setenv(k, v)
			}
			if _, err := Load(); err == nil {
				t.Fatal("produção recusa")
			}
		})
	}
}

func TestLoadDatabaseRejectsMalformedDurations(t *testing.T) {
	t.Setenv("DB_HOST", "h")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_NAME", "n")
	t.Setenv("DB_USER", "u")
	t.Setenv("DB_PASSWORD", "p")
	t.Setenv("DB_MAX_CONN_LIFETIME", "eterno")
	if _, err := LoadDatabase(); err == nil || !strings.Contains(err.Error(), "DB_MAX_CONN_LIFETIME") {
		t.Fatalf("duração inválida: %v", err)
	}
}

func TestLoadIntegersProxiesAndLocalAuth(t *testing.T) {
	for key, val := range map[string]string{"HTTP_PORT": "oitenta", "DB_MAX_CONNS": "99999999999"} {
		t.Run(key, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv(key, val)
			if _, err := Load(); err == nil || !strings.Contains(err.Error(), key) {
				t.Fatalf("%s=%q: %v", key, val, err)
			}
		})
	}
	setRequiredEnv(t)
	t.Setenv("DB_MAX_CONNS", "10")
	t.Setenv("HTTP_READ_TIMEOUT", "3s")
	t.Setenv("TRUSTED_PROXIES", " 10.0.0.0/8 , ,172.16.0.1 ")
	t.Setenv("WORKER_METRICS_HOST", "0.0.0.0")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTP.ReadTimeout != 3*time.Second || cfg.Database.MaxConns != 10 || len(cfg.Security.TrustedProxies) != 2 || cfg.Security.TrustedProxies[1] != "172.16.0.1" {
		t.Fatalf("pool e proxies: %d %v", cfg.Database.MaxConns, cfg.Security.TrustedProxies)
	}
	if !strings.HasSuffix(cfg.Worker.MetricsAddr(), fmt.Sprintf(":%d", cfg.Worker.MetricsPort)) {
		t.Fatalf("metrics addr: %s", cfg.Worker.MetricsAddr())
	}
	t.Setenv("LOCAL_AUTH_ENABLED", "true")
	t.Setenv("LOCAL_AUTH_PRIVATE_KEY", "")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "LOCAL_AUTH_PRIVATE_KEY") {
		t.Fatalf("login local sem chave: %v", err)
	}
}

// ATLAS_AI_ENDPOINT: URL http(s) com host e sem credenciais embutidas.
func TestAtlasAIEndpoint(t *testing.T) {
	for _, raw := range []string{"ftp://ia.local", "http://", "http://user:senha@ia.local", "::nao-e-url"} {
		t.Run(raw, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("ATLAS_AI_ENDPOINT", raw)
			if _, err := Load(); err == nil || !strings.Contains(err.Error(), "ATLAS_AI_ENDPOINT") {
				t.Fatalf("endpoint %q aceito: %v", raw, err)
			}
		})
	}
	setRequiredEnv(t)
	t.Setenv("ATLAS_AI_ENDPOINT", "http://ollama:11434")
	cfg, err := Load()
	if err != nil || cfg.Atlas.AIEndpoint != "http://ollama:11434" || cfg.Atlas.AIModel != "llama3.2" || cfg.Atlas.ChatRateLimit.MaxRequests != 10 {
		t.Fatalf("configuração do Atlas: %+v %v", cfg.Atlas, err)
	}
}
