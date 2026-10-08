package transport

import (
	"io"
	"mime"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/modules/atlas/application"
	"github.com/yurythx/projeto-nexus/internal/modules/atlas/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/pkg/httputil"
)

// Biblioteca de modelos de documento (ADR 024): consulta e download
// públicos (como o resto do Atlas); cadastro e versões com atlas:manage.
// O arquivo passa pela API (multipart na ida, stream na volta): mesma
// origem do sistema, sem URL do MinIO exposta ao navegador.

// maxCorpoModelo: o arquivo (até 10 MB) mais os campos do formulário.
const maxCorpoModelo = domain.MaxModeloBytes + 1<<20

// semAutoria tira quem publicou das respostas públicas (minimização, LGPD
// art. 6º III): a consulta anônima não precisa saber quem foi.
func semAutoria(m domain.Modelo) domain.Modelo {
	m.UpdatedBy, m.Atual.CreatedBy = "", ""
	for i := range m.Versoes {
		m.Versoes[i].CreatedBy = ""
	}
	return m
}

func (h *Handlers) ListModelosPublic(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListModelos(r.Context(), false)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	for i := range items {
		items[i] = semAutoria(items[i])
	}
	httputil.WriteOK(w, items)
}

func (h *Handlers) ListModelosAdmin(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListModelos(r.Context(), true)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, items)
}

func (h *Handlers) GetModelo(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	m, err := h.svc.GetModelo(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, semAutoria(m))
}

// ArquivoModelo baixa o arquivo da versão atual (ou de ?versao=N).
func (h *Handlers) ArquivoModelo(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	versao := 0
	if q := r.URL.Query().Get("versao"); q != "" {
		if versao, err = strconv.Atoi(q); err != nil || versao < 1 {
			h.fail(w, r, apperrors.BadRequest("versão inválida"))
			return
		}
	}
	v, rc, err := h.svc.ArquivoModelo(r.Context(), id, versao)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", v.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(v.Tamanho, 10))
	// ?inline=1 abre o PDF no navegador (visualizar antes de baixar); os
	// demais formatos são sempre baixados.
	disposicao := "attachment"
	if r.URL.Query().Get("inline") == "1" && v.ContentType == "application/pdf" {
		disposicao = "inline"
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposicao, map[string]string{"filename": v.ArquivoNome}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("ETag", `"`+v.SHA256+`"`)
	if _, err := io.Copy(w, rc); err != nil {
		h.logger.WarnContext(r.Context(), "atlas: download de modelo interrompido", "error", err)
	}
}

// formulario lê o multipart: os campos e o arquivo (obrigatório).
func (h *Handlers) formulario(w http.ResponseWriter, r *http.Request) (application.ArquivoEnviado, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxCorpoModelo)
	if err := r.ParseMultipartForm(maxCorpoModelo); err != nil {
		return application.ArquivoEnviado{}, apperrors.BadRequest("envie um formulário multipart de até 10 MB")
	}
	a := application.ArquivoEnviado{Nota: r.FormValue("nota")}
	f, hdr, err := r.FormFile("arquivo")
	if err != nil {
		return a, apperrors.Validation("arquivo do modelo obrigatório")
	}
	defer f.Close()
	// O formulário inteiro cabe no limite de memória do ParseMultipartForm:
	// o arquivo já está em memória e a leitura não falha.
	a.Nome = hdr.Filename
	a.Conteudo, _ = io.ReadAll(f)
	return a, nil
}

// CriarModelo: multipart com nome, descricao, arquivo e nota (opcional).
func (h *Handlers) CriarModelo(w http.ResponseWriter, r *http.Request) {
	a, err := h.formulario(w, r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	identity, _ := auth.IdentityFromContext(r.Context())
	m, err := h.svc.CriarModelo(r.Context(), identity, domain.Modelo{Nome: r.FormValue("nome"), Descricao: r.FormValue("descricao")}, a)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, m)
}

// NovaVersaoModelo: multipart com arquivo e nota (opcional).
func (h *Handlers) NovaVersaoModelo(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	a, err := h.formulario(w, r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	identity, _ := auth.IdentityFromContext(r.Context())
	m, err := h.svc.NovaVersaoModelo(r.Context(), identity, id, a)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, m)
}

type alterarModeloRequest struct {
	Nome      string `json:"nome" validate:"required,max=150"`
	Descricao string `json:"descricao" validate:"max=2000"`
	Ativo     bool   `json:"ativo"`
}

// AlterarModelo muda nome, descrição e situação (ativo).
func (h *Handlers) AlterarModelo(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req alterarModeloRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	identity, _ := auth.IdentityFromContext(r.Context())
	m, err := h.svc.AlterarModelo(r.Context(), identity, domain.Modelo{ID: id, Nome: req.Nome, Descricao: req.Descricao, Ativo: req.Ativo})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, m)
}

type ligarModeloRequest struct {
	// ModeloID nulo desliga a peça do modelo.
	ModeloID *uuid.UUID `json:"modelo_id"`
}

// LigarModelo liga (ou desliga) o modelo de uma peça do procedimento, na
// versão em vigor (sem publicar nova versão).
func (h *Handlers) LigarModelo(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	peca, err := httputil.UUIDParam(r, "peca")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req ligarModeloRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	wf, err := h.svc.LigarModelo(r.Context(), id, peca, req.ModeloID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, wf)
}

// ModelosDaSerie lista os modelos ligados à série da TTDD (público, sem
// autoria).
func (h *Handlers) ModelosDaSerie(w http.ResponseWriter, r *http.Request) {
	codigo, err := codigoSerie(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items, err := h.svc.ModelosDaSerie(r.Context(), codigo)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	for i := range items {
		items[i] = semAutoria(items[i])
	}
	httputil.WriteOK(w, items)
}

type ligarModeloSerieRequest struct {
	ModeloID uuid.UUID `json:"modelo_id" validate:"required"`
}

// LigarModeloSerie liga um modelo da biblioteca à série.
func (h *Handlers) LigarModeloSerie(w http.ResponseWriter, r *http.Request) {
	codigo, err := codigoSerie(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req ligarModeloSerieRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	identity, _ := auth.IdentityFromContext(r.Context())
	items, err := h.svc.LigarModeloSerie(r.Context(), identity, codigo, req.ModeloID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, items)
}

// DesligarModeloSerie retira o modelo da série (ele continua na biblioteca).
func (h *Handlers) DesligarModeloSerie(w http.ResponseWriter, r *http.Request) {
	codigo, err := codigoSerie(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	modelo, err := httputil.UUIDParam(r, "modelo")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items, err := h.svc.DesligarModeloSerie(r.Context(), codigo, modelo)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, items)
}
