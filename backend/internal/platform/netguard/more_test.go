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

func TestIPv6FormsThatEmbedIPv4AreBlocked(t *testing.T) {
	for _, s := range []string{"::7f00:1", "::127.0.0.1", "2002:7f00:1::", "2001:0:4136:e378::1", "64:ff9b:1::a00:1", "::ffff:10.0.0.1"} {
		if !IsBlockedIP(netip.MustParseAddr(s)) {
			t.Errorf("%s deveria ser bloqueado (embute um IPv4)", s)
		}
	}
	for _, s := range []string{"2606:4700:4700::1111", "8.8.8.8"} {
		if IsBlockedIP(netip.MustParseAddr(s)) {
			t.Errorf("%s é público", s)
		}
	}
}

func TestValidateURLEdgeCases(t *testing.T) {
	p := Policy{}
	for _, raw := range []string{"https://[::1", "https:///caminho", "https://2130706433./", "https://0x7f.1/", "https://127.1/"} {
		if _, err := ValidateURL(raw, p); !errors.Is(err, ErrBlockedDestination) {
			t.Errorf("%q deveria ser recusada: %v", raw, err)
		}
	}
	if _, err := ValidateURL("https://servico.gov.br./hook", p); err != nil {
		t.Errorf("ponto final no host é aceito: %v", err)
	}
	if _, err := ValidateURL("https://8.8.8.8/hook", p); err != nil {
		t.Errorf("IP público literal é aceito: %v", err)
	}
	if err := controlFunc(p)("tcp", "8.8.8.8:443", nil); err != nil {
		t.Errorf("conexão a IP público: %v", err)
	}
	if looksNumericHost("a..") {
		t.Error("rótulos vazios não são número")
	}
}

func TestControlAndRedirectRules(t *testing.T) {
	if err := controlFunc(Policy{})("tcp", "sem-porta", nil); !errors.Is(err, ErrBlockedDestination) {
		t.Errorf("endereço ilegível: %v", err)
	}
	if err := controlFunc(Policy{AllowPrivate: true})("tcp", "127.0.0.1:80", nil); err != nil {
		t.Errorf("modo permissivo: %v", err)
	}
	c := NewClient(time.Second, Policy{})
	req := func(u string) *http.Request { r, _ := http.NewRequest(http.MethodGet, u, nil); return r }
	if err := c.CheckRedirect(req("http://169.254.169.254/latest"), []*http.Request{req("https://a.gov.br")}); !errors.Is(err, ErrBlockedDestination) {
		t.Errorf("redirecionamento para metadados da nuvem: %v", err)
	}
	if err := c.CheckRedirect(req("https://b.gov.br/"), []*http.Request{req("https://a.gov.br")}); err != nil {
		t.Errorf("redirecionamento válido: %v", err)
	}

	// Mais de 3 redirecionamentos: interrompido.
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, srv.URL+r.URL.Path+"x", http.StatusFound)
	}))
	defer srv.Close()
	_, err := NewClient(2*time.Second, Policy{AllowHTTP: true, AllowPrivate: true}).Get(srv.URL + "/")
	if !errors.Is(err, ErrBlockedDestination) {
		t.Fatalf("redirecionamentos demais: %v", err)
	}
}

func TestResolveAndCheck(t *testing.T) {
	ctx := context.Background()
	orig := lookupNetIP
	defer func() { lookupNetIP = orig }()
	lookupNetIP = func(_ context.Context, _, host string) ([]netip.Addr, error) {
		switch host {
		case "interno.exemplo":
			return []netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("10.0.0.5")}, nil
		case "publico.exemplo":
			return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
		}
		return nil, errors.New("NXDOMAIN")
	}
	for host, blocked := range map[string]bool{"interno.exemplo": true, "10.1.2.3": true, "publico.exemplo": false, "8.8.4.4": false} {
		err := ResolveAndCheck(ctx, host, Policy{})
		if blocked != errors.Is(err, ErrBlockedDestination) || (!blocked && err != nil) {
			t.Errorf("%s: %v", host, err)
		}
	}
	if err := ResolveAndCheck(ctx, "nao-existe.exemplo", Policy{}); err == nil || errors.Is(err, ErrBlockedDestination) {
		t.Errorf("DNS falhou: %v", err)
	}
	if err := ResolveAndCheck(ctx, "10.0.0.1", Policy{AllowPrivate: true}); err != nil {
		t.Errorf("modo permissivo: %v", err)
	}
}
