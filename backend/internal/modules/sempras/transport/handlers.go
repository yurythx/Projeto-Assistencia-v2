// Package transport expõe o SEMPRAS via HTTP. A rota exige a permissão em
// algum lugar (RequirePermission); o serviço decide sobre a unidade do
// recurso (auth.Can — ADR 013).
package transport

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

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
	gestao := auth.RequirePermission(h.logger, auth.PermSemprasGestao)

	r.Get("/sempras/unidades", h.Unidades)
	r.With(gestao).Get("/sempras/unidades/configuracao", h.UnidadesParaConfigurar)
	r.With(gestao).Put("/sempras/unidades/{id}/configuracao", h.ConfigurarUnidade)
	r.With(gestao).Get("/sempras/painel", h.Painel)
	r.With(gestao).Get("/sempras/relatorios/rma", h.Rma)

	r.With(recepcao).Get("/sempras/cidadaos", h.Buscar)
	r.With(recepcao).Post("/sempras/cidadaos", h.Cadastrar)
	r.With(recepcao).Get("/sempras/cidadaos/{id}", h.Cidadao)
	r.Put("/sempras/cidadaos/{id}", h.Atualizar) // os 4 perfis podem editar — o gate é no serviço (AreaDe), não é de uma unidade só
	r.With(atendimento).Get("/sempras/cidadaos/{id}/prontuario", h.Prontuario)
	r.With(atendimento).Post("/sempras/cidadaos/{id}/registros", h.RegistrarEvolucao)

	r.With(atendimento).Get("/sempras/cidadaos/{id}/pedidos/acesso", h.SolicitarAcesso)
	r.With(atendimento).Post("/sempras/cidadaos/{id}/pedidos", h.SolicitarPedido)
	r.With(gestao).Get("/sempras/pedidos", h.ListarPedidos)
	r.With(gestao).Post("/sempras/pedidos/{id}/decidir", h.DecidirPedido)

	r.With(atendimento).Post("/sempras/cidadaos/{id}/rede-apoio", h.AdicionarContatoRedeApoio)
	r.With(atendimento).Delete("/sempras/rede-apoio/{id}", h.RemoverContatoRedeApoio)

	r.With(atendimento).Post("/sempras/cidadaos/{id}/beneficios", h.SolicitarBeneficio)
	r.With(atendimento).Post("/sempras/beneficios/{id}/decidir", h.DecidirBeneficio)

	r.With(atendimento).Post("/sempras/cidadaos/{id}/encaminhamentos", h.Encaminhar)
	r.With(atendimento).Get("/sempras/unidades/{id}/encaminhamentos", h.EncaminhamentosPendentes)
	r.With(atendimento).Post("/sempras/encaminhamentos/{id}/decidir", h.DecidirEncaminhamento)

	r.With(atendimento).Post("/sempras/cidadaos/{id}/paif", h.IncluirPaif)
	r.With(atendimento).Post("/sempras/paif/{id}/excluir", h.ExcluirPaif)

	r.With(atendimento).Post("/sempras/cidadaos/{id}/conselho-tutelar", h.RegistrarConselhoAtendimento)
	r.With(atendimento).Post("/sempras/conselho-tutelar/{id}/transitar", h.TransitarConselhoAtendimento)

	r.With(atendimento).Post("/sempras/cidadaos/{id}/paefi", h.RegistrarPaefiCaso)
	r.With(atendimento).Post("/sempras/paefi/{id}/pia", h.MarcarPiaElaborado)
	r.With(atendimento).Post("/sempras/paefi/{id}/encerrar", h.EncerrarPaefiCaso)
	r.With(atendimento).Post("/sempras/unidades/{id}/abordagens", h.RegistrarAbordagem)
	r.With(atendimento).Get("/sempras/unidades/{id}/abordagens", h.ListarAbordagens)
	r.With(atendimento).Post("/sempras/cidadaos/{id}/casa-mulher", h.RegistrarCasoCasaMulher)
	r.With(atendimento).Put("/sempras/casa-mulher/{id}", h.AtualizarCasoCasaMulher)
	r.With(atendimento).Post("/sempras/casa-mulher/{id}/encerrar", h.EncerrarCasoCasaMulher)

	r.With(atendimento).Post("/sempras/cidadaos/{id}/mse", h.RegistrarMseMedida)
	r.With(atendimento).Post("/sempras/mse/{id}/concluir", h.ConcluirMseMedida)

	r.With(atendimento).Post("/sempras/cidadaos/{id}/creas-notificacoes", h.RegistrarNotificacaoCreas)
	r.With(atendimento).Post("/sempras/creas-notificacoes/{id}/transitar", h.TransitarNotificacaoCreas)

	r.With(atendimento).Post("/sempras/cidadaos/{id}/oficios-cras", h.RegistrarOficioCras)
	r.With(atendimento).Post("/sempras/oficios-cras/{id}/responder", h.ResponderOficioCras)

	r.With(atendimento).Post("/sempras/unidades/{id}/coletivos", h.RegistrarAtendimentoColetivo)
	r.With(atendimento).Get("/sempras/unidades/{id}/coletivos", h.ListarAtendimentosColetivos)

	r.With(atendimento).Post("/sempras/cidadaos/{id}/casa-abrigo", h.RegistrarAcolhimento)
	r.With(atendimento).Put("/sempras/casa-abrigo/{id}", h.AtualizarAcolhimento)
	r.With(atendimento).Post("/sempras/casa-abrigo/{id}/desligar", h.DesligarAcolhimento)
	r.With(atendimento).Post("/sempras/casa-abrigo/{id}/irmaos", h.AdicionarIrmao)
	r.With(atendimento).Delete("/sempras/casa-abrigo-irmaos/{id}", h.RemoverIrmao)
	r.With(atendimento).Post("/sempras/casa-abrigo/{id}/diagnosticos", h.AdicionarDiagnostico)
	r.With(atendimento).Delete("/sempras/casa-abrigo-diagnosticos/{id}", h.RemoverDiagnostico)
	r.With(atendimento).Post("/sempras/casa-abrigo/{id}/visitantes", h.AdicionarVisitante)
	r.With(atendimento).Delete("/sempras/casa-abrigo-visitantes/{id}", h.RemoverVisitante)
	r.With(atendimento).Post("/sempras/casa-abrigo/{id}/termos", h.RegistrarTermo)
	r.With(atendimento).Delete("/sempras/casa-abrigo-termos/{id}", h.RemoverTermo)
	r.With(atendimento).Put("/sempras/casa-abrigo/{id}/pia", h.SalvarPiaFicha)
	r.With(atendimento).Post("/sempras/casa-abrigo/{id}/pia-metas", h.AdicionarPiaMeta)
	r.With(atendimento).Post("/sempras/casa-abrigo-pia-metas/{id}/concluir", h.ConcluirPiaMeta)
	r.With(atendimento).Delete("/sempras/casa-abrigo-pia-metas/{id}", h.RemoverPiaMeta)
	r.With(atendimento).Post("/sempras/casa-abrigo/{id}/documentos/uploads", h.StartUploadDocumento)
	r.With(atendimento).Post("/sempras/casa-abrigo-documentos/{id}/confirm", h.ConfirmUploadDocumento)
	r.With(atendimento).Get("/sempras/casa-abrigo-documentos/{id}/download", h.DownloadDocumento)
	r.With(atendimento).Delete("/sempras/casa-abrigo-documentos/{id}", h.RemoverDocumento)

	r.With(atendimento).Put("/sempras/cidadaos/{id}/centro-pop-perfil", h.AtualizarCentroPopPerfil)
	r.With(atendimento).Post("/sempras/cidadaos/{id}/encaminhamentos-externos", h.RegistrarEncaminhamentoExterno)
	r.With(atendimento).Post("/sempras/encaminhamentos-externos/{id}/transitar", h.TransitarEncaminhamentoExterno)
	r.With(atendimento).Post("/sempras/cidadaos/{id}/centro-pop-documentos/uploads", h.StartUploadDocumentoCentroPop)
	r.With(atendimento).Post("/sempras/centro-pop-documentos/{id}/confirm", h.ConfirmUploadDocumentoCentroPop)
	r.With(atendimento).Get("/sempras/centro-pop-documentos/{id}/download", h.DownloadDocumentoCentroPop)
	r.With(atendimento).Delete("/sempras/centro-pop-documentos/{id}", h.RemoverDocumentoCentroPop)

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

