package application_test

import (
	"context"
	"encoding/json"
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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/domain/events"
	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/tramite"
	"github.com/yurythx/projeto-nexus/internal/modules/tramite/application"
	"github.com/yurythx/projeto-nexus/internal/modules/tramite/domain"
	"github.com/yurythx/projeto-nexus/internal/modules/tramite/infrastructure"
	"github.com/yurythx/projeto-nexus/internal/modules/tramite/transport"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/config"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
	"github.com/yurythx/projeto-nexus/internal/platform/modkit"
	"github.com/yurythx/projeto-nexus/internal/platform/outbox"
	"github.com/yurythx/projeto-nexus/internal/platform/storage/storagetest"
)

var errBoom = errors.New("falha simulada no repositório")

// faultRepo envolve o repositório real e, na chamada de número failAt,
// devolve erro; depois da chamada poisonAt, "envenena" a transação (a
// próxima instrução SQL — do repositório, do outbox ou da auditoria — falha).
type faultRepo struct {
	domain.Repository
	calls, failAt, poisonAt int
	trace                   []string
	// noTx: o veneno caiu numa leitura fora de transação, onde não há
	// "próxima instrução" na mesma conexão para falhar.
	noTx bool
}

func (f *faultRepo) hook(ctx context.Context, db database.DBTX, name string) error {
	f.calls++
	f.trace = append(f.trace, name)
	if f.calls == f.failAt {
		return errBoom
	}
	return nil
}

// post roda depois da chamada real: envenena a transação para que a
// PRÓXIMA instrução (repositório, outbox ou auditoria) falhe.
func (f *faultRepo) post(ctx context.Context, db database.DBTX) {
	if f.calls != f.poisonAt {
		return
	}
	if _, inTx := db.(pgx.Tx); !inTx {
		f.noTx = true
		return
	}
	_, _ = db.Exec(ctx, `SELECT 1/0`)
}

