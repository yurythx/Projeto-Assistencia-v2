package iaconfig

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/platform/audit"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/pkg/httputil"
)

// Ações de auditoria (nunca com a chave).
const (
	ActionConexaoCriada   = "ia.conexao.criada"
	ActionConexaoAlterada = "ia.conexao.alterada"
	ActionConexaoExcluida = "ia.conexao.excluida"
	ActionConexaoTestada  = "ia.conexao.testada"
	ActionUsoAlterado     = "ia.uso.alterado"
	resourceConexao       = "ia_conexao"
	resourceUso           = "ia_uso"
)

// Handlers da configuração de IA (ia:manage).
type Handlers struct {
	store    Store
	cliente  *Cliente
	ambiente *Conexao
	audit    *audit.Writer
	logger   *slog.Logger
}

// NewHandlers cria os handlers. ambiente é a conexão do .env (exibida na
// tela enquanto a função não é configurada).
func NewHandlers(store Store, cliente *Cliente, ambiente *Conexao, auditWriter *audit.Writer, logger *slog.Logger) *Handlers {
	return &Handlers{store: store, cliente: cliente, ambiente: ambiente, audit: auditWriter, logger: logger}
}

// RegisterRoutes monta as rotas (r já autenticado) atrás de ia:manage.
func RegisterRoutes(r chi.Router, h *Handlers, logger *slog.Logger) {
	r.Group(func(r chi.Router) {
		r.Use(auth.RequirePermission(logger, auth.PermIAManage))
		r.Get("/ia/provedores", h.ListarProvedores)
		r.Get("/ia/conexoes", h.Listar)
		r.Post("/ia/conexoes", h.Criar)
		r.Put("/ia/conexoes/{id}", h.Alterar)
		r.Delete("/ia/conexoes/{id}", h.Excluir)
		r.Post("/ia/conexoes/{id}/testar", h.Testar)
		r.Get("/ia/uso/{funcao}", h.ObterUso)
		r.Put("/ia/uso/{funcao}", h.DefinirUso)
	})
}

// MapError traduz os erros do pacote.
func MapError(err error) error {
	var inv InvalidaError
	switch {
	case errors.As(err, &inv):
		return apperrors.Validation(inv.Msg)
	case errors.Is(err, ErrNaoEncontrada):
		return apperrors.NotFound("conexão de IA não encontrada")
	case errors.Is(err, ErrNomeRepetido):
		return apperrors.Conflict("já existe uma conexão com este nome")
	case errors.Is(err, ErrEmUso):
		return apperrors.Conflict("a conexão está em uso: troque-a nas funções antes de excluir")
	}
	return err
}

func (h *Handlers) fail(w http.ResponseWriter, r *http.Request, err error) {
	httputil.WriteError(w, r, h.logger, MapError(err))
}

func autor(r *http.Request) string {
	id, _ := auth.IdentityFromContext(r.Context())
	if id.Username != "" {
		return id.Username
	}
	return id.Subject
}

func (h *Handlers) registrar(r *http.Request, acao, recurso, id string, meta map[string]any) {
	e := audit.FromRequest(r)
	e.Action, e.ResourceType, e.ResourceID, e.Metadata = acao, recurso, id, meta
	_ = h.audit.Record(r.Context(), e)
}

func metaConexao(c Conexao) map[string]any {
	return map[string]any{"nome": c.Nome, "provedor": c.Provedor, "endpoint": c.Endpoint, "modelo": c.Modelo,
		"externo": c.Externo, "tem_chave": c.Chave != "", "timeout_segundos": c.TimeoutSegundos}
}

func (h *Handlers) ListarProvedores(w http.ResponseWriter, _ *http.Request) {
	httputil.WriteOK(w, Provedores)
}

func (h *Handlers) Listar(w http.ResponseWriter, r *http.Request) {
	cs, err := h.store.Listar(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]ConexaoPublica, len(cs))
	for i, c := range cs {
		out[i] = c.Publica()
	}
	httputil.WriteOK(w, out)
}