// Rma devolve o RMA/MDS das unidades de quem gere, numa competência
// (?competencia=AAAA-MM; padrão: mês atual).
func (h *Handlers) Rma(w http.ResponseWriter, r *http.Request) {
	competencia := httputil.Query(r, "competencia", 10)
	if competencia == "" {
		competencia = time.Now().Format("2006-01")
	}
	out, err := h.svc.Rma(r.Context(), identity(r), competencia)
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

// CidadaoRequest é o cadastro simples do munícipe. Anonimo (Centro POP —
// abordagem de rua) dispensa CPF/NIS/nascimento/mãe, nunca o nome.
type CidadaoRequest struct {
	Nome           string `json:"nome" validate:"required,max=200"`
	NomeSocial     string `json:"nome_social" validate:"max=200"`
	CPF            string `json:"cpf" validate:"max=20"`
	NIS            string `json:"nis" validate:"max=20"`
	DataNascimento string `json:"data_nascimento" validate:"max=10"`
	Sexo           string `json:"sexo" validate:"omitempty,oneof=feminino masculino"`
	NomeMae        string `json:"nome_mae" validate:"max=200"`
	Telefone       string `json:"telefone" validate:"max=30"`
	Endereco       string `json:"endereco" validate:"max=300"`
	Anonimo        bool   `json:"anonimo"`
}

func (req CidadaoRequest) input() domain.CidadaoInput {
	return domain.CidadaoInput{Nome: req.Nome, NomeSocial: req.NomeSocial, CPF: req.CPF, NIS: req.NIS,
		DataNascimento: req.DataNascimento, NomeMae: req.NomeMae, Telefone: req.Telefone, Endereco: req.Endereco,
		Sexo: req.Sexo, Anonimo: req.Anonimo}
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
	// Demanda: uma das demandas da unidade (GET /sempras/unidades);
	// "outra" exige o detalhe.
	Demanda        string `json:"demanda" validate:"required,max=40"`
	DemandaDetalhe string `json:"demanda_detalhe" validate:"max=200"`
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
		CidadaoID: req.CidadaoID, Tipo: req.Tipo, Prioridade: req.Prioridade, Demanda: req.Demanda, DemandaDetalhe: req.DemandaDetalhe})
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

// SolicitarAcesso atende o direito de acesso (art. 18, II): devolve o
// prontuário do titular para download, já registrando o pedido concluído.
func (h *Handlers) SolicitarAcesso(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	p, err := h.svc.SolicitarAcesso(r.Context(), identity(r), id, httputil.Query(r, "justificativa", 1000))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	filename := fmt.Sprintf("dados-municipe-%s.json", time.Now().UTC().Format("20060102_150405"))
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Cache-Control", "no-store")
	httputil.WriteJSON(w, http.StatusOK, p, nil)
}

// PedidoRequest é a correção ou a anonimização pedidas pelo titular.
type PedidoRequest struct {
	Tipo          string `json:"tipo" validate:"required,oneof=correcao anonimizacao"`
	Justificativa string `json:"justificativa" validate:"required,max=1000"`
}

// SolicitarPedido registra correção ou anonimização a pedido do titular.
func (h *Handlers) SolicitarPedido(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req PedidoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	p, err := h.svc.SolicitarPedido(r.Context(), identity(r), id, req.Tipo, req.Justificativa)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, p)
}

// ListarPedidos devolve a fila de pedidos pendentes (anonimização).
func (h *Handlers) ListarPedidos(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListarPedidosPendentes(r.Context(), identity(r))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, out)
}

// DecisaoRequest é a decisão da gestão sobre um pedido de anonimização.
type DecisaoRequest struct {
	Aprovar       bool   `json:"aprovar"`
	Justificativa string `json:"justificativa" validate:"max=1000"`
}

// DecidirPedido aprova ou nega uma anonimização pendente.
func (h *Handlers) DecidirPedido(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req DecisaoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	p, err := h.svc.DecidirPedido(r.Context(), identity(r), id, req.Aprovar, req.Justificativa)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, p)
}

// ContatoRedeApoioRequest é um contato da rede de apoio do munícipe.
type ContatoRedeApoioRequest struct {
	Nome       string `json:"nome" validate:"required,max=200"`
	Parentesco string `json:"parentesco" validate:"max=100"`
	Telefone   string `json:"telefone" validate:"max=30"`
}

// AdicionarContatoRedeApoio inclui uma pessoa de apoio do munícipe.
func (h *Handlers) AdicionarContatoRedeApoio(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req ContatoRedeApoioRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	c, err := h.svc.AdicionarContatoRedeApoio(r.Context(), identity(r), id, req.Nome, req.Parentesco, req.Telefone)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, c)
}