func (f *faultRepo) Tipos(ctx context.Context, db database.DBTX) ([]domain.Tipo, error) {
	if err := f.hook(ctx, db, "Tipos"); err != nil {
		return nil, err
	}
	defer f.post(ctx, db)
	return f.Repository.Tipos(ctx, db)
}
func (f *faultRepo) TipoAtivo(ctx context.Context, db database.DBTX, id uuid.UUID) (bool, error) {
	if err := f.hook(ctx, db, "TipoAtivo"); err != nil {
		return false, err
	}
	defer f.post(ctx, db)
	return f.Repository.TipoAtivo(ctx, db, id)
}
func (f *faultRepo) UnidadeAtiva(ctx context.Context, db database.DBTX, id uuid.UUID) (bool, error) {
	if err := f.hook(ctx, db, "UnidadeAtiva"); err != nil {
		return false, err
	}
	defer f.post(ctx, db)
	return f.Repository.UnidadeAtiva(ctx, db, id)
}
func (f *faultRepo) NextNumero(ctx context.Context, db database.DBTX, ano int) (int, error) {
	if err := f.hook(ctx, db, "NextNumero"); err != nil {
		return 0, err
	}
	defer f.post(ctx, db)
	return f.Repository.NextNumero(ctx, db, ano)
}
func (f *faultRepo) Insert(ctx context.Context, db database.DBTX, p domain.Processo) error {
	if err := f.hook(ctx, db, "Insert"); err != nil {
		return err
	}
	defer f.post(ctx, db)
	return f.Repository.Insert(ctx, db, p)
}
func (f *faultRepo) Get(ctx context.Context, db database.DBTX, id uuid.UUID, forUpdate bool) (domain.Processo, error) {
	if err := f.hook(ctx, db, "Get"); err != nil {
		return domain.Processo{}, err
	}
	defer f.post(ctx, db)
	return f.Repository.Get(ctx, db, id, forUpdate)
}
func (f *faultRepo) HasGrant(ctx context.Context, db database.DBTX, p, u uuid.UUID) (bool, error) {
	if err := f.hook(ctx, db, "HasGrant"); err != nil {
		return false, err
	}
	defer f.post(ctx, db)
	return f.Repository.HasGrant(ctx, db, p, u)
}
func (f *faultRepo) ListVisible(ctx context.Context, db database.DBTX, i auth.Identity, fl domain.Filter, p pagination.Params) ([]domain.Processo, int64, error) {
	if err := f.hook(ctx, db, "ListVisible"); err != nil {
		return nil, 0, err
	}
	defer f.post(ctx, db)
	return f.Repository.ListVisible(ctx, db, i, fl, p)
}
func (f *faultRepo) Update(ctx context.Context, db database.DBTX, p domain.Processo) error {
	if err := f.hook(ctx, db, "Update"); err != nil {
		return err
	}
	defer f.post(ctx, db)
	return f.Repository.Update(ctx, db, p)
}
func (f *faultRepo) Grant(ctx context.Context, db database.DBTX, p, u, by uuid.UUID) error {
	if err := f.hook(ctx, db, "Grant"); err != nil {
		return err
	}
	defer f.post(ctx, db)
	return f.Repository.Grant(ctx, db, p, u, by)
}
func (f *faultRepo) Revoke(ctx context.Context, db database.DBTX, p, u uuid.UUID) (bool, error) {
	if err := f.hook(ctx, db, "Revoke"); err != nil {
		return false, err
	}
	defer f.post(ctx, db)
	return f.Repository.Revoke(ctx, db, p, u)
}
func (f *faultRepo) Grants(ctx context.Context, db database.DBTX, p uuid.UUID) ([]domain.Grant, error) {
	if err := f.hook(ctx, db, "Grants"); err != nil {
		return nil, err
	}
	defer f.post(ctx, db)
	return f.Repository.Grants(ctx, db, p)
}
func (f *faultRepo) Documentos(ctx context.Context, db database.DBTX, p uuid.UUID) ([]domain.Documento, error) {
	if err := f.hook(ctx, db, "Documentos"); err != nil {
		return nil, err
	}
	defer f.post(ctx, db)
	return f.Repository.Documentos(ctx, db, p)
}
func (f *faultRepo) GetDocumento(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Documento, error) {
	if err := f.hook(ctx, db, "GetDocumento"); err != nil {
		return domain.Documento{}, err
	}
	defer f.post(ctx, db)
	return f.Repository.GetDocumento(ctx, db, id)
}
func (f *faultRepo) InsertDocumento(ctx context.Context, db database.DBTX, d domain.Documento) error {
	if err := f.hook(ctx, db, "InsertDocumento"); err != nil {
		return err
	}
	defer f.post(ctx, db)
	return f.Repository.InsertDocumento(ctx, db, d)
}
func (f *faultRepo) UpdateDocumento(ctx context.Context, db database.DBTX, d domain.Documento) error {
	if err := f.hook(ctx, db, "UpdateDocumento"); err != nil {
		return err
	}
	defer f.post(ctx, db)
	return f.Repository.UpdateDocumento(ctx, db, d)
}
func (f *faultRepo) DocumentoByEnvelope(ctx context.Context, db database.DBTX, id uuid.UUID) (domain.Documento, error) {
	if err := f.hook(ctx, db, "DocumentoByEnvelope"); err != nil {
		return domain.Documento{}, err
	}
	defer f.post(ctx, db)
	return f.Repository.DocumentoByEnvelope(ctx, db, id)
}
func (f *faultRepo) PendingSignatures(ctx context.Context, db database.DBTX, p uuid.UUID) (int, error) {
	if err := f.hook(ctx, db, "PendingSignatures"); err != nil {
		return 0, err
	}
	defer f.post(ctx, db)
	return f.Repository.PendingSignatures(ctx, db, p)
}
func (f *faultRepo) AddMovimento(ctx context.Context, db database.DBTX, m domain.Movimento) error {
	if err := f.hook(ctx, db, "AddMovimento"); err != nil {
		return err
	}
	defer f.post(ctx, db)
	return f.Repository.AddMovimento(ctx, db, m)
}
func (f *faultRepo) Movimentos(ctx context.Context, db database.DBTX, p uuid.UUID) ([]domain.Movimento, error) {
	if err := f.hook(ctx, db, "Movimentos"); err != nil {
		return nil, err
	}
	defer f.post(ctx, db)
	return f.Repository.Movimentos(ctx, db, p)
}

