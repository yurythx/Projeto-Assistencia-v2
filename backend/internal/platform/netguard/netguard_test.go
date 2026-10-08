package netguard

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"
)

func TestIsBlockedIP(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "10.1.2.3", "172.16.0.1", "172.31.255.255", "192.168.1.1",
		"169.254.169.254", "100.64.0.1", "0.0.0.0", "::1", "fe80::1", "fd00::1",
		"::ffff:127.0.0.1", "::ffff:169.254.169.254", "224.0.0.1", "255.255.255.255",
	}
	for _, s := range blocked {
		if !IsBlockedIP(netip.MustParseAddr(s)) {
			t.Errorf("%s deveria ser bloqueado", s)
		}
	}
	allowed := []string{"8.8.8.8", "200.160.2.3", "2001:4860:4860::8888", "172.32.0.1"}
	for _, s := range allowed {
		if IsBlockedIP(netip.MustParseAddr(s)) {
			t.Errorf("%s deveria ser permitido", s)
		}
	}
}

func TestValidateURL(t *testing.T) {
	strict := Policy{}
	bad := []string{
		"ftp://example.com/x",
		"http://example.com/hook", // http exige AllowHTTP
		"https://user:pass@example.com/",
		"https://localhost/hook",
		"https://api.localhost/hook",
		"https://metadata.google.internal/computeMetadata/v1/",
		"https://169.254.169.254/latest/meta-data/",
		"https://[::1]/",
		"https://10.0.0.5/",
		"https://2130706433/",
		"https://0x7f000001/",
		"https://127.1/",
		"https://nexus.intranet/",
		"file:///etc/passwd",
		"gopher://evil",
	}
	for _, u := range bad {
		if _, err := ValidateURL(u, strict); !errors.Is(err, ErrBlockedDestination) {
			t.Errorf("%q deveria ser recusada, err=%v", u, err)
		}
	}
	good := []string{"https://hooks.example.com/n8n/abc", "https://zabbix.example.org:8443/api_jsonrpc.php"}
	for _, u := range good {
		if _, err := ValidateURL(u, strict); err != nil {
			t.Errorf("%q deveria ser aceita: %v", u, err)
		}
	}
	if _, err := ValidateURL("http://n8n:5678/webhook", Policy{AllowPrivate: true, AllowHTTP: true}); err != nil {
		t.Errorf("modo desenvolvimento deveria aceitar host interno: %v", err)
	}
}

// O Dialer precisa barrar a conexão pelo IP real, mesmo quando a URL
// passou pela validação (cenário de DNS rebinding simulado com um
// servidor de teste em 127.0.0.1).
func TestClientBlocksLoopbackAtDialTime(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(2*time.Second, Policy{AllowHTTP: true})
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	_, err := client.Do(req)
	if err == nil || !errors.Is(err, ErrBlockedDestination) {
		t.Fatalf("conexão a loopback deveria ser barrada no dial, err=%v", err)
	}

	permissive := NewClient(2*time.Second, Policy{AllowHTTP: true, AllowPrivate: true})
	resp, err := permissive.Do(req)
	if err != nil {
		t.Fatalf("modo permissivo deveria conectar: %v", err)
	}
	_ = resp.Body.Close()
}
