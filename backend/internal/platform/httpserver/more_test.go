package httpserver

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRouterSurfaceAndMetricsToken(t *testing.T) {
	proxies, _ := ParseTrustedProxies([]string{"10.0.0.0/8"})
	r := New(Options{Logger: testLogger(), AllowedOrigins: []string{"https://nexus.gov.br"}, RequestTimeout: time.Second, MetricsToken: "s3cr3t", TrustedProxies: proxies})
	for _, p := range []string{"/health", "/livez", "/healthz"} {
		for _, m := range []string{http.MethodGet, http.MethodHead} {
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(m, p, nil))
			if rec.Code != http.StatusOK {
				t.Errorf("%s %s: %d", m, p, rec.Code)
			}
		}
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("/metrics sem token: %d", rec.Code)
	}
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer s3cr3t")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "go_goroutines") {
		t.Fatalf("/metrics com token: %d", rec.Code)
	}
	open := New(Options{Logger: testLogger(), RequestTimeout: time.Second})
	rec = httptest.NewRecorder()
	open.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/metrics sem token configurado: %d", rec.Code)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("CORS com credenciais e '*' deveria recusar subir")
		}
	}()
	New(Options{Logger: testLogger(), AllowedOrigins: []string{"*"}})
}

func TestRateLimitFailsOpen(t *testing.T) {
	l := newCountLimiter(0)
	l.err = errors.New("redis fora")
	h := RateLimit(testLogger(), l, ClientIPKey)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) }))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != 204 {
		t.Fatalf("limitador fora do ar deixa passar: %d", rec.Code)
	}
}

func TestProxyParsingAndClientIP(t *testing.T) {
	nets, err := ParseTrustedProxies([]string{"10.0.0.0/8", "203.0.113.9", "2001:db8::1"})
	if err != nil || len(nets) != 3 || nets[1].Mask.String() != "ffffffff" {
		t.Fatalf("CIDR, IPv4 puro (/32) e IPv6 puro (/128): %v %v", nets, err)
	}
	if ones, _ := nets[2].Mask.Size(); ones != 128 {
		t.Fatalf("IPv6 puro vira /128: %d", ones)
	}
	if _, err := ParseTrustedProxies([]string{"nao-e-ip"}); err == nil {
		t.Fatal("proxy inválido recusa subir")
	}
	req := func(remote, xff string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = remote
		if xff != "" {
			r.Header.Set("X-Forwarded-For", xff)
		}
		return r
	}
	for _, c := range []struct{ remote, xff, want string }{
		{"10.0.0.2:1", "", "10.0.0.2"},                       // proxy sem XFF
		{"10.0.0.2:1", " , 10.0.0.3", "10.0.0.2"},            // só proxies e vazios no XFF
		{"10.0.0.2:1", "lixo, 198.51.100.7", "198.51.100.7"}, // mais à direita não confiável
		{"10.0.0.2", "198.51.100.8", "198.51.100.8"},         // RemoteAddr sem porta
		{"pipe", "198.51.100.8", "pipe"},                     // RemoteAddr que não é IP: nunca confiável
	} {
		if got := ClientIP(req(c.remote, c.xff), nets); got != c.want {
			t.Errorf("ClientIP(%q, %q) = %q, want %q", c.remote, c.xff, got, c.want)
		}
	}
	if ClientIPKey(req("198.51.100.9", "")) != "198.51.100.9" {
		t.Error("ClientIPKey sem porta")
	}

	// TrustedRealIP reescreve o RemoteAddr só quando há um IP real diferente.
	var seen string
	h := TrustedRealIP(nets)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { seen = r.RemoteAddr }))
	h.ServeHTTP(httptest.NewRecorder(), req("10.0.0.2:1", "198.51.100.7"))
	if seen != "198.51.100.7:0" {
		t.Fatalf("IP real: %q", seen)
	}
	h.ServeHTTP(httptest.NewRecorder(), req("10.0.0.2", ""))
	if seen != "10.0.0.2" {
		t.Fatalf("sem XFF, sem troca: %q", seen)
	}
}

func TestTimeoutSkipsWebSocketUpgrade(t *testing.T) {
	var hadDeadline bool
	h := timeoutExceptWebSocket(time.Minute)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, hadDeadline = r.Context().Deadline()
	}))
	ws := httptest.NewRequest(http.MethodGet, "/ws", nil)
	ws.Header.Set("Upgrade", "websocket")
	h.ServeHTTP(httptest.NewRecorder(), ws)
	if hadDeadline {
		t.Fatal("upgrade de WebSocket não recebe timeout")
	}
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api", nil))
	if !hadDeadline {
		t.Fatal("requisição comum recebe timeout")
	}
}