func (f *faultRepo) ProcessosDoProcedimento(ctx context.Context, db database.DBTX, id uuid.UUID) ([]domain.Processo, error) {
	if err := f.hook(ctx, db, "ProcessosDoProcedimento"); err != nil {
		return nil, err
	}
	defer f.post(ctx, db)
	return f.Repository.ProcessosDoProcedimento(ctx, db, id)
}

// fakeSign simula o Signum.
type fakeSign struct {
	available bool
	err       error
}

func (f fakeSign) Available() bool { return f.available }
func (f fakeSign) OpenEnvelope(context.Context, pgx.Tx, domain.SignatureRequest) (uuid.UUID, error) {
	return uuid.New(), f.err
}

const bucket = "nexus-test"

type env struct {
	t             *testing.T
	pool          *pgxpool.Pool
	store         *storagetest.Memory
	author        auth.Identity
	unidade, tipo uuid.UUID
}

func newEnv(t *testing.T) *env {
	t.Helper()
	pool := dbtest.Pool(t)
	un := dbtest.Unidade(t, pool)
	var tipo uuid.UUID
	if err := pool.QueryRow(context.Background(), `SELECT id FROM tramite_tipos WHERE ativo LIMIT 1`).Scan(&tipo); err != nil {
		t.Fatal(err)
	}
	author := auth.Identity{UserID: dbtest.User(t, pool), Permissions: []string{string(auth.PermTramiteRoute)},
		Scopes: []auth.Scope{{Perfil: "protocolo", UnidadeID: &un, Permissions: []string{string(auth.PermTramiteCreate), string(auth.PermTramiteRoute)}}}}
	return &env{t: t, pool: pool, store: storagetest.New(), author: author, unidade: un, tipo: tipo}
}

