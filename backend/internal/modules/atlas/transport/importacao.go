package transport

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/modules/atlas/application"
	"github.com/yurythx/projeto-nexus/internal/modules/atlas/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/pkg/httputil"
)

// Importação e exportação de procedimentos em lote (ADR 025).

type importacaoRequest struct {
	Conteudo string `json:"conteudo" validate:"required"`
	// Hash devolvido pela simulação: aplicar exige o mesmo arquivo.
	Hash string `json:"hash" validate:"omitempty,len=64"`
	// Rascunho importa para validação (inativo, sem substituir a versão em
	// vigor — ADR 028).
	Rascunho bool `json:"rascunho"`
}

func (h *Handlers) importacao(w http.ResponseWriter, r *http.Request, aplicar bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxCarga)
	var req importacaoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.fail(w, r, apperrors.BadRequest("corpo inválido ou acima de 10 MB"))
		return
	}
	if err := httputil.Validate(req); err != nil {
		h.fail(w, r, err)
		return
	}
	identity, _ := auth.IdentityFromContext(r.Context())
	importar := h.svc.ImportarProcedimentos
	if req.Rascunho {
		importar = h.svc.ImportarRascunhos
	}
	out, err := importar(r.Context(), identity, req.Conteudo, req.Hash, aplicar)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, out)
}

// SimularImportacao mostra o que o arquivo cria ou muda, sem gravar.
func (h *Handlers) SimularImportacao(w http.ResponseWriter, r *http.Request) {
	h.importacao(w, r, false)
}

// AplicarImportacao grava o arquivo simulado (hash), se não houver erros.
func (h *Handlers) AplicarImportacao(w http.ResponseWriter, r *http.Request) {
	h.importacao(w, r, true)
}

// ExportarProcedimentos baixa os procedimentos em vigor no formato da
// importação (procedimentos.json).
func (h *Handlers) ExportarProcedimentos(w http.ResponseWriter, r *http.Request) {
	arq, err := h.svc.ExportarProcedimentos(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="procedimentos.json"`)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(arq)
}

// Cobertura é o painel de cobertura do Atlas (gestão).
func (h *Handlers) Cobertura(w http.ResponseWriter, r *http.Request) {
	c, err := h.svc.Cobertura(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, c)
}

func (h *Handlers) seguir(w http.ResponseWriter, r *http.Request, fn func(auth.Identity, uuid.UUID) (application.Seguimento, error)) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	identity, _ := auth.IdentityFromContext(r.Context())
	out, err := fn(identity, id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, out)
}

// Seguindo diz se o usuário segue o procedimento.
func (h *Handlers) Seguindo(w http.ResponseWriter, r *http.Request) {
	h.seguir(w, r, func(i auth.Identity, id uuid.UUID) (application.Seguimento, error) {
		return h.svc.Seguindo(r.Context(), i, id)
	})
}

// Seguir passa a avisar o usuário das novas versões do procedimento.
func (h *Handlers) Seguir(w http.ResponseWriter, r *http.Request) {
	h.seguir(w, r, func(i auth.Identity, id uuid.UUID) (application.Seguimento, error) {
		return h.svc.Seguir(r.Context(), i, id, true)
	})
}

// DeixarDeSeguir para os avisos do procedimento.
func (h *Handlers) DeixarDeSeguir(w http.ResponseWriter, r *http.Request) {
	h.seguir(w, r, func(i auth.Identity, id uuid.UUID) (application.Seguimento, error) {
		return h.svc.Seguir(r.Context(), i, id, false)
	})
}

type situacaoRequest struct {
	Situacao string `json:"situacao" validate:"required,oneof=RASCUNHO EM_VALIDACAO"`
}

// MudarSituacao alterna o fluxo entre rascunho e em validação.
func (h *Handlers) MudarSituacao(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req situacaoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	wf, err := h.svc.MudarSituacao(r.Context(), id, req.Situacao)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, wf)
}

// Homologar publica o fluxo validado.
func (h *Handlers) Homologar(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	identity, _ := auth.IdentityFromContext(r.Context())
	wf, err := h.svc.Homologar(r.Context(), identity, id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, wf)
}

// Validacoes lista as entrevistas do procedimento.
func (h *Handlers) Validacoes(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	v, err := h.svc.Validacoes(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, v)
}

type validacaoRequest struct {
	RealizadaEm   string `json:"realizada_em" validate:"required,datetime=2006-01-02"`
	Unidade       string `json:"unidade" validate:"required,max=200"`
	Participantes string `json:"participantes" validate:"max=1000"`
	Registro      string `json:"registro" validate:"required,max=10000"`
	Pendencias    string `json:"pendencias" validate:"max=5000"`
}

// RegistrarValidacao grava uma entrevista de validação.
func (h *Handlers) RegistrarValidacao(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req validacaoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	data, _ := time.Parse("2006-01-02", req.RealizadaEm) // já conferida pelo validate
	identity, _ := auth.IdentityFromContext(r.Context())
	v, err := h.svc.RegistrarValidacao(r.Context(), identity, id, domain.Validacao{RealizadaEm: data, Unidade: req.Unidade,
		Participantes: req.Participantes, Registro: req.Registro, Pendencias: req.Pendencias})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, v)
}