// RemoverContatoRedeApoio remove um contato da rede de apoio.
func (h *Handlers) RemoverContatoRedeApoio(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.RemoverContatoRedeApoio(r.Context(), identity(r), id); err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, map[string]bool{"removido": true})
}

// BeneficioRequest é o pedido de um benefício eventual.
type BeneficioRequest struct {
	SolicitanteID *uuid.UUID `json:"solicitante_id"`
	UnidadeID     uuid.UUID  `json:"unidade_id" validate:"required"`
	Tipo          string     `json:"tipo" validate:"required,oneof=natalidade funeral cesta_basica passagem aluguel_social outro auxilio_gas cobertor_agasalho"`
	FormaAcesso   string     `json:"forma_acesso" validate:"required,oneof=demanda_espontanea busca_ativa encaminhamento_rede_socioassistencial encaminhamento_sistema_garantia_direitos"`
	Quantidade    int        `json:"quantidade" validate:"required,min=1,max=100"`
	Observacao    string     `json:"observacao" validate:"max=2000"`
}

// SolicitarBeneficio registra um pedido de benefício eventual.
func (h *Handlers) SolicitarBeneficio(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req BeneficioRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	b, err := h.svc.SolicitarBeneficio(r.Context(), identity(r), id, req.SolicitanteID, req.UnidadeID, req.Tipo, req.FormaAcesso, req.Quantidade, req.Observacao)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, b)
}

// DecisaoBeneficioRequest é a decisão sobre um benefício eventual.
type DecisaoBeneficioRequest struct {
	Acao   string `json:"acao" validate:"required,oneof=conceder negar suspender"`
	Motivo string `json:"motivo" validate:"max=1000"`
}

// DecidirBeneficio concede, nega ou suspende um benefício eventual.
func (h *Handlers) DecidirBeneficio(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req DecisaoBeneficioRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	b, err := h.svc.DecidirBeneficio(r.Context(), identity(r), id, req.Acao, req.Motivo)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, b)
}

// EncaminhamentoRequest é um caso encaminhado de uma unidade a outra.
type EncaminhamentoRequest struct {
	UnidadeOrigemID  uuid.UUID `json:"unidade_origem_id" validate:"required"`
	UnidadeDestinoID uuid.UUID `json:"unidade_destino_id" validate:"required"`
	Motivo           string    `json:"motivo" validate:"required,max=2000"`
}

// Encaminhar passa o caso de uma unidade a outra.
func (h *Handlers) Encaminhar(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req EncaminhamentoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	e, err := h.svc.Encaminhar(r.Context(), identity(r), id, req.UnidadeOrigemID, req.UnidadeDestinoID, req.Motivo)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, e)
}

// EncaminhamentosPendentes devolve a fila de casos recebidos pela unidade.
func (h *Handlers) EncaminhamentosPendentes(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out, err := h.svc.EncaminhamentosPendentes(r.Context(), identity(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, out)
}

// DecisaoEncaminhamentoRequest é a decisão sobre um encaminhamento recebido.
type DecisaoEncaminhamentoRequest struct {
	Aceitar    bool   `json:"aceitar"`
	Observacao string `json:"observacao" validate:"max=1000"`
}

// DecidirEncaminhamento aceita ou recusa um encaminhamento recebido.
func (h *Handlers) DecidirEncaminhamento(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req DecisaoEncaminhamentoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	e, err := h.svc.DecidirEncaminhamento(r.Context(), identity(r), id, req.Aceitar, req.Observacao)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, e)
}

// PaifRequest é a inclusão de uma família no PAIF.
type PaifRequest struct {
	UnidadeID       uuid.UUID `json:"unidade_id" validate:"required"`
	TipoAtendimento string    `json:"tipo_atendimento" validate:"required,oneof=particularizado coletivo ambos"`
	ExtremaPobreza  bool      `json:"extrema_pobreza"`
	BolsaFamilia    bool      `json:"bolsa_familia"`
	BPC             bool      `json:"bpc"`
	SCFV            int       `json:"scfv_participantes" validate:"min=0,max=50"`
}

// IncluirPaif vincula a família ao PAIF do CRAS.
func (h *Handlers) IncluirPaif(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req PaifRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	v, err := h.svc.IncluirPaif(r.Context(), identity(r), id, req.UnidadeID, req.TipoAtendimento, req.ExtremaPobreza, req.BolsaFamilia, req.BPC, req.SCFV)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, v)
}

// ExclusaoPaifRequest é a exclusão de um vínculo PAIF.
type ExclusaoPaifRequest struct {
	Motivo string `json:"motivo" validate:"required,max=1000"`
}

// ExcluirPaif encerra um vínculo PAIF ativo.
func (h *Handlers) ExcluirPaif(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req ExclusaoPaifRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	v, err := h.svc.ExcluirPaif(r.Context(), identity(r), id, req.Motivo)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, v)
}

// ConselhoAtendimentoRequest é a abertura de um atendimento do Conselho Tutelar.
type ConselhoAtendimentoRequest struct {
	UnidadeID           uuid.UUID `json:"unidade_id" validate:"required"`
	ResponsavelNome     string    `json:"responsavel_nome" validate:"max=200"`
	ResponsavelCPF      string    `json:"responsavel_cpf" validate:"max=20"`
	ResponsavelTelefone string    `json:"responsavel_telefone" validate:"max=30"`
	OrigemDemanda       string    `json:"origem_demanda" validate:"required,oneof=disque_100 escola unidade_saude plantao_24h demanda_espontanea policia"`
	MotivoViolacao      string    `json:"motivo_violacao" validate:"required,oneof=negligencia_abandono violencia_fisica abuso_exploracao_sexual evasao_escolar trabalho_infantil conflito_familiar"`
	Observacoes         string    `json:"observacoes" validate:"max=4000"`
}

// RegistrarConselhoAtendimento abre um atendimento do Conselho Tutelar.
func (h *Handlers) RegistrarConselhoAtendimento(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req ConselhoAtendimentoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	c, err := h.svc.RegistrarConselhoAtendimento(r.Context(), identity(r), id, req.UnidadeID, req.ResponsavelNome,
		req.ResponsavelCPF, req.ResponsavelTelefone, req.OrigemDemanda, req.MotivoViolacao, req.Observacoes)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, c)
}

// TransicaoConselhoRequest é a decisão sobre um atendimento do Conselho Tutelar.
type TransicaoConselhoRequest struct {
	Acao   string `json:"acao" validate:"required,oneof=aplicar_medida encaminhar_mp arquivar"`
	Medida string `json:"medida" validate:"max=50"`
}

