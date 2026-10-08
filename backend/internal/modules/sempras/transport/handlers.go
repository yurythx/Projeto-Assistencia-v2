// Package transport expõe o SEMPRAS via HTTP. A rota exige a permissão em
// algum lugar (RequirePermission); o serviço decide sobre a unidade do
// recurso (auth.Can — ADR 013).
package transport

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/application"
	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/domain"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/auth"
	"github.com/yurythx/projeto-assistencia-v2/pkg/httputil"
)

// Handlers agrupa os handlers HTTP.
type Handlers struct {
	svc    *application.Service
	logger *slog.Logger
}

// NewHandlers cria os handlers.
func NewHandlers(svc *application.Service, logger *slog.Logger) *Handlers {
	return &Handlers{svc: svc, logger: logger}
}

// RegisterRoutes monta as rotas (autenticadas).
func (h *Handlers) RegisterRoutes(r chi.Router) {
	recepcao := auth.RequirePermission(h.logger, auth.PermSemprasRecepcao)
	atendimento := auth.RequirePermission(h.logger, auth.PermSemprasAtendimento)

	r.Get("/sempras/unidades", h.Unidades)
	r.With(auth.RequirePermission(h.logger, auth.PermSemprasGestao)).Get("/sempras/painel", h.Painel)

	r.With(recepcao).Get("/sempras/cidadaos", h.Buscar)
	r.With(recepcao).Post("/sempras/cidadaos", h.Cadastrar)
	r.With(recepcao).Get("/sempras/cidadaos/{id}", h.Cidadao)
	r.With(recepcao).Put("/sempras/cidadaos/{id}", h.Atualizar)
	r.With(atendimento).Get("/sempras/cidadaos/{id}/prontuario", h.Prontuario)
	r.With(atendimento).Post("/sempras/cidadaos/{id}/registros", h.RegistrarEvolucao)

	r.Get("/sempras/unidades/{id}/fila", h.Fila)
	r.Post("/sempras/unidades/{id}/fila", h.Incluir)
	r.With(atendimento).Post("/sempras/unidades/{id}/fila/proximo", h.ChamarProximo)

	for _, acao := range []string{"chamar", "iniciar", "concluir", "ausente", "cancelar"} {
		r.Post("/sempras/atendimentos/{id}/"+acao, h.agir(acao))
	}
}

func (h *Handlers) fail(w http.ResponseWriter, r *http.Request, err error) {
	httputil.WriteError(w, r, h.logger, application.MapError(err))
}

func identity(r *http.Request) auth.Identity {
	id, _ := auth.IdentityFromContext(r.Context())
	return id
}

// Unidades lista as unidades onde quem chama atua.
func (h *Handlers) Unidades(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Unidades(r.Context(), identity(r))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, out)
}

// Painel devolve os números do dia das unidades geridas.
func (h *Handlers) Painel(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Painel(r.Context(), identity(r))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, out)
}

// Buscar procura munícipes (?q=CPF, NIS ou trecho do nome).
func (h *Handlers) Buscar(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Buscar(r.Context(), identity(r), httputil.Query(r, "q", 200))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, out)
}

// CidadaoRequest é o cadastro simples do munícipe.
type CidadaoRequest struct {
	Nome           string `json:"nome" validate:"required,max=200"`
	NomeSocial     string `json:"nome_social" validate:"max=200"`
	CPF            string `json:"cpf" validate:"max=20"`
	NIS            string `json:"nis" validate:"max=20"`
	DataNascimento string `json:"data_nascimento" validate:"max=10"`
	NomeMae        string `json:"nome_mae" validate:"max=200"`
	Telefone       string `json:"telefone" validate:"max=30"`
	Endereco       string `json:"endereco" validate:"max=300"`
}

func (req CidadaoRequest) input() domain.CidadaoInput {
	return domain.CidadaoInput{Nome: req.Nome, NomeSocial: req.NomeSocial, CPF: req.CPF, NIS: req.NIS,
		DataNascimento: req.DataNascimento, NomeMae: req.NomeMae, Telefone: req.Telefone, Endereco: req.Endereco}
}

// Cadastrar grava um novo munícipe.
func (h *Handlers) Cadastrar(w http.ResponseWriter, r *http.Request) {
	var req CidadaoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	c, err := h.svc.Cadastrar(r.Context(), identity(r), req.input())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, c)
}

// Cidadao devolve o cadastro.
func (h *Handlers) Cidadao(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	c, err := h.svc.Cidadao(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, c)
}

// Atualizar regrava o cadastro.
func (h *Handlers) Atualizar(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req CidadaoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	c, err := h.svc.Atualizar(r.Context(), identity(r), id, req.input())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, c)
}

// Prontuario devolve a ficha (leitura auditada).
func (h *Handlers) Prontuario(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	p, err := h.svc.Prontuario(r.Context(), identity(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, p)
}

// RegistroRequest é uma evolução do prontuário.
type RegistroRequest struct {
	UnidadeID     uuid.UUID  `json:"unidade_id" validate:"required"`
	AtendimentoID *uuid.UUID `json:"atendimento_id"`
	Tipo          string     `json:"tipo" validate:"required,oneof=social psicologico"`
	Texto         string     `json:"texto" validate:"required,max=20000"`
}

// RegistrarEvolucao inclui uma evolução no prontuário.
func (h *Handlers) RegistrarEvolucao(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req RegistroRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	reg, err := h.svc.RegistrarEvolucao(r.Context(), identity(r), id, application.EvolucaoInput{
		UnidadeID: req.UnidadeID, AtendimentoID: req.AtendimentoID, Tipo: req.Tipo, Texto: req.Texto})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, reg)
}

// Fila devolve as entradas do dia da unidade.
func (h *Handlers) Fila(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out, err := h.svc.Fila(r.Context(), identity(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, out)
}

// EntradaRequest é a inclusão na fila.
type EntradaRequest struct {
	CidadaoID  uuid.UUID `json:"cidadao_id" validate:"required"`
	Tipo       string    `json:"tipo" validate:"required,oneof=social psicologico"`
	Prioridade string    `json:"prioridade" validate:"max=20"`
}

// Incluir põe o munícipe na fila da unidade.
func (h *Handlers) Incluir(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req EntradaRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	item, err := h.svc.Incluir(r.Context(), identity(r), id, application.EntradaInput{
		CidadaoID: req.CidadaoID, Tipo: req.Tipo, Prioridade: req.Prioridade})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, item)
}

// ChamadaRequest informa a sala ou o guichê de onde se chama.
type ChamadaRequest struct {
	Local string `json:"local" validate:"max=60"`
}

func (h *Handlers) local(w http.ResponseWriter, r *http.Request) (string, error) {
	var req ChamadaRequest
	if r.ContentLength == 0 {
		return "", nil
	}
	err := httputil.Bind(w, r, &req)
	return req.Local, err
}

// ChamarProximo chama o próximo da fila.
func (h *Handlers) ChamarProximo(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	local, err := h.local(w, r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	item, err := h.svc.ChamarProximo(r.Context(), identity(r), id, local)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, item)
}

// agir monta o handler de uma ação da fila.
func (h *Handlers) agir(acao string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httputil.UUIDParam(r, "id")
		if err != nil {
			h.fail(w, r, err)
			return
		}
		local, err := h.local(w, r)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		a, err := h.svc.Agir(r.Context(), identity(r), id, acao, local)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		httputil.WriteOK(w, a)
	}
}
