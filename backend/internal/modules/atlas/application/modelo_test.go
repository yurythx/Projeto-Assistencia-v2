package application_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/modules/atlas/application"
	"github.com/yurythx/projeto-nexus/internal/modules/atlas/domain"
	"github.com/yurythx/projeto-nexus/internal/modules/atlas/infrastructure"
	"github.com/yurythx/projeto-nexus/internal/modules/atlas/transport"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
	"github.com/yurythx/projeto-nexus/internal/platform/storage/storagetest"
)

const bucket = "atlas-teste"

// gravador é o armazenamento em memória que lembra as chaves gravadas
// (para conferir que nenhum arquivo fica órfão).
type gravador struct {
	*storagetest.Memory
	chaves []string
}

func novoGravador() *gravador { return &gravador{Memory: storagetest.New()} }

func (g *gravador) Put(ctx context.Context, b, name string, r io.Reader, size int64, ct string) error {
	if err := g.Memory.Put(ctx, b, name, r, size, ct); err != nil {
		return err
	}
	g.chaves = append(g.chaves, name)
	return nil
}

// ficaram conta os arquivos gravados que continuam no armazenamento.
func (g *gravador) ficaram() int {
	n := 0
	for _, k := range g.chaves {
		if g.Has(bucket, k) {
			n++
		}
	}
	return n
}

var conteudoDocx = append([]byte("PK\x03\x04"), []byte("conteúdo do modelo")...)

func docx() application.ArquivoEnviado {
	return application.ArquivoEnviado{Nome: "Requerimento.docx", Conteudo: conteudoDocx, Nota: "primeira versão"}
}

func modeloNovo() domain.Modelo {
	return domain.Modelo{Nome: "Modelo " + uuid.NewString()[:8], Descricao: "Modelo de teste"}
}

func (e *env) modelo() domain.Modelo {
	e.t.Helper()
	m, err := e.real().CriarModelo(context.Background(), gestor, modeloNovo(), docx())
	if err != nil {
		e.t.Fatal(err)
	}
	return m
}

func comModelo(w domain.Workflow, id uuid.UUID) domain.Workflow {
	w.Etapas[0].Documentos[0].ModeloID = &id
	return w
}

func codigo(err error) string {
	var ae *apperrors.Error
	if errors.As(err, &ae) {
		return string(ae.Code)
	}
	return ""
}

