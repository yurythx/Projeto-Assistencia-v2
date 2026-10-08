// Package netguard protege chamadas HTTP de saída contra SSRF (OWASP A10).
//
// Duas camadas:
//  1. ValidateURL — rejeita, no cadastro, esquemas não-HTTP(S), credenciais
//     embutidas, hosts internos conhecidos e IPs literais proibidos.
//  2. NewClient — um *http.Client cujo Dialer confere o IP REAL de cada
//     conexão (net.Dialer.Control roda depois da resolução DNS). Isso
//     derrota DNS rebinding: um host que resolvia para um IP público no
//     cadastro e passa a resolver para 169.254.169.254 na entrega é
//     barrado na hora do connect, não só na validação.
//
// Faixas bloqueadas: RFC 1918 (10/8, 172.16/12, 192.168/16), loopback
// (127/8, ::1), link-local (169.254/16 — inclui o metadata endpoint
// 169.254.169.254 — e fe80::/10), CGNAT (100.64/10), "this network"
// (0/8), multicast, broadcast, ULA IPv6 (fc00::/7), IPv4 mapeado em IPv6
// de qualquer uma delas, e faixas de documentação/benchmark.
package netguard

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// ErrBlockedDestination é devolvido quando o destino cai numa faixa
// proibida (ou não pôde ser validado).
var ErrBlockedDestination = errors.New("netguard: destino bloqueado pela proteção anti-SSRF")

var blockedPrefixes = mustPrefixes(
	"0.0.0.0/8",       // "this network"
	"10.0.0.0/8",      // RFC 1918
	"100.64.0.0/10",   // CGNAT (RFC 6598)
	"127.0.0.0/8",     // loopback
	"169.254.0.0/16",  // link-local + metadata de nuvem (169.254.169.254)
	"172.16.0.0/12",   // RFC 1918
	"192.0.0.0/24",    // IETF protocol assignments
	"192.0.2.0/24",    // TEST-NET-1
	"192.88.99.0/24",  // 6to4 relay anycast
	"192.168.0.0/16",  // RFC 1918
	"198.18.0.0/15",   // benchmark
	"198.51.100.0/24", // TEST-NET-2
	"203.0.113.0/24",  // TEST-NET-3
	"224.0.0.0/4",     // multicast
	"240.0.0.0/4",     // reservado + broadcast
	"::/96",           // unspecified, loopback e IPv4-compatível obsoleto (::127.0.0.1)
	"64:ff9b::/96",    // NAT64 (pode mapear para IPv4 interno)
	"64:ff9b:1::/48",  // NAT64 de uso local (RFC 8215)
	"2001::/32",       // Teredo (embute um IPv4)
	"2002::/16",       // 6to4 (embute um IPv4: 2002:7f00:1:: -> 127.0.0.1)
	"100::/64",        // discard
	"2001:db8::/32",   // documentação
	"fc00::/7",        // ULA
	"fe80::/10",       // link-local
	"ff00::/8",        // multicast
)

// Hostnames internos conhecidos, recusados antes mesmo da resolução DNS.
var blockedHostnames = []string{
	"localhost",
	"metadata.google.internal",
	"metadata",
	"instance-data",
}

var blockedHostSuffixes = []string{
	".localhost",
	".local",
	".internal",
	".intranet",
	".lan",
	".home.arpa",
}

func mustPrefixes(cidrs ...string) []netip.Prefix {
	out := make([]netip.Prefix, 0, len(cidrs))
	for _, c := range cidrs {
		out = append(out, netip.MustParsePrefix(c))
	}
	return out
}

// Policy parametriza a proteção. AllowPrivate só deve ser true em
// desenvolvimento (config.EgressConfig.AllowPrivateNetworks).
type Policy struct {
	AllowPrivate bool
	// AllowHTTP permite http:// (sem TLS). Em produção a política padrão
	// exige https.
	AllowHTTP bool
}

