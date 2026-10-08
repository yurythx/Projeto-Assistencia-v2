package application_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/yurythx/projeto-assistencia-v2/internal/domain/errors"
	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/application"
	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/domain"
	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/infrastructure"
	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/transport"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/auth"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/database/dbtest"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/outbox"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/ws"
)

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

type publisher struct{ err error }

func (p publisher) Publish(context.Context, string, string, any) error { return p.err }

// cenario: uma unidade e um profissional com todas as permissões nela.
type cenario struct {
	pool     *pgxpool.Pool
	unidade  uuid.UUID
	identity auth.Identity
}

func novoCenario(t *testing.T) cenario {
	t.Helper()
	pool := dbtest.Pool(t)
	ctx := context.Background()
	sfx := uuid.NewString()[:8]
	var ent, un uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO entidades (nome, slug) VALUES ($1, $1) RETURNING id`, "ent-"+sfx).Scan(&ent); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO unidades (entidade_id, nome, slug) VALUES ($1, $2, $2) RETURNING id`, ent, "un-"+sfx).Scan(&un); err != nil {
		t.Fatal(err)
	}
	identity := auth.Identity{UserID: dbtest.User(t, pool), Name: "Profissional",
		Scopes: []auth.Scope{{EntidadeID: &ent, UnidadeID: &un, Permissions: []string{"sempras:*"}}}}
	identity.Permissions = auth.EffectivePermissions(identity.Scopes)
	return cenario{pool: pool, unidade: un, identity: identity}
}

func (c cenario) svc(repo domain.Repository, pub application.Publisher) *application.Service {
	return application.NewService(c.pool, repo, outbox.NewWriter("nexus.sempras"), pub, testLogger()).WithClock(time.Now)
}

func (c cenario) limpo() *application.Service {
	return c.svc(infrastructure.NewRepository(), publisher{})
}

func (c cenario) cadastrar(t *testing.T) domain.Cidadao {
	t.Helper()
	cid, err := c.limpo().Cadastrar(context.Background(), c.identity, domain.CidadaoInput{
		Nome: "Pessoa " + uuid.NewString()[:6], DataNascimento: "1990-01-01", NomeMae: "Mãe"})
	if err != nil {
		t.Fatal(err)
	}
	return cid
}

func (c cenario) incluir(t *testing.T, tipo string) domain.FilaItem {
	t.Helper()
	item, err := c.limpo().Incluir(context.Background(), c.identity, c.unidade,
		application.EntradaInput{CidadaoID: c.cadastrar(t).ID, Tipo: tipo})
	if err != nil {
		t.Fatal(err)
	}
	return item
}

// Matriz de falhas: cada chamada ao repositório falha (ou envenena a
// transação logo depois) e o caso de uso propaga o erro — nunca devolve
// sucesso com dado gravado sem evento/auditoria, nem ficha sem auditoria.
func TestEveryRepositoryFailureIsPropagated(t *testing.T) {
	c := novoCenario(t)
	ctx := context.Background()
	id := c.identity

	// Prontuário com dois registros psicológicos (o segundo usa o cache da unidade).
	ficha := c.incluir(t, domain.TipoPsicologico)
	for range 2 {
		if _, err := c.limpo().RegistrarEvolucao(ctx, id, ficha.CidadaoID, application.EvolucaoInput{
			UnidadeID: c.unidade, Tipo: domain.TipoPsicologico, Texto: "sigiloso"}); err != nil {
			t.Fatal(err)
		}
	}
	ops := map[string]func(t *testing.T, s *application.Service) error{
		"Unidades": func(_ *testing.T, s *application.Service) error { _, err := s.Unidades(ctx, id); return err },
		"Buscar documento": func(_ *testing.T, s *application.Service) error {
			_, err := s.Buscar(ctx, id, "00000000000")
			return err
		},
		"Buscar nome": func(_ *testing.T, s *application.Service) error {
			_, err := s.Buscar(ctx, id, "Pessoa")
			return err
		},
		"Cadastrar": func(_ *testing.T, s *application.Service) error {
			_, err := s.Cadastrar(ctx, id, domain.CidadaoInput{Nome: "Nova", DataNascimento: "2000-01-01", NomeMae: "M"})
			return err
		},
		"Atualizar": func(_ *testing.T, s *application.Service) error {
			_, err := s.Atualizar(ctx, id, ficha.CidadaoID, domain.CidadaoInput{Nome: "Editada", DataNascimento: "2000-01-01", NomeMae: "M"})
			return err
		},
		"Cidadao": func(_ *testing.T, s *application.Service) error {
			_, err := s.Cidadao(ctx, ficha.CidadaoID)
			return err
		},
		"Prontuario": func(_ *testing.T, s *application.Service) error {
			_, err := s.Prontuario(ctx, id, ficha.CidadaoID)
			return err
		},
		"RegistrarEvolucao": func(_ *testing.T, s *application.Service) error {
			_, err := s.RegistrarEvolucao(ctx, id, ficha.CidadaoID, application.EvolucaoInput{
				UnidadeID: c.unidade, AtendimentoID: &ficha.ID, Tipo: domain.TipoSocial, Texto: "evolução"})
			return err
		},
		"Incluir": func(t *testing.T, s *application.Service) error {
			_, err := s.Incluir(ctx, id, c.unidade, application.EntradaInput{CidadaoID: c.cadastrar(t).ID, Tipo: domain.TipoSocial})
			return err
		},
		"Fila": func(_ *testing.T, s *application.Service) error { _, err := s.Fila(ctx, id, c.unidade); return err },
		"ChamarProximo": func(t *testing.T, s *application.Service) error {
			c.incluir(t, domain.TipoSocial)
			_, err := s.ChamarProximo(ctx, id, c.unidade, "Sala 1")
			return err
		},
		"Agir": func(t *testing.T, s *application.Service) error {
			_, err := s.Agir(ctx, id, c.incluir(t, domain.TipoSocial).ID, "chamar", "")
			return err
		},
		"Painel": func(_ *testing.T, s *application.Service) error { _, err := s.Painel(ctx, id); return err },
		"Authorize": func(_ *testing.T, s *application.Service) error {
			client := ws.ClientInfo{Scopes: id.Scopes}
			return s.Authorize(ctx, client, application.TopicFila(c.unidade))
		},
	}
	for name, op := range ops {
		probe := &faultRepo{inner: infrastructure.NewRepository()}
		if err := op(t, c.svc(probe, publisher{})); err != nil {
			t.Fatalf("%s sem falha: %v", name, err)
		}
		for i := range probe.trace {
			for _, poison := range []bool{false, true} {
				r := &faultRepo{inner: infrastructure.NewRepository()}
				if poison {
					r.poisonAt = i + 1
				} else {
					r.failAt = i + 1
				}
				if err := op(t, c.svc(r, publisher{})); err == nil && !r.noTx {
					t.Errorf("%s: falha (veneno=%v) na chamada %d (%s) foi engolida", name, poison, i+1, probe.trace[i])
				}
			}
		}
	}
}