func TestBibliotecaDeModelos(t *testing.T) {
	e := &env{t: t, pool: dbtest.Pool(t)}
	ctx := context.Background()
	s := e.real()

	m, err := s.CriarModelo(ctx, auth.Identity{Subject: "sub-1"}, modeloNovo(), docx())
	if err != nil || m.Atual.Versao != 1 || m.Atual.ContentType != "application/vnd.openxmlformats-officedocument.wordprocessingml.document" ||
		len(m.Atual.SHA256) != 64 || m.Atual.CreatedBy != "sub-1" || len(m.Versoes) != 1 || !m.Ativo {
		t.Fatalf("criar: %+v %v", m, err)
	}
	// Nova versão: a atual passa a ser a 2; a 1 fica no histórico e baixável.
	v2 := application.ArquivoEnviado{Nome: "C:\\pasta\\Requerimento v2.pdf", Conteudo: []byte("%PDF-1.7 x"), Nota: "  revisão  "}
	m2, err := s.NovaVersaoModelo(ctx, auth.Identity{Username: "maria", Subject: "sub-2"}, m.ID, v2)
	if err != nil || m2.Atual.Versao != 2 || m2.Atual.ArquivoNome != "Requerimento v2.pdf" || m2.Atual.Nota != "revisão" ||
		m2.UpdatedBy != "maria" || len(m2.Versoes) != 2 {
		t.Fatalf("nova versão: %+v %v", m2, err)
	}
	v, rc, err := s.ArquivoModelo(ctx, m.ID, 1)
	if err != nil || v.Versao != 1 {
		t.Fatalf("baixar a versão 1: %+v %v", v, err)
	}
	if b, _ := io.ReadAll(rc); !bytes.Equal(b, conteudoDocx) {
		t.Fatal("conteúdo da versão 1 alterado")
	}
	rc.Close()
	if _, _, err := s.ArquivoModelo(ctx, m.ID, 9); codigo(err) != "NOT_FOUND" {
		t.Fatalf("versão inexistente: %v", err)
	}

	// Peça ligada ao modelo: a leitura do procedimento traz a versão atual.
	w, err := s.Create(ctx, gestor, comModelo(novo(), m.ID))
	if err != nil {
		t.Fatal(err)
	}
	d := w.Etapas[0].Documentos[0]
	if d.ModeloID == nil || d.Modelo == nil || d.Modelo.Versao != 2 || d.Modelo.Nome != m.Nome {
		t.Fatalf("peça ligada: %+v", d)
	}
	if got, _ := s.GetModelo(ctx, m.ID); got.PecasLigadas != 1 {
		t.Fatalf("peças ligadas: %d", got.PecasLigadas)
	}

	// Desativado: sai da lista pública e da escolha de novas peças, mas a
	// nova versão de um procedimento que já o usava continua possível.
	des, err := s.AlterarModelo(ctx, gestor, domain.Modelo{ID: m.ID, Nome: " " + m.Nome + " ", Descricao: "x", Ativo: false})
	if err != nil || des.Ativo || des.Nome != m.Nome {
		t.Fatalf("desativar: %+v %v", des, err)
	}
	if lista, _ := s.ListModelos(ctx, false); contem(lista, m.ID) {
		t.Fatal("desativado na lista pública")
	}
	if lista, _ := s.ListModelos(ctx, true); !contem(lista, m.ID) {
		t.Fatal("desativado some da gestão")
	}
	if _, err := s.Create(ctx, gestor, comModelo(novo(), m.ID)); codigo(err) != "VALIDATION_ERROR" {
		t.Fatalf("nova peça com modelo desativado: %v", err)
	}
	if _, err := s.NovaVersao(ctx, gestor, w.ID, comModelo(novo(), m.ID)); err != nil {
		t.Fatalf("nova versão do procedimento que já usava o modelo: %v", err)
	}
	if _, err := s.Create(ctx, gestor, comModelo(novo(), uuid.New())); codigo(err) != "VALIDATION_ERROR" {
		t.Fatalf("modelo inexistente: %v", err)
	}

	// Validações e erros de domínio.
	outro := e.modelo()
	for name, c := range map[string]struct {
		err  error
		want string
	}{
		"nome vazio": {func() error { _, err := s.CriarModelo(ctx, gestor, domain.Modelo{}, docx()); return err }(), "VALIDATION_ERROR"},
		"extensão": {func() error {
			_, err := s.CriarModelo(ctx, gestor, modeloNovo(), application.ArquivoEnviado{Nome: "x.exe", Conteudo: []byte("MZ")})
			return err
		}(), "VALIDATION_ERROR"},
		"nota longa": {func() error {
			a := docx()
			a.Nota = strings.Repeat("a", 501)
			_, err := s.NovaVersaoModelo(ctx, gestor, outro.ID, a)
			return err
		}(), "VALIDATION_ERROR"},
		"nome repetido": {func() error {
			_, err := s.CriarModelo(ctx, gestor, domain.Modelo{Nome: strings.ToUpper(outro.Nome)}, docx())
			return err
		}(), "CONFLICT"},
		"renomear repetido": {func() error {
			_, err := s.AlterarModelo(ctx, gestor, domain.Modelo{ID: outro.ID, Nome: m.Nome, Ativo: true})
			return err
		}(), "CONFLICT"},
		"alterar inválido": {func() error {
			_, err := s.AlterarModelo(ctx, gestor, domain.Modelo{ID: outro.ID})
			return err
		}(), "VALIDATION_ERROR"},
		"alterar inexistente": {func() error {
			_, err := s.AlterarModelo(ctx, gestor, domain.Modelo{ID: uuid.New(), Nome: "x"})
			return err
		}(), "NOT_FOUND"},
		"versão de inexistente": {func() error { _, err := s.NovaVersaoModelo(ctx, gestor, uuid.New(), docx()); return err }(), "NOT_FOUND"},
		"detalhe inexistente":   {func() error { _, err := s.GetModelo(ctx, uuid.New()); return err }(), "NOT_FOUND"},
	} {
		if codigo(c.err) != c.want {
			t.Errorf("%s: %v, quero %s", name, c.err, c.want)
		}
	}
}

func contem(ms []domain.Modelo, id uuid.UUID) bool {
	for _, m := range ms {
		if m.ID == id {
			return true
		}
	}
	return false
}