// TransitarConselhoAtendimento aplica medida, encaminha ao MP ou arquiva.
func (h *Handlers) TransitarConselhoAtendimento(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req TransicaoConselhoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	c, err := h.svc.TransitarConselhoAtendimento(r.Context(), identity(r), id, req.Acao, req.Medida)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, c)
}

// PaefiCasoRequest é a abertura de um caso do PAEFI.
type PaefiCasoRequest struct {
	UnidadeID          uuid.UUID `json:"unidade_id" validate:"required"`
	Violacoes          []string  `json:"violacoes" validate:"required,min=1,dive,oneof=violencia_fisica violencia_psicologica violencia_sexual negligencia abandono trabalho_infantil discriminacao medida_socioeducativa_meio_aberto"`
	MembrosVulneraveis string    `json:"membros_vulneraveis" validate:"max=2000"`
	Observacoes        string    `json:"observacoes" validate:"max=4000"`
}

// RegistrarPaefiCaso abre um caso do PAEFI para o munícipe.
func (h *Handlers) RegistrarPaefiCaso(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req PaefiCasoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	p, err := h.svc.RegistrarPaefiCaso(r.Context(), identity(r), id, req.UnidadeID, req.Violacoes, req.MembrosVulneraveis, req.Observacoes)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, p)
}

// MarcarPiaElaborado registra a elaboração do PIA de um caso do PAEFI.
func (h *Handlers) MarcarPiaElaborado(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	p, err := h.svc.MarcarPiaElaborado(r.Context(), identity(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, p)
}

// EncerramentoPaefiRequest é o desligamento de um caso do PAEFI.
type EncerramentoPaefiRequest struct {
	Motivo string `json:"motivo" validate:"required,oneof=objetivos_alcancados mudanca_territorio nao_localizado a_pedido_da_familia outro"`
}

// EncerrarPaefiCaso desliga o caso do PAEFI.
func (h *Handlers) EncerrarPaefiCaso(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req EncerramentoPaefiRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	p, err := h.svc.EncerrarPaefiCaso(r.Context(), identity(r), id, req.Motivo)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, p)
}

// AbordagemRequest é o registro de uma abordagem social (Centro POP).
type AbordagemRequest struct {
	CidadaoID         *uuid.UUID `json:"cidadao_id"`
	DescricaoPessoa   string     `json:"descricao_pessoa" validate:"max=300"`
	Local             string     `json:"local" validate:"required,min=3,max=300"`
	DataAbordagem     time.Time  `json:"data_abordagem" validate:"required"`
	Resultado         string     `json:"resultado" validate:"required,oneof=aceitou_acolhimento aceitou_encaminhamento orientado recusou"`
	UsoDrogasAparente bool       `json:"uso_drogas_aparente"`
	DemandaSaude      bool       `json:"demanda_saude"`
	Observacoes       string     `json:"observacoes" validate:"max=2000"`
}

// RegistrarAbordagem registra uma abordagem social da unidade.
func (h *Handlers) RegistrarAbordagem(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req AbordagemRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	a, err := h.svc.RegistrarAbordagem(r.Context(), identity(r), id, domain.AbordagemInput{CidadaoID: req.CidadaoID,
		DescricaoPessoa: req.DescricaoPessoa, Local: req.Local, Resultado: req.Resultado, Obs: req.Observacoes,
		DataAbordagem: req.DataAbordagem, UsoDrogasAparente: req.UsoDrogasAparente, DemandaSaude: req.DemandaSaude})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, a)
}

// ListarAbordagens lista as abordagens da unidade no mês (?mes=2026-10,
// padrão o mês atual).
func (h *Handlers) ListarAbordagens(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	mes := httputil.Query(r, "mes", 10)
	if mes == "" {
		mes = time.Now().Format("2006-01")
	}
	out, err := h.svc.ListarAbordagens(r.Context(), identity(r), id, mes)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, out)
}

// ConfiguracaoUnidadeRequest é o tipo da unidade (ADR 031); tipo vazio tira
// a configuração e a unidade deixa de atender.
type ConfiguracaoUnidadeRequest struct {
	Tipo            string `json:"tipo" validate:"omitempty,oneof=cras scfv creas centro_pop casa_mulher casa_abrigo conselho_tutelar"`
	PainelExibeNome bool   `json:"painel_exibe_nome"`
}

// UnidadesParaConfigurar lista as unidades da SEMPRAS da gestão, com e sem tipo.
func (h *Handlers) UnidadesParaConfigurar(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.UnidadesParaConfigurar(r.Context(), identity(r))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, out)
}

// ConfigurarUnidade define o tipo da unidade e o painel de chamada.
func (h *Handlers) ConfigurarUnidade(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req ConfiguracaoUnidadeRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	u, err := h.svc.ConfigurarUnidade(r.Context(), identity(r), id, req.Tipo, req.PainelExibeNome)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, u)
}

// CasoCasaMulherRequest é a avaliação de um caso da Casa da Mulher (abertura
// e reavaliação; unidade_id só na abertura).
type CasoCasaMulherRequest struct {
	UnidadeID               uuid.UUID `json:"unidade_id"`
	Violencias              []string  `json:"violencias" validate:"required,min=1,max=5,dive,oneof=fisica psicologica sexual patrimonial moral"`
	RelacaoAgressor         string    `json:"relacao_agressor" validate:"required,oneof=conjuge_companheiro ex_conjuge_companheiro namorado ex_namorado familiar outro"`
	NivelRisco              string    `json:"nivel_risco" validate:"required,oneof=baixo medio alto extremo"`
	MedidaProtetivaStatus   string    `json:"medida_protetiva_status" validate:"required,oneof=nao_solicitada em_analise deferida indeferida descumprida"`
	MedidaProtetivaProcesso string    `json:"medida_protetiva_processo" validate:"max=60"`
	PatrulhaMariaDaPenha    bool      `json:"patrulha_maria_da_penha"`
	Filhos                  int       `json:"filhos" validate:"min=0,max=30"`
	Abrigada                bool      `json:"abrigada"`
	Observacoes             string    `json:"observacoes" validate:"max=4000"`
}

func (req CasoCasaMulherRequest) input() domain.CasoCasaMulherInput {
	return domain.CasoCasaMulherInput{Violencias: req.Violencias, RelacaoAgressor: req.RelacaoAgressor, NivelRisco: req.NivelRisco,
		MedidaProtetivaStatus: req.MedidaProtetivaStatus, MedidaProtetivaProcesso: req.MedidaProtetivaProcesso,
		PatrulhaMariaDaPenha: req.PatrulhaMariaDaPenha, Filhos: req.Filhos, Abrigada: req.Abrigada, Observacoes: req.Observacoes}
}