// O tempo real é de melhor esforço: a falha ao publicar não desfaz nada.
func TestPublishFailureDoesNotFailTheUseCase(t *testing.T) {
	c := novoCenario(t)
	s := c.svc(infrastructure.NewRepository(), publisher{err: errors.New("hub fora")})
	if _, err := s.Incluir(context.Background(), c.identity, c.unidade,
		application.EntradaInput{CidadaoID: c.cadastrar(t).ID, Tipo: domain.TipoSocial}); err != nil {
		t.Fatal(err)
	}
}

func TestMapError(t *testing.T) {
	for err, code := range map[error]int{
		&domain.ValidationError{Msg: "x"}: http.StatusUnprocessableEntity,
		domain.ErrNotFound:                http.StatusNotFound,
		domain.ErrForbidden:               http.StatusForbidden,
		domain.ErrInvalidState:            http.StatusConflict,
		domain.ErrFilaVazia:               http.StatusConflict,
		domain.ErrJaNaFila:                http.StatusConflict,
		domain.ErrDocumentoDuplicado:      http.StatusConflict,
		domain.ErrUnidadeInativa:          http.StatusUnprocessableEntity,
		domain.ErrAtendimentoOutro:        http.StatusUnprocessableEntity,
	} {
		ae, ok := apperrors.As(application.MapError(err))
		if !ok || ae.Status != code {
			t.Errorf("%v -> %+v", err, ae)
		}
	}
	if err := application.MapError(errBoom); !errors.Is(err, errBoom) {
		t.Error("erro desconhecido passa adiante")
	}
}