// O arquivo vai ao armazenamento antes da transação: se ela falha, ele é
// apagado; se nem o armazenamento aceita, nada é registrado.
func TestModeloSemArquivoOrfao(t *testing.T) {
	e := &env{t: t, pool: dbtest.Pool(t)}
	ctx := context.Background()
	m := e.modelo()
	antes := e.store.ficaram()
	falha := e.svc(&faultRepo{inner: infrastructure.NewRepository(), failAt: 1})
	if _, err := falha.CriarModelo(ctx, gestor, modeloNovo(), docx()); err == nil {
		t.Fatal("falha no banco engolida")
	}
	falhaVersao := e.svc(&faultRepo{inner: infrastructure.NewRepository(), failAt: 2})
	if _, err := falhaVersao.NovaVersaoModelo(ctx, gestor, m.ID, docx()); err == nil {
		t.Fatal("falha no banco engolida (versão)")
	}
	if e.store.ficaram() != antes {
		t.Fatal("arquivo órfão no armazenamento")
	}
	// Apagar também falha: o órfão é só registrado no log.
	e.store.FailDelete = true
	falha = e.svc(&faultRepo{inner: infrastructure.NewRepository(), failAt: 1})
	if _, err := falha.CriarModelo(ctx, gestor, modeloNovo(), docx()); err == nil {
		t.Fatal("falha no banco engolida")
	}
	e.store.FailDelete = false
	e.store.FailPut = true
	if _, err := e.real().CriarModelo(ctx, gestor, modeloNovo(), docx()); !errors.Is(err, storagetest.ErrInjected) {
		t.Fatalf("armazenamento fora: %v", err)
	}
	e.store.FailPut = false
	e.store.FailGet = true
	if _, _, err := e.real().ArquivoModelo(ctx, m.ID, 0); !errors.Is(err, storagetest.ErrInjected) {
		t.Fatalf("download com o armazenamento fora: %v", err)
	}
}

// formModelo monta o multipart de cadastro/versão.
func formModelo(campos map[string]string, arquivo string, conteudo []byte) (*bytes.Buffer, string) {
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	for k, v := range campos {
		_ = mw.WriteField(k, v)
	}
	if arquivo != "" {
		fw, _ := mw.CreateFormFile("arquivo", arquivo)
		_, _ = fw.Write(conteudo)
	}
	_ = mw.Close()
	return &b, mw.FormDataContentType()
}