// RegistrarCasoCasaMulher abre o caso de acompanhamento da mulher.
func (h *Handlers) RegistrarCasoCasaMulher(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req CasoCasaMulherRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	c, err := h.svc.RegistrarCasoCasaMulher(r.Context(), identity(r), id, req.UnidadeID, req.input())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, c)
}

// AtualizarCasoCasaMulher regrava a avaliação de um caso ativo.
func (h *Handlers) AtualizarCasoCasaMulher(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req CasoCasaMulherRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	c, err := h.svc.AtualizarCasoCasaMulher(r.Context(), identity(r), id, req.input())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, c)
}

// EncerramentoCasaMulherRequest é o encerramento de um caso da Casa da Mulher.
type EncerramentoCasaMulherRequest struct {
	Motivo string `json:"motivo" validate:"required,oneof=superacao_violencia mudanca_municipio transferencia_rede desistencia outro"`
}

// EncerrarCasoCasaMulher fecha o caso.
func (h *Handlers) EncerrarCasoCasaMulher(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req EncerramentoCasaMulherRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	c, err := h.svc.EncerrarCasoCasaMulher(r.Context(), identity(r), id, req.Motivo)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, c)
}

// MseMedidaRequest é o registro de uma medida socioeducativa em meio aberto.
type MseMedidaRequest struct {
	UnidadeID           uuid.UUID `json:"unidade_id" validate:"required"`
	NumeroProcesso      string    `json:"numero_processo" validate:"max=100"`
	VaraOrigem          string    `json:"vara_origem" validate:"max=200"`
	AtoInfracional      string    `json:"ato_infracional" validate:"required,min=3,max=500"`
	TipoMedida          string    `json:"tipo_medida" validate:"required,oneof=liberdade_assistida prestacao_servico_comunidade"`
	InstituicaoParceira string    `json:"instituicao_parceira" validate:"max=200"`
}

// RegistrarMseMedida abre o acompanhamento de uma medida socioeducativa.
func (h *Handlers) RegistrarMseMedida(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req MseMedidaRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	m, err := h.svc.RegistrarMseMedida(r.Context(), identity(r), id, req.UnidadeID, req.NumeroProcesso, req.VaraOrigem,
		req.AtoInfracional, req.TipoMedida, req.InstituicaoParceira)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, m)
}

// ConclusaoMseRequest é o desfecho de uma medida socioeducativa.
type ConclusaoMseRequest struct {
	Resultado string `json:"resultado" validate:"required,oneof=cumprida descumprida extinta"`
}

// ConcluirMseMedida encerra o acompanhamento de uma medida socioeducativa.
func (h *Handlers) ConcluirMseMedida(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req ConclusaoMseRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	m, err := h.svc.ConcluirMseMedida(r.Context(), identity(r), id, req.Resultado)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, m)
}

// NotificacaoCreasRequest é o recebimento de um ofício/requisição do CREAS.
type NotificacaoCreasRequest struct {
	UnidadeID      uuid.UUID  `json:"unidade_id" validate:"required"`
	OrigemEntidade string     `json:"origem_entidade" validate:"required,oneof=ministerio_publico poder_judiciario conselho_tutelar disque_100_180 delegacia outro"`
	Assunto        string     `json:"assunto" validate:"required,min=3,max=2000"`
	PrazoResposta  *time.Time `json:"prazo_resposta"`
}

// RegistrarNotificacaoCreas registra uma notificação recebida pelo CREAS.
func (h *Handlers) RegistrarNotificacaoCreas(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req NotificacaoCreasRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	n, err := h.svc.RegistrarNotificacaoCreas(r.Context(), identity(r), id, req.UnidadeID, req.OrigemEntidade, req.Assunto, req.PrazoResposta)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, n)
}

// TransicaoNotificacaoRequest é a decisão sobre uma notificação do CREAS.
type TransicaoNotificacaoRequest struct {
	Acao     string `json:"acao" validate:"required,oneof=iniciar_diligencia responder arquivar"`
	Resposta string `json:"resposta" validate:"max=4000"`
}

// TransitarNotificacaoCreas inicia diligência, responde ou arquiva a notificação.
func (h *Handlers) TransitarNotificacaoCreas(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req TransicaoNotificacaoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	n, err := h.svc.TransitarNotificacaoCreas(r.Context(), identity(r), id, req.Acao, req.Resposta)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, n)
}

// OficioCrasRequest é o recebimento de um ofício pelo CRAS.
type OficioCrasRequest struct {
	UnidadeID    uuid.UUID  `json:"unidade_id" validate:"required"`
	OrgaoEmissor string     `json:"orgao_emissor" validate:"required,min=3,max=200"`
	NumeroOficio string     `json:"numero_oficio" validate:"required,max=100"`
	DataOficio   *time.Time `json:"data_oficio"`
	Assunto      string     `json:"assunto" validate:"max=2000"`
}

// RegistrarOficioCras registra um ofício recebido pelo CRAS.
func (h *Handlers) RegistrarOficioCras(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req OficioCrasRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	o, err := h.svc.RegistrarOficioCras(r.Context(), identity(r), id, req.UnidadeID, req.OrgaoEmissor, req.NumeroOficio, req.DataOficio, req.Assunto)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, o)
}

// ResponderOficioRequest é a resposta a um ofício pendente do CRAS.
type ResponderOficioRequest struct {
	NumeroResposta string `json:"numero_resposta" validate:"required,max=100"`
}

// ResponderOficioCras registra a resposta de um ofício do CRAS.
func (h *Handlers) ResponderOficioCras(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req ResponderOficioRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	o, err := h.svc.ResponderOficioCras(r.Context(), identity(r), id, req.NumeroResposta)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, o)
}

// AtendimentoColetivoRequest é o registro de um evento de grupo (SCFV).
type AtendimentoColetivoRequest struct {
	DataEvento    time.Time `json:"data_evento" validate:"required"`
	Categoria     string    `json:"categoria" validate:"required,oneof=paif_familias scfv_0_6 scfv_7_14 scfv_15_17 scfv_18_59 scfv_idosos palestras_nao_continuado pcd"`
	Participantes int       `json:"participantes" validate:"min=0,max=1000"`
	Descricao     string    `json:"descricao" validate:"max=2000"`
}

