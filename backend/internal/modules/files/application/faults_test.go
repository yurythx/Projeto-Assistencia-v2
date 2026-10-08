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

	"github.com/yurythx/projeto-nexus/internal/modules/files"
	"github.com/yurythx/projeto-nexus/internal/modules/files/application"
	"github.com/yurythx/projeto-nexus/internal/modules/files/domain"
	"github.com/yurythx/projeto-nexus/internal/modules/files/infrastructure"
	"github.com/yurythx/projeto-nexus/internal/modules/files/transport"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/config"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
	"github.com/yurythx/projeto-nexus/internal/platform/modkit"
	"github.com/yurythx/projeto-nexus/internal/platform/outbox"
	"github.com/yurythx/projeto-nexus/internal/platform/storage/storagetest"
)

const bucket = "nexus-test"

type env struct {
	t     *testing.T
	pool  *pgxpool.Pool
	store *storagetest.Memory
	owner auth.Identity
}

func newEnv(t *testing.T) *env {
	pool := dbtest.Pool(t)
	return &env{t: t, pool: pool, store: storagetest.New(), owner: auth.Identity{UserID: dbtest.User(t, pool), Username: "dono"}}
}

func (e *env) svc(repo domain.Repository) *application.Service {
	return application.NewService(e.pool, repo, outbox.NewWriter("test"), e.store, bucket, 1<<20, time.Minute,
		slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func (e *env) real() *application.Service { return e.svc(infrastructure.NewRepository()) }

func (e *env) folder(parent *uuid.UUID) domain.Folder {
	e.t.Helper()
	f, err := e.real().CreateFolder(context.Background(), e.owner, parent, "Pasta "+uuid.NewString()[:8], nil)
	if err != nil {
		e.t.Fatal(err)
	}
	return f
}

// pending cria um registro pendente com o objeto já enviado.
func (e *env) pending(folder uuid.UUID) application.UploadResult {
	e.t.Helper()
	up, err := e.real().StartUpload(context.Background(), e.owner, folder, "doc.pdf", "application/pdf", 8)
	if err != nil {
		e.t.Fatal(err)
	}
	if err := e.store.Put(context.Background(), bucket, up.Upload.ObjectKey, strings.NewReader("%PDF-1.4"), 8, "application/pdf"); err != nil {
		e.t.Fatal(err)
	}
	return up
}

func (e *env) ready(folder uuid.UUID) domain.File {
	e.t.Helper()
	f, err := e.real().ConfirmUpload(context.Background(), e.owner, e.pending(folder).File.ID)
	if err != nil {
		e.t.Fatal(err)
	}
	return f
}

func TestEveryRepositoryFailureIsPropagated(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	id := e.owner
	type op = func() func(s *application.Service) error
	ops := map[string]op{
		"BrowseRoot": func() func(*application.Service) error {
			e.folder(nil)
			return func(s *application.Service) error { _, err := s.Browse(ctx, id, nil); return err }
		},
		"Browse": func() func(*application.Service) error {
			f := e.folder(nil)
			e.folder(&f.ID)
			e.ready(f.ID)
			return func(s *application.Service) error { _, err := s.Browse(ctx, id, &f.ID); return err }
		},
		"CreateFolder": func() func(*application.Service) error {
			f := e.folder(nil)
			return func(s *application.Service) error {
				_, err := s.CreateFolder(ctx, id, &f.ID, "Sub "+uuid.NewString()[:6], nil)
				return err
			}
		},
		"UpdateFolder": func() func(*application.Service) error {
			a, b := e.folder(nil), e.folder(nil)
			return func(s *application.Service) error {
				_, err := s.UpdateFolder(ctx, id, a.ID, application.FolderChange{Name: "Movida", Parent: &b.ID, Move: true})
				return err
			}
		},
		"DeleteFolder": func() func(*application.Service) error {
			f := e.folder(nil)
			return func(s *application.Service) error { return s.DeleteFolder(ctx, id, f.ID, false) }
		},
		"ACL": func() func(*application.Service) error {
			f := e.folder(nil)
			return func(s *application.Service) error { _, err := s.ACL(ctx, id, f.ID); return err }
		},
		"SetACL": func() func(*application.Service) error {
			f := e.folder(nil)
			return func(s *application.Service) error {
				_, err := s.SetACL(ctx, id, f.ID, []domain.ACLEntry{{SubjectType: "everyone"}})
				return err
			}
		},
		"StartUpload": func() func(*application.Service) error {
			f := e.folder(nil)
			return func(s *application.Service) error {
				_, err := s.StartUpload(ctx, id, f.ID, "a.pdf", "application/pdf", 8)
				return err
			}
		},
		"ConfirmUpload": func() func(*application.Service) error {
			up := e.pending(e.folder(nil).ID)
			return func(s *application.Service) error { _, err := s.ConfirmUpload(ctx, id, up.File.ID); return err }
		},
		"DownloadURL": func() func(*application.Service) error {
			f := e.ready(e.folder(nil).ID)
			return func(s *application.Service) error { _, _, err := s.DownloadURL(ctx, id, f.ID); return err }
		},
		"UpdateFile": func() func(*application.Service) error {
			f := e.ready(e.folder(nil).ID)
			dst := e.folder(nil)
			return func(s *application.Service) error {
				_, err := s.UpdateFile(ctx, id, f.ID, "novo.pdf", dst.ID)
				return err
			}
		},
		"DeleteFile": func() func(*application.Service) error {
			f := e.ready(e.folder(nil).ID)
			return func(s *application.Service) error { return s.DeleteFile(ctx, id, f.ID) }
		},
		"Search": func() func(*application.Service) error {
			return func(s *application.Service) error { _, err := s.Search(ctx, id, "doc", 5); return err }
		},
		"CollectStaleOnce": func() func(*application.Service) error {
			if _, err := e.real().CollectStaleOnce(ctx, time.Now().Add(time.Hour)); err != nil {
				t.Fatal(err)
			}
			e.pending(e.folder(nil).ID)
			return func(s *application.Service) error {
				_, err := s.CollectStaleOnce(ctx, time.Now().Add(time.Hour))
				return err
			}
		},
	}
	// A busca filtra pela ACL: pasta que falha na avaliação some do
	// resultado (degrada, não derruba a busca) — só a consulta principal
	// propaga erro.
	tolerant := map[string]bool{"Search/Chain": true}
	for name, o := range ops {
		probe := &faultRepo{inner: infrastructure.NewRepository()}
		if err := o()(e.svc(probe)); err != nil {
			t.Fatalf("%s sem falha: %v", name, err)
		}
		for i := range probe.trace {
			if tolerant[name+"/"+probe.trace[i]] {
				continue
			}
			for _, poison := range []bool{false, true} {
				r := &faultRepo{inner: infrastructure.NewRepository()}
				if poison {
					r.poisonAt = i + 1
				} else {
					r.failAt = i + 1
				}
				if err := o()(e.svc(r)); err == nil && !r.noTx {
					t.Errorf("%s: falha (veneno=%v) na chamada %d (%s) foi engolida", name, poison, i+1, probe.trace[i])
				}
			}
		}
	}
}

// Limpeza de uploads abandonados: só apaga o registro depois do objeto; se
// o armazenamento falhar, o registro fica para a próxima varredura.
func TestCollectStaleUploads(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	folder := e.folder(nil)
	enviado := e.pending(folder.ID)
	nuncaEnviado, err := e.real().StartUpload(ctx, e.owner, folder.ID, "nunca.pdf", "application/pdf", 8)
	if err != nil {
		t.Fatal(err)
	}
	recente := e.pending(folder.ID)
	if _, err := e.pool.Exec(ctx, `UPDATE files_objects SET created_at = now() - interval '2 days' WHERE id = ANY($1)`,
		[]uuid.UUID{enviado.File.ID, nuncaEnviado.File.ID}); err != nil {
		t.Fatal(err)
	}
	cutoff := time.Now().Add(-24 * time.Hour)

	e.store.FailDelete = true
	if n, err := e.real().CollectStaleOnce(ctx, cutoff); err != nil || n != 0 {
		t.Fatalf("armazenamento fora: nada é apagado do banco: %d %v", n, err)
	}
	e.store.FailDelete = false
	n, err := e.real().CollectStaleOnce(ctx, cutoff)
	if err != nil || n < 2 {
		t.Fatalf("pendentes antigos (enviado ou não) removidos: %d %v", n, err)
	}
	if e.store.Has(bucket, enviado.Upload.ObjectKey) {
		t.Fatal("o objeto abandonado sai do armazenamento")
	}
	if _, err := e.real().ConfirmUpload(ctx, e.owner, recente.File.ID); err != nil {
		t.Fatalf("pendente recente não é tocado: %v", err)
	}

	// O worker roda a varredura e para com o contexto.
	wctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	m := files.New(modkit.Deps{Pool: e.pool, Outbox: outbox.NewWriter("test"), Storage: e.store, Config: &config.Config{},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	go func() { done <- m.Workers()[0].Run(wctx) }()
	time.Sleep(50 * time.Millisecond)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("worker encerra limpo: %v", err)
	}
	// Falha na varredura: só vira aviso no log, e o worker segue. Espera o
	// aviso em vez de um tempo fixo (numa máquina lenta a varredura podia
	// nem ter começado quando o contexto era cancelado).
	warned := make(chan struct{}, 1)
	failing := application.NewService(e.pool, &faultRepo{inner: infrastructure.NewRepository(), failAt: 1}, outbox.NewWriter("test"),
		e.store, bucket, 1<<20, time.Minute, slog.New(warnSignal{warned}))
	wctx, cancel = context.WithCancel(ctx)
	go func() { done <- failing.CollectStaleUploads(wctx) }()
	select {
	case <-warned:
	case <-time.After(5 * time.Second):
		t.Fatal("a falha da varredura não foi registrada")
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("falha na varredura só é registrada: %v", err)
	}
}

// warnSignal é um slog.Handler que avisa (sem bloquear) a cada registro de
// nível Warn ou acima.
type warnSignal struct{ ch chan struct{} }

func (h warnSignal) Enabled(context.Context, slog.Level) bool { return true }
func (h warnSignal) Handle(_ context.Context, r slog.Record) error {
	if r.Level >= slog.LevelWarn {
		select {
		case h.ch <- struct{}{}:
		default:
		}
	}
	return nil
}
func (h warnSignal) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h warnSignal) WithGroup(string) slog.Handler      { return h }

// Armazenamento: URL de download indisponível e objetos órfãos logados.
func TestStorageFailures(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	f := e.ready(e.folder(nil).ID)
	e.store.FailPresign = true
	if _, _, err := e.real().DownloadURL(ctx, e.owner, f.ID); err == nil {
		t.Fatal("sem URL pré-assinada o download falha")
	}
	if _, err := e.real().StartUpload(ctx, e.owner, f.FolderID, "a.pdf", "application/pdf", 8); err == nil {
		t.Fatal("sem URL pré-assinada o upload não começa")
	}
	e.store.FailPresign = false
	e.store.FailDelete = true
	if err := e.real().DeleteFile(ctx, e.owner, f.ID); err != nil {
		t.Fatalf("falha ao remover o objeto não desfaz a exclusão (só é logada): %v", err)
	}
	e.store.FailDelete = false
	if err := application.MapError(errBoom); !errors.Is(err, errBoom) {
		t.Fatalf("erro desconhecido passa adiante: %v", err)
	}
}

// Busca: pasta cuja ACL não pode ser avaliada some do resultado; o
// provedor da busca global propaga a falha da consulta principal.
func TestSearch(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	f := e.ready(e.folder(nil).ID)
	flaky := &faultRepo{inner: infrastructure.NewRepository(), failAt: 2}
	res, err := e.svc(flaky).Search(ctx, e.owner, "doc", 50)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range res {
		if r.ID == f.ID {
			t.Fatal("arquivo cuja pasta não pôde ser avaliada não aparece")
		}
	}
	m := files.New(modkit.Deps{Pool: e.pool, Outbox: outbox.NewWriter("test"), Storage: e.store, Config: &config.Config{},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := m.SearchProviders()[0].Search(cctx, e.owner, "doc", 5); err == nil {
		t.Fatal("busca com o banco indisponível falha")
	}
	if got, err := m.SearchProviders()[0].Search(ctx, e.owner, "doc", 1); err != nil || len(got) != 1 || m.SearchProviders()[0].Module() != files.Key {
		t.Fatalf("provedor devolve o limite pedido: %v %v", got, err)
	}
	if m.Manifest().Key != files.Key {
		t.Fatal("manifesto")
	}
}

// Handlers: falha do serviço vira 500 sem vazar a causa.
func TestHandlersReportServiceFailures(t *testing.T) {
	e := newEnv(t)
	down := &faultRepo{inner: infrastructure.NewRepository(), failAt: 1}
	h := transport.NewHandlers(e.svc(down), slog.New(slog.NewTextHandler(io.Discard, nil)))
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.WithIdentity(req.Context(), e.owner)))
		})
	})
	h.RegisterRoutes(r)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/files/browse", nil))
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), errBoom.Error()) {
		t.Errorf("raiz com o banco fora: %d %s", rec.Code, rec.Body.String())
	}
}
