package notificacoes

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
)

var logger = slog.New(slog.NewTextHandler(io.Discard, nil))

// hubFalso registra o que foi publicado (e pode falhar).
type hubFalso struct {
	mu      sync.Mutex
	topicos []string
	falha   bool
}

func (h *hubFalso) Publish(_ context.Context, topic, frameType string, _ any) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.falha {
		return errors.New("redis fora")
	}
	h.topicos = append(h.topicos, topic+"|"+frameType)
	return nil
}

func nova(chave string) Nova {
	return Nova{Modulo: "atlas", Titulo: "Procedimento atualizado", Mensagem: "Versão 2", Link: "/atlas/procedimentos/x", Chave: chave}
}

func TestValidar(t *testing.T) {
	if nova("k").Validar() != nil {
		t.Fatal("válida")
	}
	for name, n := range map[string]Nova{
		"sem módulo":           {Titulo: "t", Chave: "k"},
		"sem título":           {Modulo: "m", Chave: "k"},
		"mensagem longa":       {Modulo: "m", Titulo: "t", Chave: "k", Mensagem: strings.Repeat("a", 1001)},
		"link externo":         {Modulo: "m", Titulo: "t", Chave: "k", Link: "https://x.com"},
		"link //":              {Modulo: "m", Titulo: "t", Chave: "k", Link: "//x.com"},
		"link barra-invertida": {Modulo: "m", Titulo: "t", Chave: "k", Link: "/\\x.com"},
		"sem chave":            {Modulo: "m", Titulo: "t"},
	} {
		if n.Validar() == nil {
			t.Errorf("%s aceito", name)
		}
	}
}

