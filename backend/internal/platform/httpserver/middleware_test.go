package httpserver

import (
	"bufio"
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/yurythx/projeto-nexus/internal/platform/logging"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestRequestID_GeneratesWhenAbsent(t *testing.T) {
	var captured string
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = logging.RequestID(r.Context())
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if captured == "" {
		t.Error("expected a generated request id in context")
	}
	if rec.Header().Get(RequestIDHeader) != captured {
		t.Error("expected the response header to echo the generated request id")
	}
}

func TestRequestID_HonorsInboundHeader(t *testing.T) {
	var captured string
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = logging.RequestID(r.Context())
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set(RequestIDHeader, "client-supplied-id")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if captured != "client-supplied-id" {
		t.Errorf("captured = %q, want client-supplied-id", captured)
	}
	if rec.Header().Get(RequestIDHeader) != "client-supplied-id" {
		t.Error("expected the response header to echo the client-supplied id")
	}
}

// Achado real desta revisão: logging.WithCorrelationID nunca era chamado
// em lugar nenhum do código — o campo correlation_id nunca aparecia em
// nenhum log passando por logging.FromContext (a própria linha de acesso
// HTTP incluída), apesar do comentário de FromContext e do README
// prometerem os três campos "sempre que disponíveis". Este teste tranca
// a correção: o mesmo valor do request_id também precisa aparecer como
// correlation_id no contexto.
func TestRequestID_AlsoSetsCorrelationID(t *testing.T) {
	var gotRequestID, gotCorrelationID string
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRequestID = logging.RequestID(r.Context())
		gotCorrelationID = logging.CorrelationID(r.Context())
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if gotCorrelationID == "" {
		t.Fatal("expected a correlation id in context, got empty string")
	}
	if gotCorrelationID != gotRequestID {
		t.Errorf("correlation_id = %q, want the same value as request_id (%q)", gotCorrelationID, gotRequestID)
	}
}

func TestRecoverer_ConvertsPanicToJSON500(t *testing.T) {
	handler := Recoverer(testLogger())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	// Não pode vazar um panic para fora de ServeHTTP.
	handler.ServeHTTP(rec, req)

	if rec.Code != 500 {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestSecurityHeaders_SetsBaselineHeaders(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing X-Content-Type-Options: nosniff")
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("missing X-Frame-Options: DENY")
	}
	// Gap G-03: CSP e HSTS agora incondicionais (antes CSP não existia e
	// HSTS só saía com r.TLS != nil — nil atrás de um proxy TLS).
	if got := rec.Header().Get("Content-Security-Policy"); got != "default-src 'none'; frame-ancestors 'none'; base-uri 'none'" {
		t.Errorf("Content-Security-Policy = %q, want a política restritiva de API", got)
	}
	// Skill §4 (M01/A05): HSTS com preload e Referrer-Policy
	// strict-origin-when-cross-origin.
	if got := rec.Header().Get("Strict-Transport-Security"); got != "max-age=63072000; includeSubDomains; preload" {
		t.Errorf("Strict-Transport-Security = %q, want emitido incondicionalmente com preload", got)
	}
	if got := rec.Header().Get("Referrer-Policy"); got != "strict-origin-when-cross-origin" {
		t.Errorf("Referrer-Policy = %q, want strict-origin-when-cross-origin", got)
	}
}

func TestTrustedRealIP(t *testing.T) {
	trusted, _ := ParseTrustedProxies([]string{"10.0.0.0/8"})
	var seen string
	h := TrustedRealIP(trusted)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { seen = r.RemoteAddr }))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.1.2.3:4444"
	req.Header.Set("X-Forwarded-For", "203.0.113.7, 10.9.9.9")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if seen != "203.0.113.7:0" {
		t.Fatalf("via proxy confiável, RemoteAddr deveria virar o IP real; veio %q", seen)
	}

	spoof := httptest.NewRequest(http.MethodGet, "/", nil)
	spoof.RemoteAddr = "198.51.100.1:5555"
	spoof.Header.Set("X-Forwarded-For", "1.2.3.4")
	h.ServeHTTP(httptest.NewRecorder(), spoof)
	if seen != "198.51.100.1:5555" {
		t.Fatalf("XFF de origem não confiável deve ser ignorado; veio %q", seen)
	}
}

func TestRequireMetricsToken(t *testing.T) {
	guarded := requireMetricsToken("s3cr3t")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("metrics"))
	}))

	t.Run("sem token → 401", func(t *testing.T) {
		rec := httptest.NewRecorder()
		guarded.ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("token errado → 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/metrics", nil)
		req.Header.Set("Authorization", "Bearer wrong")
		rec := httptest.NewRecorder()
		guarded.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("token certo → 200", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/metrics", nil)
		req.Header.Set("Authorization", "Bearer s3cr3t")
		rec := httptest.NewRecorder()
		guarded.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
	})
}