// RegistrarAtendimentoColetivo registra um evento de grupo (SCFV) da unidade.
func (h *Handlers) RegistrarAtendimentoColetivo(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req AtendimentoColetivoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	a, err := h.svc.RegistrarAtendimentoColetivo(r.Context(), identity(r), id, req.DataEvento, req.Categoria, req.Participantes, req.Descricao)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, a)
}

// ListarAtendimentosColetivos lista os eventos de grupo de uma unidade
// no mês (?mes=2026-10, padrão o mês atual).
func (h *Handlers) ListarAtendimentosColetivos(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	mes := httputil.Query(r, "mes", 10)
	if mes == "" {
		mes = time.Now().Format("2006-01")
	}
	out, err := h.svc.ListarAtendimentosColetivos(r.Context(), identity(r), id, mes)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, out)
}

// AcolhimentoRequest é o cadastro ou a edição de um acolhimento da Casa
// Abrigo — a edição nunca inclui status/motivo/data de desligamento
// (isso só muda por DesligarAcolhimento, RN-14).
type AcolhimentoRequest struct {
	UnidadeID                      uuid.UUID `json:"unidade_id" validate:"required"`
	Sexo                           string    `json:"sexo" validate:"required,oneof=masculino feminino"`
	Motivos                        []string  `json:"motivos" validate:"required,min=1,dive,oneof=negligencia_abandono violencia_fisica violencia_psicologica abuso_sexual orfandade_ausencia_responsavel usuario_drogas_responsavel situacao_rua trabalho_infantil outro"`
	ResponsavelNome                string    `json:"responsavel_nome" validate:"max=200"`
	ResponsavelNIS                 string    `json:"responsavel_nis" validate:"max=20"`
	ResponsavelTelefone            string    `json:"responsavel_telefone" validate:"max=30"`
	ResponsavelVinculo             string    `json:"responsavel_vinculo" validate:"max=100"`
	ResponsavelEndereco            string    `json:"responsavel_endereco" validate:"max=300"`
	ProcessoJudicial               string    `json:"processo_judicial" validate:"max=200"`
	GuiaAcolhimento                string    `json:"guia_acolhimento" validate:"max=2000"`
	ConselheiroNome                string    `json:"conselheiro_nome" validate:"max=200"`
	ConselheiroTelefone            string    `json:"conselheiro_telefone" validate:"max=30"`
	SituacaoJuridica               string    `json:"situacao_juridica" validate:"max=500"`
	HistoricoAcolhidoAnteriormente bool      `json:"historico_acolhido_anteriormente"`
	HistoricoIrmaosOutraUnidade    bool      `json:"historico_irmaos_outra_unidade"`
	Observacoes                    string    `json:"observacoes_gerais" validate:"max=4000"`
}

func (req AcolhimentoRequest) input() application.AcolhimentoInput {
	return application.AcolhimentoInput{
		Sexo: req.Sexo, Motivos: req.Motivos, ResponsavelNome: req.ResponsavelNome, ResponsavelNIS: req.ResponsavelNIS,
		ResponsavelTelefone: req.ResponsavelTelefone, ResponsavelVinculo: req.ResponsavelVinculo, ResponsavelEndereco: req.ResponsavelEndereco,
		ProcessoJudicial: req.ProcessoJudicial, GuiaAcolhimento: req.GuiaAcolhimento, ConselheiroNome: req.ConselheiroNome,
		ConselheiroTelefone: req.ConselheiroTelefone, SituacaoJuridica: req.SituacaoJuridica,
		HistoricoAcolhidoAnteriormente: req.HistoricoAcolhidoAnteriormente, HistoricoIrmaosOutraUnidade: req.HistoricoIrmaosOutraUnidade,
		Observacoes: req.Observacoes,
	}
}

// RegistrarAcolhimento abre um acolhimento institucional na Casa Abrigo.
func (h *Handlers) RegistrarAcolhimento(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req AcolhimentoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	a, err := h.svc.RegistrarAcolhimento(r.Context(), identity(r), id, req.UnidadeID, req.input())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, a)
}

// AtualizarAcolhimento regrava os campos descritivos do acolhimento.
func (h *Handlers) AtualizarAcolhimento(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req AcolhimentoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	a, err := h.svc.AtualizarAcolhimento(r.Context(), identity(r), id, req.input())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, a)
}

// DesligamentoRequest é o desligamento de um acolhimento.
type DesligamentoRequest struct {
	Motivo string `json:"motivo" validate:"required,min=3,max=2000"`
}

// DesligarAcolhimento encerra o acolhimento (RN-14: equipe técnica).
func (h *Handlers) DesligarAcolhimento(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req DesligamentoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	a, err := h.svc.DesligarAcolhimento(r.Context(), identity(r), id, req.Motivo)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, a)
}

// IrmaoRequest é um irmão do acolhido (RN-07).
type IrmaoRequest struct {
	Nome             string     `json:"nome" validate:"required,min=2,max=200"`
	DataNascimento   *time.Time `json:"data_nascimento"`
	EstaAcolhido     bool       `json:"esta_acolhido"`
	UnidadeDescricao string     `json:"unidade_descricao" validate:"max=200"`
	Observacao       string     `json:"observacao" validate:"max=1000"`
}

// AdicionarIrmao registra um irmão do acolhido.
func (h *Handlers) AdicionarIrmao(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req IrmaoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	i, err := h.svc.AdicionarIrmao(r.Context(), identity(r), id, req.Nome, req.DataNascimento, req.EstaAcolhido, req.UnidadeDescricao, req.Observacao)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, i)
}

// RemoverIrmao remove um irmão registrado por engano.
func (h *Handlers) RemoverIrmao(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.RemoverIrmao(r.Context(), identity(r), id); err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, map[string]bool{"removido": true})
}

// DiagnosticoRequest é um diagnóstico de saúde/deficiência/transtorno do acolhido (RN-12).
type DiagnosticoRequest struct {
	Categoria   string `json:"categoria" validate:"required,oneof=doenca deficiencia transtorno outro"`
	Diagnostico string `json:"diagnostico" validate:"required,min=2,max=255"`
	Descricao   string `json:"descricao" validate:"max=2000"`
}

// AdicionarDiagnostico registra um diagnóstico do acolhido.
func (h *Handlers) AdicionarDiagnostico(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req DiagnosticoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	d, err := h.svc.AdicionarDiagnostico(r.Context(), identity(r), id, req.Categoria, req.Diagnostico, req.Descricao)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, d)
}