func TestModeloHTTP(t *testing.T) {
	e := &env{t: t, pool: dbtest.Pool(t)}
	gestorMaria := gestor
	gestorMaria.Username = "maria"
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.WithIdentity(req.Context(), gestorMaria)))
		})
	})
	h := transport.NewHandlers(e.real(), logger, 100)
	h.RegisterPublicRoutes(r)
	h.RegisterRoutes(r, unlimited{})
	do := func(method, path string, body io.Reader, ct string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, body)
		req.Header.Set("Content-Type", ct)
		r.ServeHTTP(rec, req)
		return rec
	}

	nome := "Ofício " + uuid.NewString()[:8]
	body, ct := formModelo(map[string]string{"nome": nome, "descricao": "Ofício padrão", "nota": "v1"}, "Ofício padrão.docx", conteudoDocx)
	rec := do(http.MethodPost, "/atlas/admin/modelos", body, ct)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"updated_by":"maria"`) {
		t.Fatalf("cadastro: %d %s", rec.Code, rec.Body.String())
	}
	m, err := e.real().ListModelos(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	var id string
	for _, x := range m {
		if x.Nome == nome {
			id = x.ID.String()
		}
	}

	// Download: nome com acento (RFC 2231), tipo, tamanho e hash.
	rec = do(http.MethodGet, "/atlas/modelos/"+id+"/arquivo", nil, "")
	if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), conteudoDocx) ||
		!strings.Contains(rec.Header().Get("Content-Disposition"), "attachment; filename*=utf-8''Of%C3%ADcio%20padr%C3%A3o.docx") ||
		rec.Header().Get("X-Content-Type-Options") != "nosniff" || rec.Header().Get("ETag") == "" {
		t.Fatalf("download: %d %v", rec.Code, rec.Header())
	}
	// Respostas públicas sem autoria.
	for _, path := range []string{"/atlas/modelos", "/atlas/modelos/" + id} {
		if rec = do(http.MethodGet, path, nil, ""); rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "maria") ||
			!strings.Contains(rec.Body.String(), nome) {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Body.String())
		}
	}
	if rec = do(http.MethodGet, "/atlas/admin/modelos", nil, ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "maria") {
		t.Fatalf("gestão: %d %s", rec.Code, rec.Body.String())
	}

	body, ct = formModelo(map[string]string{"nota": "v2"}, "oficio.odt", []byte("PK\x03\x04odt"))
	if rec = do(http.MethodPost, "/atlas/admin/modelos/"+id+"/versoes", body, ct); rec.Code != http.StatusCreated ||
		!strings.Contains(rec.Body.String(), `"versao":2`) {
		t.Fatalf("nova versão: %d %s", rec.Code, rec.Body.String())
	}
	// ?inline=1: só o PDF abre no navegador; os demais formatos baixam.
	if rec = do(http.MethodGet, "/atlas/modelos/"+id+"/arquivo?versao=1&inline=1", nil, ""); !strings.HasPrefix(rec.Header().Get("Content-Disposition"), "attachment") {
		t.Fatalf("docx com inline: %v", rec.Header())
	}
	body, ct = formModelo(nil, "oficio.pdf", []byte("%PDF-1.7 x"))
	if rec = do(http.MethodPost, "/atlas/admin/modelos/"+id+"/versoes", body, ct); rec.Code != http.StatusCreated {
		t.Fatalf("versão em PDF: %d %s", rec.Code, rec.Body.String())
	}
	if rec = do(http.MethodGet, "/atlas/modelos/"+id+"/arquivo?inline=1", nil, ""); !strings.HasPrefix(rec.Header().Get("Content-Disposition"), "inline") {
		t.Fatalf("pdf com inline: %v", rec.Header())
	}
	if rec = do(http.MethodGet, "/atlas/modelos/"+id+"/arquivo?versao=1", nil, ""); !bytes.Equal(rec.Body.Bytes(), conteudoDocx) {
		t.Fatalf("versão 1: %d", rec.Code)
	}
	if rec = do(http.MethodPut, "/atlas/admin/modelos/"+id, strings.NewReader(`{"nome":"`+nome+`","descricao":"d","ativo":false}`),
		"application/json"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"ativo":false`) {
		t.Fatalf("desativar: %d %s", rec.Code, rec.Body.String())
	}

	// Pedidos malformados.
	semArquivo, ctSem := formModelo(map[string]string{"nome": "x"}, "", nil)
	invalido, ctInv := formModelo(map[string]string{"nome": "y"}, "x.docx", []byte("não é docx"))
	for _, c := range []struct {
		method, path string
		body         io.Reader
		ct           string
		want         int
	}{
		{http.MethodPost, "/atlas/admin/modelos", strings.NewReader("{}"), "application/json", http.StatusBadRequest},
		{http.MethodPost, "/atlas/admin/modelos", semArquivo, ctSem, http.StatusUnprocessableEntity},
		{http.MethodPost, "/atlas/admin/modelos", invalido, ctInv, http.StatusUnprocessableEntity},
		{http.MethodPost, "/atlas/admin/modelos/" + id + "/versoes", strings.NewReader("{}"), "application/json", http.StatusBadRequest},
		{http.MethodPost, "/atlas/admin/modelos/x/versoes", nil, "", http.StatusBadRequest},
		{http.MethodPut, "/atlas/admin/modelos/x", nil, "", http.StatusBadRequest},
		{http.MethodPut, "/atlas/admin/modelos/" + id, strings.NewReader("{"), "application/json", http.StatusBadRequest},
		{http.MethodGet, "/atlas/modelos/x", nil, "", http.StatusBadRequest},
		{http.MethodGet, "/atlas/modelos/x/arquivo", nil, "", http.StatusBadRequest},
		{http.MethodGet, "/atlas/modelos/" + id + "/arquivo?versao=abc", nil, "", http.StatusBadRequest},
		{http.MethodGet, "/atlas/modelos/" + id + "/arquivo?versao=0", nil, "", http.StatusBadRequest},
		{http.MethodGet, "/atlas/modelos/" + uuid.NewString() + "/arquivo", nil, "", http.StatusNotFound},
	} {
		if rec = do(c.method, c.path, c.body, c.ct); rec.Code != c.want {
			t.Errorf("%s %s: %d, quero %d — %s", c.method, c.path, rec.Code, c.want, rec.Body.String())
		}
	}

	// Leitura do armazenamento interrompida no meio do download: só log.
	e.store.FailRead = true
	if rec = do(http.MethodGet, "/atlas/modelos/"+id+"/arquivo", nil, ""); rec.Code != http.StatusOK {
		t.Fatalf("download interrompido: %d", rec.Code)
	}
}

