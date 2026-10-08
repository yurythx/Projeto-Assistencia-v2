// Package transport expõe a API HTTP do Trâmite.
package transport

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/modules/tramite/application"
	"github.com/yurythx/projeto-nexus/internal/modules/tramite/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/pkg/httputil"
)

// Handlers da API do Trâmite.
type Handlers struct {
	svc         *application.Service
	logger      *slog.Logger
	maxPageSize int
}

// NewHandlers cria os handlers.
func NewHandlers(svc *application.Service, logger *slog.Logger, maxPageSize int) *Handlers {
	return &Handlers{svc: svc, logger: logger, maxPageSize: maxPageSize}
}

// RegisterRoutes monta as rotas (autenticadas).
func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Get("/tramite/tipos", h.Tipos)
	r.Get("/tramite/processos", h.List)
	r.Get("/tramite/processos/{id}", h.Get)
	r.With(auth.RequirePermission(h.logger, auth.PermTramiteCreate)).Post("/tramite/processos", h.Abrir)
	r.With(auth.RequirePermission(h.logger, auth.PermTramiteRoute)).Post("/tramite/processos/{id}/tramitar", h.Tramitar)
	r.Post("/tramite/processos/{id}/concluir", h.Concluir)
	r.Put("/tramite/processos/{id}/classificacao", h.Classificar)
	r.Post("/tramite/processos/{id}/arquivar", h.Arquivar)
	r.With(auth.RequirePermission(h.logger, auth.PermTramiteManage)).Post("/tramite/processos/{id}/reabrir", h.Reabrir)
	r.Post("/tramite/processos/{id}/acessos", h.ConcederAcesso)
	r.Delete("/tramite/processos/{id}/acessos/{userID}", h.RevogarAcesso)
	r.Post("/tramite/processos/{id}/uploads", h.Upload)
	r.Post("/tramite/processos/{id}/documentos", h.AdicionarDocumento)
	r.Get("/tramite/documentos/{docID}", h.GetDocumento)
	r.Put("/tramite/documentos/{docID}", h.EditarDocumento)
	r.Post("/tramite/documentos/{docID}/assinatura", h.SolicitarAssinatura)
}

func (h *Handlers) fail(w http.ResponseWriter, r *http.Request, err error) {
	httputil.WriteError(w, r, h.logger, application.MapError(err))
}

func identity(r *http.Request) auth.Identity {
	id, _ := auth.IdentityFromContext(r.Context())
	return id
}

func (h *Handlers) Tipos(w http.ResponseWriter, r *http.Request) {
	tipos, err := h.svc.Tipos(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, tipos)
}