// RemoverDiagnostico remove um diagnóstico registrado por engano.
func (h *Handlers) RemoverDiagnostico(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.RemoverDiagnostico(r.Context(), identity(r), id); err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, map[string]bool{"removido": true})
}

// VisitanteRequest é uma visita recebida pelo acolhido.
type VisitanteRequest struct {
	NomeVisitante       string    `json:"nome_visitante" validate:"required,min=2,max=200"`
	DocumentoIdentidade string    `json:"documento_identidade" validate:"max=30"`
	ParentescoVinculo   string    `json:"parentesco_vinculo" validate:"max=100"`
	DataVisita          time.Time `json:"data_visita" validate:"required"`
	HoraEntrada         string    `json:"hora_entrada" validate:"max=5"`
	HoraSaida           string    `json:"hora_saida" validate:"max=5"`
	Observacoes         string    `json:"observacoes" validate:"max=1000"`
}

// AdicionarVisitante registra uma visita recebida pelo acolhido.
func (h *Handlers) AdicionarVisitante(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req VisitanteRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	v, err := h.svc.AdicionarVisitante(r.Context(), identity(r), id, req.NomeVisitante, req.DocumentoIdentidade,
		req.ParentescoVinculo, req.DataVisita, req.HoraEntrada, req.HoraSaida, req.Observacoes)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, v)
}

// RemoverVisitante remove uma visita registrada por engano.
func (h *Handlers) RemoverVisitante(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.RemoverVisitante(r.Context(), identity(r), id); err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, map[string]bool{"removido": true})
}

// TermoRequest é um termo de advertência, passeio ou saída.
type TermoRequest struct {
	TipoTermo             string    `json:"tipo_termo" validate:"required,oneof=advertencia passeio saida outro"`
	ResponsavelRetirada   string    `json:"responsavel_retirada" validate:"max=200"`
	ParentescoResponsavel string    `json:"parentesco_responsavel" validate:"max=100"`
	MotivoObservacao      string    `json:"motivo_observacao" validate:"max=2000"`
	DataTermo             time.Time `json:"data_termo" validate:"required"`
}

// RegistrarTermo registra um termo de advertência, passeio ou saída.
func (h *Handlers) RegistrarTermo(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req TermoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	t, err := h.svc.RegistrarTermo(r.Context(), identity(r), id, req.TipoTermo, req.ResponsavelRetirada,
		req.ParentescoResponsavel, req.MotivoObservacao, req.DataTermo)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, t)
}

// RemoverTermo remove um termo registrado por engano.
func (h *Handlers) RemoverTermo(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.RemoverTermo(r.Context(), identity(r), id); err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, map[string]bool{"removido": true})
}

// PiaFichaRequest é a ficha 1:1 do PIA (saúde/família/rotina).
type PiaFichaRequest struct {
	MatriculaEscolar    string `json:"matricula_escolar" validate:"max=100"`
	SerieEscolar        string `json:"serie_escolar" validate:"max=100"`
	Escola              string `json:"escola" validate:"max=200"`
	Medicamentos        string `json:"medicamentos" validate:"max=2000"`
	Alergias            string `json:"alergias" validate:"max=2000"`
	VacinacaoAtualizada bool   `json:"vacinacao_atualizada"`
	CartaoSUS           string `json:"cartao_sus" validate:"max=30"`
	ContatoFamiliar     string `json:"contato_familiar" validate:"max=2000"`
	VisitasAutorizadas  string `json:"visitas_autorizadas" validate:"max=2000"`
	HistoricoFamiliar   string `json:"historico_familiar" validate:"max=4000"`
	AtividadesManha     string `json:"atividades_manha" validate:"max=1000"`
	AtividadesTarde     string `json:"atividades_tarde" validate:"max=1000"`
	AtividadesNoite     string `json:"atividades_noite" validate:"max=1000"`
	AtividadesFimSemana string `json:"atividades_fim_semana" validate:"max=1000"`
}

// SalvarPiaFicha grava ou atualiza a ficha do PIA.
func (h *Handlers) SalvarPiaFicha(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req PiaFichaRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	p, err := h.svc.SalvarPiaFicha(r.Context(), identity(r), id, application.PiaFichaInput{
		MatriculaEscolar: req.MatriculaEscolar, SerieEscolar: req.SerieEscolar, Escola: req.Escola,
		Medicamentos: req.Medicamentos, Alergias: req.Alergias, VacinacaoAtualizada: req.VacinacaoAtualizada,
		CartaoSUS: req.CartaoSUS, ContatoFamiliar: req.ContatoFamiliar, VisitasAutorizadas: req.VisitasAutorizadas,
		HistoricoFamiliar: req.HistoricoFamiliar, AtividadesManha: req.AtividadesManha, AtividadesTarde: req.AtividadesTarde,
		AtividadesNoite: req.AtividadesNoite, AtividadesFimSemana: req.AtividadesFimSemana,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, p)
}

// PiaMetaRequest é uma meta do plano ou da intervenção do PIA.
type PiaMetaRequest struct {
	Tipo        string     `json:"tipo" validate:"required,oneof=plano intervencao"`
	Descricao   string     `json:"descricao" validate:"required,min=3,max=1000"`
	Providencia string     `json:"providencia" validate:"max=1000"`
	Responsavel string     `json:"responsavel" validate:"max=200"`
	Prazo       *time.Time `json:"prazo"`
}

// AdicionarPiaMeta registra uma meta do plano ou da intervenção do PIA.
func (h *Handlers) AdicionarPiaMeta(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req PiaMetaRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	m, err := h.svc.AdicionarPiaMeta(r.Context(), identity(r), id, req.Tipo, req.Descricao, req.Providencia, req.Responsavel, req.Prazo)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, m)
}

// ConclusaoPiaMetaRequest é a (des)conclusão de uma meta do PIA.
type ConclusaoPiaMetaRequest struct {
	Concluida bool `json:"concluida"`
}

// ConcluirPiaMeta marca (ou desmarca) uma meta do PIA como concluída.
func (h *Handlers) ConcluirPiaMeta(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req ConclusaoPiaMetaRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	m, err := h.svc.ConcluirPiaMeta(r.Context(), identity(r), id, req.Concluida)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, m)
}

// RemoverPiaMeta remove uma meta registrada por engano.
func (h *Handlers) RemoverPiaMeta(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.RemoverPiaMeta(r.Context(), identity(r), id); err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, map[string]bool{"removido": true})
}