type conexaoRequest struct {
	Nome            string `json:"nome" validate:"required,max=80"`
	Provedor        string `json:"provedor" validate:"required,max=32"`
	Endpoint        string `json:"endpoint" validate:"required,max=500"`
	Modelo          string `json:"modelo" validate:"required,max=120"`
	Chave           string `json:"chave" validate:"max=4096"`
	Externo         bool   `json:"externo"`
	TimeoutSegundos int    `json:"timeout_segundos" validate:"omitempty,min=1,max=300"`
	RemoverChave    bool   `json:"remover_chave"`
}

func (req conexaoRequest) conexao() Conexao {
	return Conexao{Nome: req.Nome, Provedor: Provedor(req.Provedor), Endpoint: req.Endpoint, Modelo: req.Modelo,
		Chave: req.Chave, Externo: req.Externo, TimeoutSegundos: req.TimeoutSegundos}
}

// testarESalvar valida, testa a conexão com a chave que vai valer e só
// grava se o teste passar.
func (h *Handlers) testarESalvar(w http.ResponseWriter, r *http.Request, c Conexao, chaveAtual string, removerChave bool) (Conexao, bool) {
	c.Normalizar()
	efetiva := c.Chave
	if efetiva == "" && !removerChave {
		efetiva = chaveAtual
	}
	if err := c.Validar(efetiva != ""); err != nil {
		h.fail(w, r, err)
		return c, false
	}
	teste := c
	teste.Chave = efetiva
	t := h.cliente.Testar(r.Context(), teste)
	if !t.OK {
		h.fail(w, r, InvalidaError{Msg: "o teste da conexão falhou, nada foi salvo: " + t.Erro})
		return c, false
	}
	salva, err := h.store.Salvar(r.Context(), c, removerChave, autor(r))
	if err == nil {
		err = h.store.RegistrarTeste(r.Context(), salva.ID, t)
	}
	if err != nil {
		h.fail(w, r, err)
		return c, false
	}
	salva.UltimoTeste = &t
	return salva, true
}

func (h *Handlers) Criar(w http.ResponseWriter, r *http.Request) {
	var req conexaoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	c, ok := h.testarESalvar(w, r, req.conexao(), "", false)
	if !ok {
		return
	}
	h.registrar(r, ActionConexaoCriada, resourceConexao, c.ID.String(), metaConexao(c))
	httputil.WriteCreated(w, c.Publica())
}