func TestCaixaDeNotificacoes(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	hub := &hubFalso{}
	s := NewService(pool, hub, logger)
	a, b, desligou := dbtest.User(t, pool), dbtest.User(t, pool), dbtest.User(t, pool)
	if err := s.DefinirPreferencia(ctx, desligou, Preferencia{Modulo: "atlas", Ativo: false}); err != nil {
		t.Fatal(err)
	}
	chave := "teste:" + uuid.NewString()
	// Usuário inexistente e quem desligou o módulo ficam de fora.
	n, err := s.Enviar(ctx, []uuid.UUID{a, b, desligou, uuid.New()}, nova(chave))
	if err != nil || n != 2 || len(hub.topicos) != 2 || !strings.HasSuffix(hub.topicos[0], "|"+FrameNova) {
		t.Fatalf("enviar: %d %v %v", n, err, hub.topicos)
	}
	// Idempotente: a mesma chave não chega de novo.
	if n, err := s.Enviar(ctx, []uuid.UUID{a, b}, nova(chave)); err != nil || n != 0 {
		t.Fatalf("reenvio: %d %v", n, err)
	}
	if n, err := s.Enviar(ctx, nil, nova("x")); err != nil || n != 0 {
		t.Fatalf("sem destinatários: %d %v", n, err)
	}
	if _, err := s.Enviar(ctx, []uuid.UUID{a}, Nova{}); err == nil {
		t.Fatal("aviso inválido aceito")
	}

	c, err := s.Caixa(ctx, a, 0)
	if err != nil || c.NaoLida != 1 || len(c.Itens) != 1 || c.Itens[0].Titulo != "Procedimento atualizado" || c.Itens[0].Lida {
		t.Fatalf("caixa: %+v %v", c, err)
	}
	if err := s.MarcarLida(ctx, a, c.Itens[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := s.MarcarLida(ctx, b, c.Itens[0].ID); !errors.Is(err, ErrNaoEncontrada) {
		t.Fatalf("notificação de outra pessoa: %v", err)
	}
	if c, _ = s.Caixa(ctx, a, 500); c.NaoLida != 0 || !c.Itens[0].Lida {
		t.Fatalf("lida: %+v", c)
	}
	// Retenção: lida há mais de 180 dias some quando chega um aviso novo.
	if _, err := pool.Exec(ctx, `UPDATE notificacoes SET lida_em = now() - interval '200 days' WHERE user_id = $1`, a); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Enviar(ctx, []uuid.UUID{a}, nova("teste:"+uuid.NewString())); err != nil {
		t.Fatal(err)
	}
	if c, _ = s.Caixa(ctx, a, 10); len(c.Itens) != 1 || c.Itens[0].Lida {
		t.Fatalf("retenção: %+v", c)
	}
	if err := s.MarcarTodas(ctx, b); err != nil {
		t.Fatal(err)
	}
	if c, _ = s.Caixa(ctx, b, 10); c.NaoLida != 0 {
		t.Fatalf("todas lidas: %+v", c)
	}
	if p, err := s.Preferencias(ctx, desligou); err != nil || len(p) != 1 || p[0].Ativo {
		t.Fatalf("preferências: %+v %v", p, err)
	}
	if err := s.DefinirPreferencia(ctx, desligou, Preferencia{}); err == nil {
		t.Fatal("módulo vazio aceito")
	}
	// Tempo real fora: o aviso fica na caixa (só registra no log).
	hub.falha = true
	if n, err := s.Enviar(ctx, []uuid.UUID{a}, nova("teste:"+uuid.NewString())); err != nil || n != 1 {
		t.Fatalf("entrega falhou mas gravou: %d %v", n, err)
	}
	// Sem hub: só grava.
	if n, err := NewService(pool, nil, logger).Enviar(ctx, []uuid.UUID{a}, nova("teste:"+uuid.NewString())); err != nil || n != 1 {
		t.Fatalf("sem hub: %d %v", n, err)
	}
}

func TestFalhasDoBanco(t *testing.T) {
	ctx := context.Background()
	u := uuid.New()
	for name, db := range map[string]*Service{
		"fora":    {db: dbtest.Fail{}, logger: logger},
		"linha":   {db: dbtest.ScanFail{}, logger: logger},
		"leitura": {db: dbtest.RowsErr{}, logger: logger},
	} {
		if _, err := Registrar(ctx, db.db, []uuid.UUID{u}, nova("k")); err == nil {
			t.Errorf("%s: registrar", name)
		}
		if _, err := db.Caixa(ctx, u, 5); err == nil {
			t.Errorf("%s: caixa", name)
		}
		if _, err := db.Preferencias(ctx, u); err == nil {
			t.Errorf("%s: preferências", name)
		}
	}
	// Gravou, mas a limpeza da retenção falhou: o erro sobe (a transação desfaz).
	if _, err := Registrar(ctx, &dbtest.Seq{Queries: []dbtest.QueryResult{{Rows: dbtest.NoRows()}}}, []uuid.UUID{u}, nova("k")); err == nil {
		t.Error("falha na retenção engolida")
	}
	// Lista ok, contagem falha.
	s := &Service{db: &dbtest.Seq{Queries: []dbtest.QueryResult{{Rows: dbtest.NoRows()}}}, logger: logger}
	if _, err := s.Caixa(ctx, u, 5); err == nil {
		t.Error("contagem com falha engolida")
	}
	f := &Service{db: dbtest.Fail{}, logger: logger}
	if err := f.MarcarLida(ctx, u, u); err == nil {
		t.Error("marcar lida")
	}
	if err := f.MarcarTodas(ctx, u); err == nil {
		t.Error("marcar todas")
	}
	if err := f.DefinirPreferencia(ctx, u, Preferencia{Modulo: "atlas"}); err == nil {
		t.Error("definir preferência")
	}
	if err := (&Service{db: &dbtest.Seq{Execs: []dbtest.ExecResult{{Tag: pgconn.NewCommandTag("UPDATE 0")}}}}).MarcarLida(ctx, u, u); !errors.Is(err, ErrNaoEncontrada) {
		t.Errorf("inexistente: %v", err)
	}
}

func TestRotas(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	s := NewService(pool, nil, logger)
	eu := dbtest.User(t, pool)
	if _, err := s.Enviar(ctx, []uuid.UUID{eu}, nova("teste:"+uuid.NewString())); err != nil {
		t.Fatal(err)
	}
	router := func(svc *Service, id auth.Identity) *chi.Mux {
		r := chi.NewRouter()
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				next.ServeHTTP(w, req.WithContext(auth.WithIdentity(req.Context(), id)))
			})
		})
		RegisterRoutes(r, NewHandlers(svc, logger))
		return r
	}
	do := func(r *chi.Mux, method, path, body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(rec, req)
		return rec
	}
	r := router(s, auth.Identity{UserID: eu})
	rec := do(r, http.MethodGet, "/notificacoes?limit=5", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"nao_lidas":1`) {
		t.Fatalf("caixa: %d %s", rec.Code, rec.Body.String())
	}
	c, _ := s.Caixa(ctx, eu, 1)
	for _, tc := range []struct {
		method, path, body string
		want               int
	}{
		{http.MethodPost, "/notificacoes/" + c.Itens[0].ID.String() + "/lida", "", http.StatusNoContent},
		{http.MethodPost, "/notificacoes/" + uuid.NewString() + "/lida", "", http.StatusNotFound},
		{http.MethodPost, "/notificacoes/x/lida", "", http.StatusBadRequest},
		{http.MethodPost, "/notificacoes/lidas", "", http.StatusNoContent},
		{http.MethodGet, "/notificacoes/preferencias", "", http.StatusOK},
		{http.MethodPut, "/notificacoes/preferencias", `{"modulo":"atlas","ativo":false}`, http.StatusOK},
		{http.MethodPut, "/notificacoes/preferencias", `{`, http.StatusBadRequest},
	} {
		if rec := do(r, tc.method, tc.path, tc.body); rec.Code != tc.want {
			t.Errorf("%s %s: %d, quero %d — %s", tc.method, tc.path, rec.Code, tc.want, rec.Body.String())
		}
	}
	// Sem usuário local: 403 em todas.
	anon := router(s, auth.Identity{})
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/notificacoes", ""},
		{http.MethodPost, "/notificacoes/" + uuid.NewString() + "/lida", ""},
		{http.MethodPost, "/notificacoes/lidas", ""},
		{http.MethodGet, "/notificacoes/preferencias", ""},
		{http.MethodPut, "/notificacoes/preferencias", `{"modulo":"atlas","ativo":true}`},
	} {
		if rec := do(anon, tc.method, tc.path, tc.body); rec.Code != http.StatusForbidden {
			t.Errorf("sem usuário %s %s: %d", tc.method, tc.path, rec.Code)
		}
	}
	// Banco fora: 500.
	fora := router(&Service{db: dbtest.Fail{}, logger: logger}, auth.Identity{UserID: eu})
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/notificacoes", ""},
		{http.MethodPost, "/notificacoes/lidas", ""},
		{http.MethodGet, "/notificacoes/preferencias", ""},
		{http.MethodPut, "/notificacoes/preferencias", `{"modulo":"atlas","ativo":true}`},
	} {
		if rec := do(fora, tc.method, tc.path, tc.body); rec.Code != http.StatusInternalServerError {
			t.Errorf("banco fora %s %s: %d", tc.method, tc.path, rec.Code)
		}
	}
	if (ErrInvalida{"motivo"}).Error() != "motivo" {
		t.Fatal("mensagem do erro")
	}
	if err := MapError(ErrInvalida{"x"}); err == nil {
		t.Fatal("inválida mapeada")
	}
}
