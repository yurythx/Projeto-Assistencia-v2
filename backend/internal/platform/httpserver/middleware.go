package httpserver

import (
	"bufio"
	"context"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/platform/logging"
	"github.com/yurythx/projeto-nexus/internal/platform/metrics"
	"github.com/yurythx/projeto-nexus/pkg/httputil"
)

const RequestIDHeader = "X-Request-ID"

// RequestID respeita um header X-Request-ID de entrada ou gera um novo,
// devolve o mesmo valor na resposta, e o guarda no contexto da requisição
// para que toda linha de log e chamada subsequente (banco, outbox,
// RabbitMQ) possa carregá-lo — é a base da correlação de logs entre
// serviços descrita em §50.
//
// O mesmo valor também vira o correlation_id do contexto (achado real
// desta revisão: logging.WithCorrelationID nunca era chamado em lugar
// nenhum, então o campo correlation_id nunca aparecia em nenhum log que
// passa por logging.FromContext — ex.: a própria linha de acesso HTTP
// abaixo, panic recovered, rate limiter check failed — apesar do
// comentário de FromContext e do README prometerem os três campos
// "sempre que disponíveis"). Nunca diverge de request_id aqui na origem
// — os handlers que precisam de um correlation_id pra passar adiante
// pra uma operação de negócio já reaproveitam exatamente este mesmo
// valor; as duas chaves de contexto continuam distintas (não uma só)
// porque um fluxo de negócio pode um dia abranger mais de uma requisição
// HTTP, cada uma com seu próprio request_id mas o mesmo correlation_id.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(RequestIDHeader)
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set(RequestIDHeader, id)
		ctx := logging.WithRequestID(r.Context(), id)
		ctx = logging.WithCorrelationID(ctx, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AccessLog loga uma linha estruturada por requisição, com método, path,
// status, duração e o request id.
func AccessLog(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(ww, r)

			logging.FromContext(r.Context(), logger).Info("http_request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", ww.status),
				slog.Duration("duration", time.Since(start)),
			)
		})
	}
}

// statusRecorder envolve um http.ResponseWriter só para capturar qual
// status code foi de fato escrito — o ResponseWriter padrão não expõe
// isso, e tanto AccessLog quanto Metrics precisam saber o status final
// depois que o handler já rodou.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// Hijack repassa para o http.ResponseWriter original, se ele suportar
// hijacking — necessário para que o upgrade de WebSocket (GET /ws)
// funcione atravessando AccessLog/Metrics. Embutir http.ResponseWriter
// (um valor de interface) só promove os métodos DA INTERFACE
// http.ResponseWriter (Header/Write/WriteHeader); não promove Hijack,
// mesmo que o valor concreto por trás dela implemente http.Hijacker —
// sem este método explícito, ww.(http.Hijacker) falha silenciosamente e
// gorilla/websocket rejeita todo upgrade com "response does not
// implement http.Hijacker", quebrando WebSocket em produção mesmo com o
// servidor HTTP subjacente suportando hijacking perfeitamente bem (bug
// real, encontrado ao vivo: o indicador de conexão do frontend ficava
// preso em "Reconectando…" para sempre).
func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("httpserver: underlying ResponseWriter does not support hijacking")
	}
	return hijacker.Hijack()
}

// Metrics registra nexus_http_requests_total e
// nexus_http_request_duration_seconds (§53). Rotula pelo *padrão* de rota
// que o chi casou (ex.: "/api/v1/users/{id}"), lido do contexto de
// roteamento depois que ServeHTTP retorna — nunca o path bruto, que
// explodiria a cardinalidade das métricas com uma série por id de usuário.
func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(ww, r)

		route := "unmatched"
		if rctx := chi.RouteContext(r.Context()); rctx != nil {
			if pattern := rctx.RoutePattern(); pattern != "" {
				route = pattern
			}
		}

		metrics.HTTPRequestsTotal.WithLabelValues(r.Method, route, strconv.Itoa(ww.status)).Inc()
		metrics.HTTPRequestDuration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
	})
}