func TestParseTrustedProxies(t *testing.T) {
	nets, err := ParseTrustedProxies([]string{"10.0.0.0/8", "203.0.113.9"})
	if err != nil {
		t.Fatalf("ParseTrustedProxies: %v", err)
	}
	if len(nets) != 2 {
		t.Fatalf("len = %d, want 2", len(nets))
	}
	if _, err := ParseTrustedProxies([]string{"não-é-cidr"}); err == nil {
		t.Error("esperava erro para um CIDR inválido — configuração de segurança deve falhar rápido")
	}
}

func TestClientIP_OnlyTrustsXFFFromTrustedProxy(t *testing.T) {
	trusted, _ := ParseTrustedProxies([]string{"172.18.0.0/16"})

	t.Run("conexão de proxy confiável: usa o XFF", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/", nil)
		req.RemoteAddr = "172.18.0.5:40000"
		req.Header.Set("X-Forwarded-For", "198.51.100.23, 172.18.0.5")
		if got := ClientIP(req, trusted); got != "198.51.100.23" {
			t.Errorf("ClientIP = %q, want 198.51.100.23", got)
		}
	})

	t.Run("conexão direta (não confiável): ignora o XFF forjado", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/", nil)
		req.RemoteAddr = "203.0.113.77:51000"
		req.Header.Set("X-Forwarded-For", "10.0.0.1")
		if got := ClientIP(req, trusted); got != "203.0.113.77" {
			t.Errorf("ClientIP = %q, want 203.0.113.77 (RemoteAddr, XFF ignorado)", got)
		}
	})

	t.Run("sem proxies confiáveis: sempre RemoteAddr", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/", nil)
		req.RemoteAddr = "192.0.2.10:1234"
		req.Header.Set("X-Forwarded-For", "1.2.3.4")
		if got := ClientIP(req, nil); got != "192.0.2.10" {
			t.Errorf("ClientIP = %q, want 192.0.2.10", got)
		}
	})
}

func TestRateLimit_AllowsWithinBurstThenRejects(t *testing.T) {
	handler := RateLimit(testLogger(), newCountLimiter(2), func(r *http.Request) string { return "same-key" })(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }),
	)

	req := httptest.NewRequest("GET", "/", nil)

	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want 200 (within burst)", i, rec.Code)
		}
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429 once burst is exhausted", rec.Code)
	}
}

func TestRateLimit_DifferentKeysAreIndependent(t *testing.T) {
	callCount := map[string]int{}
	handler := RateLimit(testLogger(), newCountLimiter(1), func(r *http.Request) string { return r.Header.Get("X-User") })(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount[r.Header.Get("X-User")]++
			w.WriteHeader(http.StatusOK)
		}),
	)

	for _, user := range []string{"alice", "bob"} {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-User", user)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("user %s: status = %d, want 200", user, rec.Code)
		}
	}
}

// fakeHijackableWriter é um http.ResponseWriter mínimo que também
// implementa http.Hijacker, para testar que statusRecorder repassa
// Hijack() em vez de escondê-lo atrás do embedding de uma interface (o
// bug real que quebrava GET /ws atrás de AccessLog/Metrics — ver o
// comentário de statusRecorder.Hijack).
type fakeHijackableWriter struct {
	http.ResponseWriter
	hijacked bool
}

func (f *fakeHijackableWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	f.hijacked = true
	return nil, nil, nil
}

func TestStatusRecorder_HijackDelegatesToUnderlyingWriter(t *testing.T) {
	underlying := &fakeHijackableWriter{ResponseWriter: httptest.NewRecorder()}
	rec := &statusRecorder{ResponseWriter: underlying, status: http.StatusOK}

	hijacker, ok := (http.ResponseWriter(rec)).(http.Hijacker)
	if !ok {
		t.Fatal("statusRecorder does not satisfy http.Hijacker — WebSocket upgrades behind AccessLog/Metrics would fail")
	}
	if _, _, err := hijacker.Hijack(); err != nil {
		t.Fatalf("Hijack: unexpected error %v", err)
	}
	if !underlying.hijacked {
		t.Error("expected the underlying ResponseWriter's Hijack to have been called")
	}
}

func TestStatusRecorder_HijackErrorsWhenUnderlyingDoesNotSupportIt(t *testing.T) {
	rec := &statusRecorder{ResponseWriter: httptest.NewRecorder(), status: http.StatusOK}

	if _, _, err := rec.Hijack(); err == nil {
		t.Fatal("expected an error when the underlying ResponseWriter does not implement http.Hijacker")
	}
}

// countLimiter permite n requisições por chave (dublê simples do Limiter).
type countLimiter struct {
	max  int
	seen map[string]int
	err  error
}

func newCountLimiter(n int) *countLimiter { return &countLimiter{max: n, seen: map[string]int{}} }

func (c *countLimiter) Allow(_ context.Context, key string) (bool, error) {
	if c.err != nil {
		return false, c.err
	}
	c.seen[key]++
	return c.seen[key] <= c.max, nil
}