// IsBlockedIP reporta se ip pertence a uma faixa proibida.
func IsBlockedIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || ip.IsUnspecified() {
		return true
	}
	for _, p := range blockedPrefixes {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// ValidateURL confere a URL de destino no momento do cadastro.
func ValidateURL(raw string, policy Policy) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("%w: URL inválida", ErrBlockedDestination)
	}
	switch u.Scheme {
	case "https":
	case "http":
		if !policy.AllowHTTP {
			return nil, fmt.Errorf("%w: apenas https é permitido", ErrBlockedDestination)
		}
	default:
		return nil, fmt.Errorf("%w: esquema %q não permitido", ErrBlockedDestination, u.Scheme)
	}
	if u.User != nil {
		return nil, fmt.Errorf("%w: credenciais embutidas na URL não são permitidas", ErrBlockedDestination)
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if host == "" {
		return nil, fmt.Errorf("%w: host ausente", ErrBlockedDestination)
	}
	if policy.AllowPrivate {
		return u, nil
	}
	for _, h := range blockedHostnames {
		if host == h {
			return nil, fmt.Errorf("%w: host interno %q", ErrBlockedDestination, host)
		}
	}
	for _, s := range blockedHostSuffixes {
		if strings.HasSuffix(host, s) {
			return nil, fmt.Errorf("%w: domínio interno %q", ErrBlockedDestination, host)
		}
	}
	// IP literal (inclusive formas decimais/octais que net/netip rejeita,
	// ex.: "2130706433" ou "0x7f.1" — não são aceitas como IP, e um host
	// numérico puro também é recusado abaixo).
	if addr, err := netip.ParseAddr(host); err == nil {
		if IsBlockedIP(addr) {
			return nil, fmt.Errorf("%w: IP %s em faixa proibida", ErrBlockedDestination, addr)
		}
		return u, nil
	}
	if looksNumericHost(host) {
		return nil, fmt.Errorf("%w: representação numérica de IP não permitida", ErrBlockedDestination)
	}
	return u, nil
}

// looksNumericHost aplica a regra do WHATWG URL Standard: se o último
// rótulo do host é numérico (decimal ou "0x..."), o host é um IPv4 em
// forma ofuscada (ex.: "2130706433", "0x7f000001", "127.1") — nenhum TLD
// real é numérico, então é recusado.
func looksNumericHost(host string) bool {
	labels := strings.Split(host, ".")
	last := labels[len(labels)-1]
	if last == "" && len(labels) > 1 {
		last = labels[len(labels)-2]
	}
	if strings.HasPrefix(last, "0x") {
		return true
	}
	if last == "" {
		return false
	}
	for _, r := range last {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// controlFunc barra a conexão se o IP resolvido estiver bloqueado.
func controlFunc(policy Policy) func(network, address string, _ syscall.RawConn) error {
	return func(network, address string, _ syscall.RawConn) error {
		if policy.AllowPrivate {
			return nil
		}
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return ErrBlockedDestination
		}
		addr, err := netip.ParseAddr(host)
		if err != nil || IsBlockedIP(addr) {
			return fmt.Errorf("%w: conexão para %s recusada", ErrBlockedDestination, host)
		}
		return nil
	}
}

// NewClient cria um *http.Client seguro para chamadas de saída:
//   - Dialer com Control anti-SSRF (IP real da conexão);
//   - sem proxy de ambiente (HTTP_PROXY poderia contornar a checagem);
//   - redirecionamentos revalidados contra a mesma política e limitados a 3;
//   - timeouts de conexão, TLS e resposta.
func NewClient(timeout time.Duration, policy Policy) *http.Client {
	dialer := &net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
		Control:   controlFunc(policy),
	}
	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          50,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: timeout,
		ExpectContinueTimeout: time.Second,
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("%w: redirecionamentos demais", ErrBlockedDestination)
			}
			if _, err := ValidateURL(req.URL.String(), policy); err != nil {
				return err
			}
			return nil
		},
	}
}

// lookupNetIP é o resolvedor de DNS (substituível em teste).
var lookupNetIP = net.DefaultResolver.LookupNetIP

// ResolveAndCheck resolve host e confere todos os IPs — útil para dar um
// erro claro no cadastro (a proteção efetiva continua sendo o Dialer).
func ResolveAndCheck(ctx context.Context, host string, policy Policy) error {
	if policy.AllowPrivate {
		return nil
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		if IsBlockedIP(addr) {
			return ErrBlockedDestination
		}
		return nil
	}
	addrs, err := lookupNetIP(ctx, "ip", host)
	if err != nil {
		return fmt.Errorf("netguard: resolver %q: %w", host, err)
	}
	for _, a := range addrs {
		if IsBlockedIP(a) {
			return fmt.Errorf("%w: %q resolve para %s", ErrBlockedDestination, host, a)
		}
	}
	return nil
}