func (h *Handlers) Alterar(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req conexaoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	atual, err := h.store.Obter(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	c := req.conexao()
	c.ID = id
	// A chave salva nunca segue para outro endereço (seria o jeito de
	// extrair, por um servidor próprio, uma chave que a tela não mostra).
	if req.Chave == "" && !req.RemoverChave && atual.Chave != "" && !mesmoEndpoint(atual.Endpoint, c.Endpoint) {
		h.fail(w, r, InvalidaError{"ao trocar o endereço, informe a chave de API de novo (a atual não é enviada a outro servidor)"})
		return
	}
	salva, ok := h.testarESalvar(w, r, c, atual.Chave, req.RemoverChave)
	if !ok {
		return
	}
	meta := metaConexao(salva)
	meta["antes"] = metaConexao(atual)
	meta["chave_trocada"] = req.Chave != "" || req.RemoverChave
	h.registrar(r, ActionConexaoAlterada, resourceConexao, id.String(), meta)
	httputil.WriteOK(w, salva.Publica())
}

func (h *Handlers) Excluir(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	atual, err := h.store.Obter(r.Context(), id)
	if err == nil {
		err = h.store.Excluir(r.Context(), id)
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.registrar(r, ActionConexaoExcluida, resourceConexao, id.String(), metaConexao(atual))
	w.WriteHeader(http.StatusNoContent)
}

// Testar conversa com a conexão salva e registra o resultado (200 mesmo
// se o teste falhar: o resultado é o conteúdo).
func (h *Handlers) Testar(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	c, err := h.store.Obter(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	t := h.cliente.Testar(r.Context(), c)
	if err := h.store.RegistrarTeste(r.Context(), id, t); err != nil {
		h.fail(w, r, err)
		return
	}
	h.registrar(r, ActionConexaoTestada, resourceConexao, id.String(), map[string]any{"ok": t.OK, "latencia_ms": t.LatenciaMs, "erro": t.Erro})
	httputil.WriteOK(w, t)
}

type usoResponse struct {
	Uso
	// Ambiente: a conexão do .env, que vale enquanto Configurado=false.
	Ambiente *ConexaoPublica `json:"ambiente"`
}

func (h *Handlers) funcao(w http.ResponseWriter, r *http.Request) (string, bool) {
	f := chi.URLParam(r, "funcao")
	if !FuncaoValida(f) {
		h.fail(w, r, apperrors.NotFound("função de IA desconhecida"))
		return "", false
	}
	return f, true
}

func (h *Handlers) responderUso(w http.ResponseWriter, u Uso) {
	resp := usoResponse{Uso: u}
	if h.ambiente != nil {
		p := h.ambiente.Publica()
		resp.Ambiente = &p
	}
	httputil.WriteOK(w, resp)
}

func (h *Handlers) ObterUso(w http.ResponseWriter, r *http.Request) {
	f, ok := h.funcao(w, r)
	if !ok {
		return
	}
	u, err := h.store.Uso(r.Context(), f)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.responderUso(w, u)
}

type usoRequest struct {
	PrincipalID           *uuid.UUID `json:"principal_id"`
	ReservaID             *uuid.UUID `json:"reserva_id"`
	MascararDadosPessoais bool       `json:"mascarar_dados_pessoais"`
	// AutorizoEnvioExterno: confirmação explícita de que a pergunta pode
	// sair para um fornecedor externo (LGPD art. 33).
	AutorizoEnvioExterno bool `json:"autorizo_envio_externo"`
}

// DefinirUso liga a função às conexões (principal nula = IA desligada).
func (h *Handlers) DefinirUso(w http.ResponseWriter, r *http.Request) {
	f, ok := h.funcao(w, r)
	if !ok {
		return
	}
	var req usoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	switch {
	case req.PrincipalID == nil && req.ReservaID != nil:
		h.fail(w, r, InvalidaError{"escolha a conexão principal antes da reserva"})
		return
	case req.PrincipalID != nil && req.ReservaID != nil && *req.PrincipalID == *req.ReservaID:
		h.fail(w, r, InvalidaError{"a reserva precisa ser outra conexão"})
		return
	}
	externa := ""
	for _, id := range []*uuid.UUID{req.PrincipalID, req.ReservaID} {
		if id == nil {
			continue
		}
		c, err := h.store.Obter(r.Context(), *id)
		if errors.Is(err, ErrNaoEncontrada) {
			err = InvalidaError{"conexão inexistente: " + id.String()}
		}
		if err != nil {
			h.fail(w, r, err)
			return
		}
		if c.Externo && externa == "" {
			externa = c.Nome
		}
	}
	u := Uso{Funcao: f, PrincipalID: req.PrincipalID, ReservaID: req.ReservaID, MascararDadosPessoais: req.MascararDadosPessoais}
	if externa != "" {
		if !req.AutorizoEnvioExterno {
			h.fail(w, r, InvalidaError{"\"" + externa + "\" é um fornecedor externo: confirme que as perguntas podem sair da rede da instituição"})
			return
		}
		agora := time.Now()
		u.ExternoAutorizadoPor, u.ExternoAutorizadoEm = autor(r), &agora
	}
	salvo, err := h.store.DefinirUso(r.Context(), u, autor(r))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.registrar(r, ActionUsoAlterado, resourceUso, f, map[string]any{"principal_id": salvo.PrincipalID, "reserva_id": salvo.ReservaID,
		"mascarar_dados_pessoais": salvo.MascararDadosPessoais, "externo_autorizado": externa != ""})
	h.responderUso(w, salvo)
}

// mesmoEndpoint compara endereços ignorando espaços, a barra final e a
// caixa do esquema e do host.
func mesmoEndpoint(a, b string) bool {
	norm := func(s string) string {
		c := Conexao{Endpoint: s}
		c.Normalizar()
		return strings.ToLower(c.Endpoint)
	}
	return norm(a) == norm(b)
}