func TestModeloHandlersReportServiceFailures(t *testing.T) {
	e := &env{t: t, pool: dbtest.Pool(t)}
	down := &faultRepo{inner: infrastructure.NewRepository(), failAt: 1}
	h := transport.NewHandlers(e.svc(down), logger, 100)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.WithIdentity(req.Context(), gestor)))
		})
	})
	h.RegisterPublicRoutes(r)
	h.RegisterRoutes(r, unlimited{})
	id := uuid.NewString()
	for _, c := range []struct{ method, path string }{
		{http.MethodGet, "/atlas/modelos"},
		{http.MethodGet, "/atlas/admin/modelos"},
		{http.MethodGet, "/atlas/modelos/" + id},
		{http.MethodGet, "/atlas/modelos/" + id + "/arquivo"},
		{http.MethodPost, "/atlas/admin/modelos"},
		{http.MethodPost, "/atlas/admin/modelos/" + id + "/versoes"},
		{http.MethodPut, "/atlas/admin/modelos/" + id},
	} {
		down.calls = 0
		body, ct := formModelo(map[string]string{"nome": "Modelo " + uuid.NewString()[:8]}, "a.docx", conteudoDocx)
		var rd io.Reader = body
		if c.method == http.MethodPut {
			rd, ct = strings.NewReader(`{"nome":"x","ativo":true}`), "application/json"
		}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(c.method, c.path, rd)
		req.Header.Set("Content-Type", ct)
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), errBoom.Error()) {
			t.Errorf("%s %s com o banco fora: %d %s", c.method, c.path, rec.Code, rec.Body.String())
		}
	}
}

// Ligar o modelo direto na peça do procedimento em vigor (sem nova versão).
func TestLigarModeloNaPeca(t *testing.T) {
	e := &env{t: t, pool: dbtest.Pool(t)}
	ctx := context.Background()
	s := e.real()
	w, m := e.workflow(true), e.modelo()
	peca := w.Etapas[0].Documentos[0].ID

	got, err := s.LigarModelo(ctx, w.ID, peca, &m.ID)
	if err != nil || got.ID != w.ID || got.Versao != w.Versao || got.Etapas[0].Documentos[0].Modelo == nil ||
		got.Etapas[0].Documentos[0].Modelo.Nome != m.Nome {
		t.Fatalf("ligar: %+v %v", got.Etapas[0].Documentos[0], err)
	}
	if got, err = s.LigarModelo(ctx, w.ID, peca, nil); err != nil || got.Etapas[0].Documentos[0].ModeloID != nil {
		t.Fatalf("desligar: %+v %v", got.Etapas[0].Documentos[0], err)
	}
	if _, err := s.LigarModelo(ctx, uuid.New(), peca, &m.ID); codigo(err) != "NOT_FOUND" {
		t.Fatalf("peça de outro procedimento: %v", err)
	}
	if _, err := s.AlterarModelo(ctx, gestor, domain.Modelo{ID: m.ID, Nome: m.Nome, Ativo: false}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.LigarModelo(ctx, w.ID, peca, &m.ID); codigo(err) != "VALIDATION_ERROR" {
		t.Fatalf("modelo desativado: %v", err)
	}
}

func TestLigarModeloHTTP(t *testing.T) {
	e := &env{t: t, pool: dbtest.Pool(t)}
	w, m := e.workflow(true), e.modelo()
	peca := w.Etapas[0].Documentos[0].ID.String()
	ok := func(svc *application.Service) *chi.Mux {
		r := chi.NewRouter()
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				next.ServeHTTP(rw, req.WithContext(auth.WithIdentity(req.Context(), gestor)))
			})
		})
		h := transport.NewHandlers(svc, logger, 100)
		h.RegisterRoutes(r, unlimited{})
		return r
	}
	r := ok(e.real())
	base := "/atlas/admin/workflows/" + w.ID.String() + "/pecas/"
	for _, c := range []struct {
		path, body string
		want       int
	}{
		{base + peca + "/modelo", `{"modelo_id":"` + m.ID.String() + `"}`, http.StatusOK},
		{base + peca + "/modelo", `{"modelo_id":null}`, http.StatusOK},
		{base + peca + "/modelo", `{`, http.StatusBadRequest},
		{base + "x/modelo", `{}`, http.StatusBadRequest},
		{"/atlas/admin/workflows/x/pecas/" + peca + "/modelo", `{}`, http.StatusBadRequest},
		{base + uuid.NewString() + "/modelo", `{}`, http.StatusNotFound},
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, c.path, strings.NewReader(c.body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(rec, req)
		if rec.Code != c.want {
			t.Errorf("PUT %s %s: %d, quero %d — %s", c.path, c.body, rec.Code, c.want, rec.Body.String())
		}
	}
	down := ok(e.svc(&faultRepo{inner: infrastructure.NewRepository(), failAt: 1}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, base+peca+"/modelo", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	down.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("banco fora: %d", rec.Code)
	}
}