// UploadDocumentoRequest inicia o upload de um documento do acolhido ou do responsável.
type UploadDocumentoRequest struct {
	Tipo        string `json:"tipo" validate:"required,oneof=acolhido responsavel"`
	Filename    string `json:"filename" validate:"required,max=255"`
	ContentType string `json:"content_type" validate:"required,max=150"`
	Size        int64  `json:"size" validate:"required,min=1"`
}

// StartUploadDocumento cria o metadado pendente e a URL de upload direto ao object storage.
func (h *Handlers) StartUploadDocumento(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req UploadDocumentoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	res, err := h.svc.StartUploadDocumento(r.Context(), identity(r), id, req.Tipo, req.Filename, req.ContentType, req.Size)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, res)
}

// ConfirmUploadDocumento confere o objeto enviado e publica o documento.
func (h *Handlers) ConfirmUploadDocumento(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	d, err := h.svc.ConfirmUploadDocumento(r.Context(), identity(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, d)
}

// DownloadDocumento devolve uma URL pré-assinada de leitura (curta).
func (h *Handlers) DownloadDocumento(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	url, d, err := h.svc.DownloadURLDocumento(r.Context(), identity(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httputil.WriteOK(w, map[string]any{"url": url, "documento": d})
}

// RemoverDocumento remove o documento (metadado + objeto no storage).
func (h *Handlers) RemoverDocumento(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.RemoverDocumento(r.Context(), identity(r), id); err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, map[string]bool{"removido": true})
}

// CentroPopPerfilRequest é o perfil de rua do munícipe (Centro POP).
type CentroPopPerfilRequest struct {
	UnidadeID             uuid.UUID `json:"unidade_id" validate:"required"`
	Situacao              string    `json:"situacao" validate:"required,oneof=situacao_rua abrigado_institucionalmente egresso_sistema_prisional trecheiro albergado outro"`
	TempoRuaAnos          *float64  `json:"tempo_rua_anos" validate:"omitempty,min=0"`
	MotivoVulnerabilidade string    `json:"motivo_vulnerabilidade" validate:"required,oneof=conflito_familiar uso_de_drogas alcoolismo desemprego perda_de_moradia egresso_sistema_prisional migracao escolha_propria outro"`
	PossuiDoenca          bool      `json:"possui_doenca"`
	DoencaDescricao       string    `json:"doenca_descricao" validate:"max=1000"`
	UsaMedicacao          bool      `json:"usa_medicacao"`
	MedicacaoDescricao    string    `json:"medicacao_descricao" validate:"max=1000"`
	UsoDrogas             bool      `json:"uso_drogas"`
	DrogasDescricao       string    `json:"drogas_descricao" validate:"max=1000"`
}

// AtualizarCentroPopPerfil grava (upsert) o perfil de rua.
func (h *Handlers) AtualizarCentroPopPerfil(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req CentroPopPerfilRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	p, err := h.svc.AtualizarCentroPopPerfil(r.Context(), identity(r), id, req.UnidadeID, req.Situacao, req.TempoRuaAnos,
		req.MotivoVulnerabilidade, req.PossuiDoenca, req.DoencaDescricao, req.UsaMedicacao, req.MedicacaoDescricao,
		req.UsoDrogas, req.DrogasDescricao)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, p)
}

// EncaminhamentoExternoRequest é o registro de um encaminhamento pra
// fora do sistema.
type EncaminhamentoExternoRequest struct {
	UnidadeID  uuid.UUID `json:"unidade_id" validate:"required"`
	Destino    string    `json:"destino" validate:"required,oneof=caps_ad caps_iii albergue_noturno abrigo_municipal saude_upa_hospital sine_qualificacao conselho_tutelar policia_civil_deam outro"`
	Prioridade string    `json:"prioridade" validate:"required,oneof=baixa media alta urgente"`
	Motivo     string    `json:"motivo" validate:"required,min=3,max=2000"`
}

// RegistrarEncaminhamentoExterno registra o encaminhamento.
func (h *Handlers) RegistrarEncaminhamentoExterno(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req EncaminhamentoExternoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	e, err := h.svc.RegistrarEncaminhamentoExterno(r.Context(), identity(r), id, req.UnidadeID, req.Destino, req.Prioridade, req.Motivo)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, e)
}

// TransicaoEncaminhamentoExternoRequest é a mudança de status de um
// encaminhamento externo.
type TransicaoEncaminhamentoExternoRequest struct {
	Status      string `json:"status" validate:"required,oneof=pendente em_atendimento concluido cancelado"`
	Observacoes string `json:"observacoes" validate:"max=2000"`
}

// TransitarEncaminhamentoExterno muda o status do encaminhamento.
func (h *Handlers) TransitarEncaminhamentoExterno(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req TransicaoEncaminhamentoExternoRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	e, err := h.svc.TransitarEncaminhamentoExterno(r.Context(), identity(r), id, req.Status, req.Observacoes)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, e)
}

// UploadDocumentoCentroPopRequest é o pedido de upload de um documento
// do Centro POP.
type UploadDocumentoCentroPopRequest struct {
	UnidadeID   uuid.UUID `json:"unidade_id" validate:"required"`
	Tipo        string    `json:"tipo" validate:"required,oneof=identificacao laudo_saude outro"`
	Filename    string    `json:"filename" validate:"required,max=255"`
	ContentType string    `json:"content_type" validate:"required,max=150"`
	Size        int64     `json:"size" validate:"required,min=1"`
}

// StartUploadDocumentoCentroPop cria o metadado pendente e a URL de upload direto ao object storage.
func (h *Handlers) StartUploadDocumentoCentroPop(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req UploadDocumentoCentroPopRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	res, err := h.svc.StartUploadDocumentoCentroPop(r.Context(), identity(r), id, req.UnidadeID, req.Tipo, req.Filename, req.ContentType, req.Size)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, res)
}

// ConfirmUploadDocumentoCentroPop confere o objeto enviado e publica o documento.
func (h *Handlers) ConfirmUploadDocumentoCentroPop(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	d, err := h.svc.ConfirmUploadDocumentoCentroPop(r.Context(), identity(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, d)
}

// DownloadDocumentoCentroPop devolve uma URL pré-assinada de leitura (curta).
func (h *Handlers) DownloadDocumentoCentroPop(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	url, d, err := h.svc.DownloadURLDocumentoCentroPop(r.Context(), identity(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httputil.WriteOK(w, map[string]any{"url": url, "documento": d})
}

// RemoverDocumentoCentroPop remove o documento (metadado + objeto no storage).
func (h *Handlers) RemoverDocumentoCentroPop(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.RemoverDocumentoCentroPop(r.Context(), identity(r), id); err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, map[string]bool{"removido": true})
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