// Recoverer converte um panic em qualquer handler downstream numa resposta
// 500 estruturada, em vez de derrubar o processo inteiro ou vazar um stack
// trace para o cliente.
func Recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logging.FromContext(r.Context(), logger).Error("panic recovered",
						slog.Any("panic", rec),
						slog.String("path", r.URL.Path),
					)
					httputil.WriteError(w, r, logger, apperrors.Internal(fmt.Errorf("panic: %v", rec)))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeaders define os headers de segurança de base em toda resposta.
// O CSP com nonce (política mais forte, por requisição) é gerado à parte
// no proxy.ts do frontend — estes aqui são os headers estáticos que fazem
// sentido em qualquer resposta da API, independente de rota.
//
// Gap G-03 da auditoria de conformidade: antes desta revisão não havia
// Content-Security-Policy nenhuma nas respostas do backend, e o
// Strict-Transport-Security só era emitido quando r.TLS != nil — atrás de
// um proxy TLS terminador (cenário padrão em produção Gov) r.TLS é nil e
// o header nunca saía. Agora os dois são incondicionais: um handler que
// serve HTML (ex.: /docs com Swagger UI) sobrescreve a CSP no próprio
// handler; para toda resposta de API (JSON, CSV) a política mais
// restritiva possível é a correta, já que nada ali é documento executável.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'")
		// Incondicional: o navegador ignora HSTS quando a resposta chega
		// por HTTP puro (dev local), então mandar sempre é inofensivo e
		// não depende de detectar TLS terminado upstream.
		h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
		h.Set("Cross-Origin-Resource-Policy", "same-site")
		next.ServeHTTP(w, r)
	})
}