func (h *Handlers) List(w http.ResponseWriter, r *http.Request) {
	un, err := httputil.OptionalUUIDQuery(r, "unidade_id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	p := httputil.Page(r, h.maxPageSize)
	items, total, err := h.svc.List(r.Context(), identity(r), domain.Filter{
		Query: httputil.Query(r, "q", 200), Status: httputil.Query(r, "status", 20), UnidadeID: un,
		Mine: r.URL.Query().Get("minha_caixa") == "true",
	}, p)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WritePage(w, items, p, total)
}

func (h *Handlers) Get(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	v, err := h.svc.Get(r.Context(), identity(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, v)
}

type abrirRequest struct {
	TipoID          uuid.UUID `json:"tipo_id" validate:"required"`
	Assunto         string    `json:"assunto" validate:"required,max=300"`
	Interessado     string    `json:"interessado" validate:"max=300"`
	Descricao       string    `json:"descricao" validate:"max=20000"`
	Sigilo          string    `json:"sigilo" validate:"required,oneof=publico restrito sigiloso"`
	UnidadeOrigemID uuid.UUID `json:"unidade_origem_id" validate:"required"`
	// Classificação pelo Atlas (opcional — ADR 026).
	AtlasProcedimentoID *uuid.UUID `json:"atlas_procedimento_id"`
	CodigoTTDD          string     `json:"codigo_ttdd" validate:"max=32"`
}

func (h *Handlers) Abrir(w http.ResponseWriter, r *http.Request) {
	var req abrirRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	p, err := h.svc.Abrir(r.Context(), identity(r), application.AbrirInput{
		TipoID: req.TipoID, Assunto: req.Assunto, Interessado: req.Interessado, Descricao: req.Descricao,
		Sigilo: req.Sigilo, UnidadeOrigemID: req.UnidadeOrigemID,
		AtlasProcedimentoID: req.AtlasProcedimentoID, CodigoTTDD: req.CodigoTTDD,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, p)
}

type despachoRequest struct {
	ParaUnidadeID *uuid.UUID `json:"para_unidade_id"`
	Despacho      string     `json:"despacho" validate:"required,min=3,max=5000"`
}

func (h *Handlers) action(w http.ResponseWriter, r *http.Request, fn func(id uuid.UUID, req despachoRequest) (domain.Processo, error)) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req despachoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	p, err := fn(id, req)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, p)
}

func (h *Handlers) Tramitar(w http.ResponseWriter, r *http.Request) {
	h.action(w, r, func(id uuid.UUID, req despachoRequest) (domain.Processo, error) {
		if req.ParaUnidadeID == nil {
			return domain.Processo{}, httputilValidation("para_unidade_id é obrigatório")
		}
		return h.svc.Tramitar(r.Context(), identity(r), id, *req.ParaUnidadeID, req.Despacho)
	})
}

func (h *Handlers) Concluir(w http.ResponseWriter, r *http.Request) {
	h.action(w, r, func(id uuid.UUID, req despachoRequest) (domain.Processo, error) {
		return h.svc.Concluir(r.Context(), identity(r), id, req.Despacho)
	})
}

func (h *Handlers) Arquivar(w http.ResponseWriter, r *http.Request) {
	h.action(w, r, func(id uuid.UUID, req despachoRequest) (domain.Processo, error) {
		return h.svc.Arquivar(r.Context(), identity(r), id, req.Despacho)
	})
}

func (h *Handlers) Reabrir(w http.ResponseWriter, r *http.Request) {
	h.action(w, r, func(id uuid.UUID, req despachoRequest) (domain.Processo, error) {
		return h.svc.Reabrir(r.Context(), identity(r), id, req.Despacho)
	})
}

type acessoRequest struct {
	UserID uuid.UUID `json:"user_id" validate:"required"`
}

func (h *Handlers) ConcederAcesso(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req acessoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.ConcederAcesso(r.Context(), identity(r), id, req.UserID); err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteNoContent(w)
}

func (h *Handlers) RevogarAcesso(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	userID, err := httputil.UUIDParam(r, "userID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.RevogarAcesso(r.Context(), identity(r), id, userID); err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteNoContent(w)
}

type uploadRequest struct {
	Filename    string `json:"filename" validate:"required,max=255"`
	ContentType string `json:"content_type" validate:"required,max=150"`
}

func (h *Handlers) Upload(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req uploadRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	t, err := h.svc.UploadAnexo(r.Context(), identity(r), id, req.Filename, req.ContentType)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, t)
}

type documentoRequest struct {
	Tipo      string `json:"tipo" validate:"max=60"`
	Titulo    string `json:"titulo" validate:"required,max=300"`
	Conteudo  string `json:"conteudo" validate:"max=200000"`
	ObjectKey string `json:"object_key" validate:"max=400"`
}

func (h *Handlers) AdicionarDocumento(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req documentoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	d, err := h.svc.AdicionarDocumento(r.Context(), identity(r), id, application.NovoDocumentoInput{
		Tipo: req.Tipo, Titulo: req.Titulo, Conteudo: req.Conteudo, ObjectKey: req.ObjectKey,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, d)
}

func (h *Handlers) GetDocumento(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "docID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	d, err := h.svc.GetDocumento(r.Context(), identity(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httputil.WriteOK(w, d)
}

func (h *Handlers) EditarDocumento(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "docID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req documentoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	d, err := h.svc.EditarDocumento(r.Context(), identity(r), id, req.Titulo, req.Conteudo)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, d)
}

type assinaturaRequest struct {
	SignerIDs  []uuid.UUID `json:"signer_ids" validate:"required,min=1,max=50"`
	Sequential bool        `json:"sequential"`
}

func (h *Handlers) SolicitarAssinatura(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "docID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req assinaturaRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	d, err := h.svc.SolicitarAssinatura(r.Context(), identity(r), id, req.SignerIDs, req.Sequential)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, d)
}

type classificacaoRequest struct {
	AtlasProcedimentoID *uuid.UUID `json:"atlas_procedimento_id"`
	CodigoTTDD          string     `json:"codigo_ttdd" validate:"max=32"`
}

// Classificar define o procedimento do Atlas e a série da TTDD do processo.
func (h *Handlers) Classificar(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req classificacaoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	p, err := h.svc.Classificar(r.Context(), identity(r), id, req.AtlasProcedimentoID, req.CodigoTTDD)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, p)
}