func (e *env) svc(repo domain.Repository, sign domain.SignaturePort) *application.Service {
	return application.NewService(e.pool, repo, sign, outbox.NewWriter("test"), e.store, bucket, 1<<20, time.Minute,
		slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func (e *env) real() *application.Service {
	return e.svc(infrastructure.NewRepository(), fakeSign{available: true})
}

func (e *env) processo(sigilo string) domain.Processo {
	e.t.Helper()
	p, err := e.real().Abrir(context.Background(), e.author, application.AbrirInput{TipoID: e.tipo, Assunto: "Processo de teste",
		Sigilo: sigilo, UnidadeOrigemID: e.unidade})
	if err != nil {
		e.t.Fatal(err)
	}
	return p
}

func (e *env) documento(procID uuid.UUID) domain.Documento {
	e.t.Helper()
	d, err := e.real().AdicionarDocumento(context.Background(), e.author, procID, application.NovoDocumentoInput{Titulo: "Despacho", Conteudo: "Texto"})
	if err != nil {
		e.t.Fatal(err)
	}
	return d
}

func (e *env) anexo(procID uuid.UUID) string {
	e.t.Helper()
	key := "tramite/" + procID.String() + "/" + uuid.NewString() + ".pdf"
	if err := e.store.Put(context.Background(), bucket, key, strings.NewReader("%PDF-1.4"), 8, "application/pdf"); err != nil {
		e.t.Fatal(err)
	}
	return key
}

// Cada chamada ao repositório de cada caso de uso é, uma de cada vez,
// derrubada (erro) e envenenada (a instrução seguinte falha). O caso de
// uso tem de devolver erro — nunca sucesso parcial.
func TestEveryRepositoryFailureIsPropagated(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	outro := dbtest.User(t, e.pool)

	type op struct {
		name  string
		setup func() func(s *application.Service) error
	}
	ops := []op{
		{"Tipos", func() func(*application.Service) error {
			return func(s *application.Service) error { _, err := s.Tipos(ctx); return err }
		}},
		{"Abrir", func() func(*application.Service) error {
			return func(s *application.Service) error {
				_, err := s.Abrir(ctx, e.author, application.AbrirInput{TipoID: e.tipo, Assunto: "x", Sigilo: "publico", UnidadeOrigemID: e.unidade})
				return err
			}
		}},
		{"List", func() func(*application.Service) error {
			return func(s *application.Service) error {
				_, _, err := s.List(ctx, e.author, domain.Filter{}, pagination.New(1, 5, 5))
				return err
			}
		}},
		{"Search", func() func(*application.Service) error {
			return func(s *application.Service) error { _, err := s.Search(ctx, e.author, "x", 5); return err }
		}},
		{"Get", func() func(*application.Service) error {
			p := e.processo(domain.SigiloRestrito)
			return func(s *application.Service) error { _, err := s.Get(ctx, e.author, p.ID); return err }
		}},
		{"Tramitar", func() func(*application.Service) error {
			p := e.processo(domain.SigiloPublico)
			destino := dbtest.Unidade(t, e.pool)
			return func(s *application.Service) error {
				_, err := s.Tramitar(ctx, e.author, p.ID, destino, "Encaminho")
				return err
			}
		}},
		{"Concluir", func() func(*application.Service) error {
			p := e.processo(domain.SigiloPublico)
			return func(s *application.Service) error { _, err := s.Concluir(ctx, e.author, p.ID, "Fim"); return err }
		}},
		{"Arquivar", func() func(*application.Service) error {
			p := e.processo(domain.SigiloPublico)
			if _, err := e.real().Concluir(ctx, e.author, p.ID, "Fim"); err != nil {
				t.Fatal(err)
			}
			return func(s *application.Service) error { _, err := s.Arquivar(ctx, e.author, p.ID, "Arquivo"); return err }
		}},
		{"ConcederAcesso", func() func(*application.Service) error {
			p := e.processo(domain.SigiloSigiloso)
			return func(s *application.Service) error { return s.ConcederAcesso(ctx, e.author, p.ID, outro) }
		}},
		{"RevogarAcesso", func() func(*application.Service) error {
			p := e.processo(domain.SigiloSigiloso)
			if err := e.real().ConcederAcesso(ctx, e.author, p.ID, outro); err != nil {
				t.Fatal(err)
			}
			return func(s *application.Service) error { return s.RevogarAcesso(ctx, e.author, p.ID, outro) }
		}},
		{"UploadAnexo", func() func(*application.Service) error {
			p := e.processo(domain.SigiloPublico)
			return func(s *application.Service) error {
				_, err := s.UploadAnexo(ctx, e.author, p.ID, "a.pdf", "application/pdf")
				return err
			}
		}},
		{"AdicionarDocumento", func() func(*application.Service) error {
			p := e.processo(domain.SigiloPublico)
			key := e.anexo(p.ID)
			return func(s *application.Service) error {
				_, err := s.AdicionarDocumento(ctx, e.author, p.ID, application.NovoDocumentoInput{Titulo: "Anexo", ObjectKey: key})
				return err
			}
		}},
		{"EditarDocumento", func() func(*application.Service) error {
			d := e.documento(e.processo(domain.SigiloPublico).ID)
			return func(s *application.Service) error {
				_, err := s.EditarDocumento(ctx, e.author, d.ID, "Novo", "Texto")
				return err
			}
		}},
		{"GetDocumento", func() func(*application.Service) error {
			p := e.processo(domain.SigiloPublico)
			d, err := e.real().AdicionarDocumento(ctx, e.author, p.ID, application.NovoDocumentoInput{Titulo: "Anexo", ObjectKey: e.anexo(p.ID)})
			if err != nil {
				t.Fatal(err)
			}
			return func(s *application.Service) error { _, err := s.GetDocumento(ctx, e.author, d.ID); return err }
		}},
		{"SolicitarAssinatura", func() func(*application.Service) error {
			d := e.documento(e.processo(domain.SigiloPublico).ID)
			return func(s *application.Service) error {
				_, err := s.SolicitarAssinatura(ctx, e.author, d.ID, []uuid.UUID{outro}, false)
				return err
			}
		}},
		{"HandleSignatureEvent", func() func(*application.Service) error {
			d := e.documento(e.processo(domain.SigiloPublico).ID)
			d, err := e.real().SolicitarAssinatura(ctx, e.author, d.ID, []uuid.UUID{outro}, false)
			if err != nil {
				t.Fatal(err)
			}
			ev := signumEvent(t, "signum.envelope.completed", *d.EnvelopeID, "tramite")
			return func(s *application.Service) error { return s.HandleSignatureEvent(ctx, ev) }
		}},
	}

	for _, o := range ops {
		// Roda sem falha para descobrir a sequência de chamadas.
		probe := &faultRepo{Repository: infrastructure.NewRepository()}
		if err := o.setup()(e.svc(probe, fakeSign{available: true})); err != nil {
			t.Fatalf("%s sem falha: %v", o.name, err)
		}
		for i := range probe.trace {
			for _, mode := range []string{"erro", "veneno"} {
				r := &faultRepo{Repository: infrastructure.NewRepository()}
				if mode == "erro" {
					r.failAt = i + 1
				} else {
					r.poisonAt = i + 1
				}
				err := o.setup()(e.svc(r, fakeSign{available: true}))
				// Veneno na última instrução da transação: o commit falha.
				if err == nil && !r.noTx {
					t.Errorf("%s: %s na chamada %d (%s) foi engolido", o.name, mode, i+1, probe.trace[i])
				}
			}
		}
	}
}

func signumEvent(t *testing.T, typ string, envelope uuid.UUID, source string) events.Event {
	t.Helper()
	ev, err := events.New(typ, "signum", uuid.Nil, map[string]any{"envelope_id": envelope, "status": "x", "source_module": source})
	if err != nil {
		t.Fatal(err)
	}
	return ev
}

// Regras que não passam pelo repositório: disponibilidade do Signum,
// falhas do armazenamento e eventos que o Trâmite deve ignorar.
func TestSignatureStorageAndEventEdgeCases(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	outro := dbtest.User(t, e.pool)
	repo := infrastructure.NewRepository()

	// Signum desligado (ou ausente) e envelope recusado pelo Signum.
	d := e.documento(e.processo(domain.SigiloPublico).ID)
	for _, sign := range []domain.SignaturePort{nil, fakeSign{available: false}} {
		_, err := e.svc(repo, sign).SolicitarAssinatura(ctx, e.author, d.ID, []uuid.UUID{outro}, false)
		if status(err) != http.StatusServiceUnavailable {
			t.Errorf("Signum indisponível: esperado 503, veio %v", err)
		}
	}
	signErr := apperrors.Validation("signatário inválido")
	if _, err := e.svc(repo, fakeSign{available: true, err: signErr}).SolicitarAssinatura(ctx, e.author, d.ID, []uuid.UUID{outro}, false); !errors.Is(err, signErr) {
		t.Errorf("erro do Signum chega ao chamador: %v", err)
	}
	if got, _ := e.real().GetDocumento(ctx, e.author, d.ID); got.Status != "rascunho" {
		t.Errorf("falha ao abrir o envelope não altera o documento: %+v", got)
	}

	// Armazenamento: indisponível, leitura interrompida e URL não emitida.
	p := e.processo(domain.SigiloPublico)
	key := e.anexo(p.ID)
	for name, set := range map[string]func(bool){
		"stat": func(v bool) { e.store.FailStat = v },
		"get":  func(v bool) { e.store.FailGet = v },
		"read": func(v bool) { e.store.FailRead = v },
	} {
		set(true)
		_, err := e.real().AdicionarDocumento(ctx, e.author, p.ID, application.NovoDocumentoInput{Titulo: "Anexo", ObjectKey: key})
		set(false)
		if status(err) != http.StatusServiceUnavailable {
			t.Errorf("armazenamento com falha em %s: esperado 503, veio %v", name, err)
		}
	}
	anexo, err := e.real().AdicionarDocumento(ctx, e.author, p.ID, application.NovoDocumentoInput{Titulo: "Anexo", ObjectKey: key})
	if err != nil {
		t.Fatal(err)
	}
	e.store.FailPresign = true
	if _, err := e.real().GetDocumento(ctx, e.author, anexo.ID); status(err) != http.StatusServiceUnavailable {
		t.Errorf("sem URL temporária, a leitura do anexo falha com erro claro: %v", err)
	}
	e.store.FailPresign = false

	// Eventos que não são do Trâmite, inválidos ou já tratados são ignorados.
	s := e.real()
	ignored := []events.Event{
		{Type: "signum.envelope.completed", Payload: json.RawMessage(`{`)},
		signumEvent(t, "signum.envelope.completed", uuid.New(), "outro-modulo"),
		signumEvent(t, "signum.envelope.completed", uuid.New(), "tramite"), // envelope desconhecido
	}
	for _, ev := range ignored {
		if err := s.HandleSignatureEvent(ctx, ev); err != nil {
			t.Errorf("evento %s deveria ser ignorado: %v", ev.Payload, err)
		}
	}
	doc := e.documento(p.ID)
	doc, err = s.SolicitarAssinatura(ctx, e.author, doc.ID, []uuid.UUID{outro}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.HandleSignatureEvent(ctx, signumEvent(t, "signum.envelope.viewed", *doc.EnvelopeID, "tramite")); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.GetDocumento(ctx, e.author, doc.ID); got.Status != "aguardando_assinatura" {
		t.Errorf("evento desconhecido não muda o documento: %+v", got)
	}
	if err := s.HandleSignatureEvent(ctx, signumEvent(t, "signum.envelope.cancelled", *doc.EnvelopeID, "tramite")); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.GetDocumento(ctx, e.author, doc.ID); got.Status != "rascunho" || got.EnvelopeID != nil {
		t.Errorf("envelope cancelado devolve o documento a rascunho: %+v", got)
	}
	if err := s.HandleSignatureEvent(ctx, signumEvent(t, "signum.envelope.completed", *doc.EnvelopeID, "tramite")); err != nil {
		t.Errorf("evento de envelope já encerrado é ignorado: %v", err)
	}

	// Processo com sigilo: o erro de "sem acesso" nunca revela existência.
	if err := application.MapError(domain.ErrForbidden); status(err) != http.StatusForbidden {
		t.Errorf("ErrForbidden vira 403: %v", err)
	}
	if err := application.MapError(errBoom); !errors.Is(err, errBoom) {
		t.Errorf("erro desconhecido passa adiante sem tradução: %v", err)
	}
}

// Os handlers traduzem falhas do serviço em resposta de erro (sem vazar
// detalhes internos).
func TestHandlersReportServiceFailures(t *testing.T) {
	e := newEnv(t)
	down := &faultRepo{Repository: infrastructure.NewRepository(), failAt: 1}
	h := transport.NewHandlers(e.svc(down, fakeSign{available: true}), slog.New(slog.NewTextHandler(io.Discard, nil)), 100)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.WithIdentity(req.Context(), e.author)))
		})
	})
	h.RegisterRoutes(r)
	for _, path := range []string{"/tramite/tipos", "/tramite/processos"} {
		down.calls = 0
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), errBoom.Error()) {
			t.Errorf("GET %s com o banco fora: %d %s", path, rec.Code, rec.Body.String())
		}
	}
}