// requireMetricsToken protege /metrics exigindo "Authorization: Bearer
// <token>" (gap G-02). Comparação em tempo constante para o endpoint não
// virar um oráculo de temporização sobre o token. Só é montado quando um
// token está configurado (SecurityConfig.MetricsToken) — sem token, o
// endpoint segue aberto (comportamento histórico, para dev/rede interna).
func requireMetricsToken(token string) func(http.Handler) http.Handler {
	want := []byte("Bearer " + token)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got := []byte(r.Header.Get("Authorization"))
			if subtle.ConstantTimeCompare(got, want) != 1 {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Limiter decide se uma requisição identificada por key é permitida agora.
// RateLimit é desacoplado da implementação: a plataforma usa
// ratelimit.RedisLimiter, que compartilha o estado entre as réplicas.
type Limiter interface {
	Allow(ctx context.Context, key string) (bool, error)
}

// RateLimit retorna um middleware que limita cada cliente (identificado
// por keyFunc, tipicamente o id do usuário autenticado ou o IP remoto) de
// acordo com limiter. Um erro do Limiter (ex.: o Redis por trás do
// RedisLimiter fica brevemente inalcançável) falha ABERTO — a
// requisição é deixada passar e o erro é apenas logado — em vez de deixar
// uma instabilidade do rate limiter virar uma indisponibilidade completa
// do endpoint que ele protege.
func RateLimit(logger *slog.Logger, limiter Limiter, keyFunc func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFunc(r)
			allowed, err := limiter.Allow(r.Context(), key)
			if err != nil {
				logging.FromContext(r.Context(), logger).Error("rate limiter check failed, allowing request", slog.Any("error", err))
				next.ServeHTTP(w, r)
				return
			}
			if !allowed {
				httputil.WriteError(w, r, logger, apperrors.RateLimited("too many requests, please try again later"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ClientIPKey extrai só o IP de r.RemoteAddr (que vem como "host:porta").
// Dois motivos para não devolver o RemoteAddr cru:
//   - a auditoria grava isto na coluna ip_address (tipo `inet`), e
//     "172.19.0.8:40752"::inet estoura ("invalid input syntax for type
//     inet") — toda escrita de auditoria com IP falhava no Postgres.
//   - como chave de rate limiter, a porta efêmera muda a cada conexão
//     TCP, então cada conexão ganhava um bucket novo e o limite por IP
//     não segurava nada.
//
// Não confia em X-Forwarded-For de propósito: sem um proxy reverso de
// confiança à frente terminando esse header, um cliente poderia forjá-lo
// para escapar do rate limiter. Quando houver esse proxy, é ele que deve
// popular o RemoteAddr real (ou trocar esta função por uma que valide a
// cadeia XFF contra a lista de proxies confiáveis).
func ClientIPKey(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	// RemoteAddr sem porta (raro, mas possível em testes/execução fora de
	// um servidor HTTP real) — usa como está.
	return r.RemoteAddr
}

// ParseTrustedProxies converte uma lista de CIDRs (ex.:
// "10.0.0.0/8", "172.18.0.0/16") em *net.IPNet. Um item inválido faz a
// função retornar erro — a configuração de proxies confiáveis é de
// segurança, então é melhor o processo recusar subir do que confiar numa
// faixa errada. Aceita também um IP puro ("203.0.113.9"), tratado como
// /32 ou /128.
func ParseTrustedProxies(cidrs []string) ([]*net.IPNet, error) {
	out := make([]*net.IPNet, 0, len(cidrs))
	for _, raw := range cidrs {
		if _, ipNet, err := net.ParseCIDR(raw); err == nil {
			out = append(out, ipNet)
			continue
		}
		if ip := net.ParseIP(raw); ip != nil {
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			out = append(out, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
			continue
		}
		return nil, fmt.Errorf("httpserver: %q não é um CIDR nem um IP válido", raw)
	}
	return out, nil
}

// ClientIP devolve o IP real do cliente. Só lê o X-Forwarded-For quando a
// conexão TCP (r.RemoteAddr) vem de um proxy confiável (trusted) — nesse
// caso usa a entrada mais à direita do XFF que NÃO seja ela mesma um
// proxy confiável (a primeira, da direita pra esquerda, que um proxy
// confiável não poderia ter forjado). Sem proxy confiável configurado, ou
// com a conexão vindo de fora da lista, devolve só o host de RemoteAddr —
// o mesmo comportamento seguro de ClientIPKey (gap G-04: sem isto, um
// cliente forja o XFF para escapar do rate limiter e para poluir o
// ip_address da prova de consentimento LGPD).
func ClientIP(r *http.Request, trusted []*net.IPNet) string {
	host := r.RemoteAddr
	if h, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		host = h
	}
	if len(trusted) == 0 || !ipInAny(host, trusted) {
		return host
	}
	xff := r.Header.Get("X-Forwarded-For")
	if xff == "" {
		return host
	}
	parts := strings.Split(xff, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		candidate := strings.TrimSpace(parts[i])
		if candidate == "" {
			continue
		}
		if !ipInAny(candidate, trusted) {
			return candidate
		}
	}
	return host
}

func ipInAny(ipStr string, nets []*net.IPNet) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// contextTimeout é um pequeno helper usado pelas verificações de readiness.
func contextTimeout(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, d)
}

// timeoutExceptWebSocket é chimiddleware.Timeout(d) para todo request HTTP
// normal, mas PULA a conexão de upgrade do WebSocket (/ws): ela é
// deliberadamente longa (fica aberta horas) e, sob o Timeout, o contexto
// do handler era cancelado e o AccessLog registrava um "504" com duração
// absurda a cada conexão encerrada — ruído puro nos logs.
func timeoutExceptWebSocket(d time.Duration) func(http.Handler) http.Handler {
	timeout := chimiddleware.Timeout(d)
	return func(next http.Handler) http.Handler {
		wrapped := timeout(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
				next.ServeHTTP(w, r)
				return
			}
			wrapped.ServeHTTP(w, r)
		})
	}
}

// TrustedRealIP reescreve r.RemoteAddr com o IP real do cliente (ver
// ClientIP) quando a conexão TCP vem de um proxy reverso confiável. Roda
// primeiro na cadeia: rate limiting, auditoria e prova de consentimento
// passam a enxergar o IP real sem cada um reimplementar a leitura segura
// do X-Forwarded-For. Sem proxies configurados, é um no-op.
func TrustedRealIP(trusted []*net.IPNet) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if len(trusted) == 0 {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				host = r.RemoteAddr
			}
			if ip := ClientIP(r, trusted); ip != "" && ip != host {
				r.RemoteAddr = net.JoinHostPort(ip, "0")
			}
			next.ServeHTTP(w, r)
		})
	}
}