// Regras que não dependem de falha do banco.
func TestRegrasDoServico(t *testing.T) {
	c := novoCenario(t)
	ctx := context.Background()
	s := c.limpo()
	if _, err := s.Agir(ctx, c.identity, uuid.New(), "teletransportar", ""); !errors.Is(err, domain.ErrInvalidState) {
		t.Errorf("ação desconhecida: %v", err)
	}
	semPsico := auth.Identity{UserID: c.identity.UserID, Scopes: []auth.Scope{{EntidadeID: c.identity.Scopes[0].EntidadeID,
		UnidadeID: &c.unidade, Permissions: []string{"sempras:recepcao", "sempras:atendimento"}}}}
	if _, err := s.RegistrarEvolucao(ctx, semPsico, c.cadastrar(t).ID, application.EvolucaoInput{
		UnidadeID: c.unidade, Tipo: domain.TipoPsicologico, Texto: "x"}); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("psicológico sem sigilo: %v", err)
	}
	if _, err := s.RegistrarEvolucao(ctx, semPsico, uuid.New(), application.EvolucaoInput{
		UnidadeID: c.unidade, Tipo: "?", Texto: "x"}); err == nil {
		t.Error("registro inválido")
	}
	if _, err := s.Incluir(ctx, semPsico, c.unidade, application.EntradaInput{Tipo: "?"}); err == nil {
		t.Error("entrada inválida")
	}
	if _, err := s.Cadastrar(ctx, semPsico, domain.CidadaoInput{Nome: "x"}); err == nil {
		t.Error("cadastro inválido")
	}
	if _, err := s.Atualizar(ctx, semPsico, uuid.New(), domain.CidadaoInput{Nome: "x"}); err == nil {
		t.Error("edição inválida")
	}
	if _, err := s.Prontuario(ctx, auth.Identity{}, uuid.New()); !errors.Is(err, domain.ErrForbidden) {
		t.Error("ficha sem área")
	}
	if _, err := s.Painel(ctx, semPsico); !errors.Is(err, domain.ErrForbidden) {
		t.Error("painel sem gestão")
	}
	// Só a recepção na unidade: vê a fila, mas não chama.
	recepcao := auth.Identity{UserID: c.identity.UserID, Scopes: []auth.Scope{{UnidadeID: &c.unidade, Permissions: []string{"sempras:recepcao"}}}}
	if _, err := s.ChamarProximo(ctx, recepcao, c.unidade, ""); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("recepção não chama: %v", err)
	}
	if _, err := s.Fila(ctx, recepcao, c.unidade); err != nil {
		t.Errorf("recepção vê a fila: %v", err)
	}
	if err := s.Inbound(ctx, ws.ClientInfo{}, ws.Frame{}); err == nil {
		t.Error("frames do navegador são recusados")
	}
}

// Os handlers devolvem 4xx em entrada ruim e 500 (sem vazar o erro) quando
// o serviço falha.
func TestHandlersReportFailures(t *testing.T) {
	c := novoCenario(t)
	down := &faultRepo{inner: infrastructure.NewRepository(), failAt: 1}
	h := transport.NewHandlers(c.svc(down, publisher{}), testLogger())
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.WithIdentity(req.Context(), c.identity)))
		})
	})
	h.RegisterRoutes(r)
	u, x := c.unidade.String(), uuid.NewString()
	for _, tc := range []struct {
		method, path, body string
		want               int
	}{
		{http.MethodGet, "/sempras/unidades", "", 500},
		{http.MethodGet, "/sempras/painel", "", 500},
		{http.MethodGet, "/sempras/cidadaos?q=Pessoa", "", 500},
		{http.MethodPost, "/sempras/cidadaos", `{"nome":"Nova","data_nascimento":"2000-01-01","nome_mae":"M"}`, 500},
		{http.MethodPost, "/sempras/cidadaos", `{`, 400},
		{http.MethodGet, "/sempras/cidadaos/" + x, "", 500},
		{http.MethodGet, "/sempras/cidadaos/zz", "", 400},
		{http.MethodPut, "/sempras/cidadaos/" + x, `{"nome":"Nova","data_nascimento":"2000-01-01","nome_mae":"M"}`, 500},
		{http.MethodPut, "/sempras/cidadaos/zz", `{}`, 400},
		{http.MethodPut, "/sempras/cidadaos/" + x, `{`, 400},
		{http.MethodGet, "/sempras/cidadaos/" + x + "/prontuario", "", 500},
		{http.MethodGet, "/sempras/cidadaos/zz/prontuario", "", 400},
		{http.MethodPost, "/sempras/cidadaos/" + x + "/registros", `{"unidade_id":"` + u + `","tipo":"social","texto":"t"}`, 500},
		{http.MethodPost, "/sempras/cidadaos/zz/registros", `{}`, 400},
		{http.MethodPost, "/sempras/cidadaos/" + x + "/registros", `{`, 400},
		{http.MethodGet, "/sempras/unidades/" + u + "/fila", "", 500},
		{http.MethodGet, "/sempras/unidades/zz/fila", "", 400},
		{http.MethodPost, "/sempras/unidades/" + u + "/fila", `{"cidadao_id":"` + x + `","tipo":"social"}`, 500},
		{http.MethodPost, "/sempras/unidades/zz/fila", `{}`, 400},
		{http.MethodPost, "/sempras/unidades/" + u + "/fila", `{`, 400},
		{http.MethodPost, "/sempras/unidades/" + u + "/fila/proximo", "", 500},
		{http.MethodPost, "/sempras/unidades/zz/fila/proximo", "", 400},
		{http.MethodPost, "/sempras/unidades/" + u + "/fila/proximo", `{`, 400},
		{http.MethodPost, "/sempras/atendimentos/" + x + "/iniciar", "", 500},
	} {
		down.calls = 0
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(rec, req)
		if rec.Code != tc.want || strings.Contains(rec.Body.String(), errBoom.Error()) {
			t.Errorf("%s %s: esperado %d, veio %d %s", tc.method, tc.path, tc.want, rec.Code, rec.Body.String())
		}
	}
}