// status devolve o HTTP status do erro de aplicação (0 se não for um).
func status(err error) int {
	if appErr, ok := apperrors.As(err); ok {
		return appErr.Status
	}
	return 0
}

// A busca global propaga a falha do banco (o agregador decide o que fazer).
func TestSearchProviderPropagatesFailure(t *testing.T) {
	e := newEnv(t)
	m := tramite.New(modkit.Deps{Pool: e.pool, Outbox: outbox.NewWriter("test"), Storage: e.store, Config: &config.Config{},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}, fakeSign{available: true})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := m.SearchProviders()[0].Search(ctx, e.author, "x", 5); err == nil {
		t.Fatal("busca com o banco indisponível deveria falhar")
	}
	p := e.processo(domain.SigiloSigiloso)
	res, err := m.SearchProviders()[0].Search(context.Background(), e.author, p.Numero, 5)
	if err != nil || len(res) != 1 || strings.Contains(res[0].Title, p.Assunto) {
		t.Fatalf("sigiloso aparece na busca de quem pode ler, sem o assunto: %+v %v", res, err)
	}
	if m.SearchProviders()[0].Module() != tramite.Key || len(m.Consumers()) != 2 || m.Manifest().Key != tramite.Key {
		t.Fatal("manifesto, consumidor e provedor de busca registrados")
	}
}
