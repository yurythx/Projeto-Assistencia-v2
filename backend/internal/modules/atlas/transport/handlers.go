// Package transport expõe a API HTTP do Atlas.
package transport

import (
	"encoding/csv"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/modules/atlas/application"
	"github.com/yurythx/projeto-nexus/internal/modules/atlas/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/httpserver"
	"github.com/yurythx/projeto-nexus/pkg/httputil"
)

// Handlers da API do Atlas.
type Handlers struct {
	svc         *application.Service
	logger      *slog.Logger
	maxPageSize int
}

// NewHandlers cria os handlers.
func NewHandlers(svc *application.Service, logger *slog.Logger, maxPageSize int) *Handlers {
	return &Handlers{svc: svc, logger: logger, maxPageSize: maxPageSize}
}

// RegisterPublicRoutes: consulta anônima (transparência ativa) — só
// procedimentos ativos. O módulo aplica o limite por IP.
func (h *Handlers) RegisterPublicRoutes(r chi.Router) {
	r.Get("/atlas/ttdd", h.ListTTDD)
	r.Get("/atlas/ttdd/estrutura", h.EstruturaTTDD)
	r.Get("/atlas/ttdd/exportar", h.ExportarTTDD)
	r.Get("/atlas/ttdd/{codigo}", h.GetTTDD)
	r.Get("/atlas/ttdd/{codigo}/historico", h.HistoricoTTDD)
	r.Get("/atlas/ttdd/{codigo}/modelos", h.ModelosDaSerie)
	r.Get("/atlas/workflows", h.ListPublic)
	r.Get("/atlas/workflows/secretarias", h.SecretariasPublic)
	r.Get("/atlas/organograma", h.OrganogramaPublic)
	r.Get("/atlas/workflows/{id}", h.GetPublic)
	r.Get("/atlas/modelos", h.ListModelosPublic)
	r.Get("/atlas/modelos/{id}", h.GetModelo)
	r.Get("/atlas/modelos/{id}/arquivo", h.ArquivoModelo)
}

// RegisterRoutes: assistente (atlas:read + limite próprio por identidade,
// pois cada consulta pode acionar o modelo de linguagem) e gestão
// (atlas:manage, concessão global — o catálogo é institucional).
func (h *Handlers) RegisterRoutes(r chi.Router, chatLimiter httpserver.Limiter) {
	r.With(auth.RequirePermission(h.logger, auth.PermAtlasRead), httpserver.RateLimit(h.logger, chatLimiter, identityKey)).
		Post("/atlas/chat", h.Chat)
	// Seguir um procedimento (avisos de nova versão — ADR 027): qualquer
	// pessoa autenticada.
	r.Get("/atlas/workflows/{id}/seguir", h.Seguindo)
	r.Put("/atlas/workflows/{id}/seguir", h.Seguir)
	r.Delete("/atlas/workflows/{id}/seguir", h.DeixarDeSeguir)
	r.Group(func(r chi.Router) {
		r.Use(auth.RequirePermission(h.logger, auth.PermAtlasManage))
		r.Get("/atlas/admin/workflows", h.ListAdmin)
		r.Get("/atlas/admin/workflows/secretarias", h.SecretariasAdmin)
		r.Get("/atlas/admin/organograma", h.OrganogramaAdmin)
		r.Get("/atlas/admin/workflows/exportar", h.ExportarProcedimentos)
		r.Get("/atlas/admin/cobertura", h.Cobertura)
		r.Post("/atlas/admin/workflows/importacao/simular", h.SimularImportacao)
		r.Post("/atlas/admin/workflows/importacao/aplicar", h.AplicarImportacao)
		r.Post("/atlas/admin/workflows", h.Create)
		r.Get("/atlas/admin/workflows/{id}", h.GetAdmin)
		r.Post("/atlas/admin/workflows/{id}/ativar", h.Ativar)
		r.Post("/atlas/admin/workflows/{id}/desativar", h.Desativar)
		r.Post("/atlas/admin/workflows/{id}/versoes", h.NovaVersao)
		r.Post("/atlas/admin/workflows/{id}/situacao", h.MudarSituacao)
		r.Post("/atlas/admin/workflows/{id}/homologar", h.Homologar)
		r.Get("/atlas/admin/workflows/{id}/validacoes", h.Validacoes)
		r.Post("/atlas/admin/workflows/{id}/validacoes", h.RegistrarValidacao)
		r.Put("/atlas/admin/workflows/{id}/pecas/{peca}/modelo", h.LigarModelo)
		r.Get("/atlas/admin/modelos", h.ListModelosAdmin)
		r.Post("/atlas/admin/modelos", h.CriarModelo)
		r.Put("/atlas/admin/modelos/{id}", h.AlterarModelo)
		r.Post("/atlas/admin/modelos/{id}/versoes", h.NovaVersaoModelo)
		r.Post("/atlas/admin/ttdd/{codigo}/modelos", h.LigarModeloSerie)
		r.Delete("/atlas/admin/ttdd/{codigo}/modelos/{modelo}", h.DesligarModeloSerie)
		r.Post("/atlas/admin/ttdd/carga/simular", h.SimularCarga)
		r.Post("/atlas/admin/ttdd/carga/aplicar", h.AplicarCarga)
	})
}

// identityKey limita por identidade: RequirePermission, antes do limite,
// garante que ela existe.
func identityKey(r *http.Request) string {
	identity, _ := auth.IdentityFromContext(r.Context())
	return "atlas-chat:" + identity.Subject
}

func (h *Handlers) fail(w http.ResponseWriter, r *http.Request, err error) {
	httputil.WriteError(w, r, h.logger, application.MapError(err))
}

func (h *Handlers) ListTTDD(w http.ResponseWriter, r *http.Request) {
	p := httputil.Page(r, h.maxPageSize)
	// ?codigo= é um prefixo hierárquico: órgão (2.0), função (2.0.01),
	// subfunção (2.0.01.00) ou a própria série.
	f, err := h.filtroTTDD(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items, total, err := h.svc.ListTTDD(r.Context(), f, p)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WritePage(w, items, p, total)
}

// filtroTTDD lê ?q= e ?codigo= (prefixo hierárquico validado).
func (h *Handlers) filtroTTDD(r *http.Request) (domain.FiltroTTDD, error) {
	codigo := httputil.Query(r, "codigo", 32)
	if codigo != "" && !domain.CodigoTTDDValido(codigo) {
		return domain.FiltroTTDD{}, apperrors.BadRequest("código TTDD inválido (ex.: 2.0, 2.0.01, 2.0.01.00 ou 2.0.01.00.00)")
	}
	return domain.FiltroTTDD{Query: httputil.Query(r, "q", 200), Codigo: codigo}, nil
}

// ExportarTTDD entrega as séries filtradas em CSV (transparência ativa, LAI
// art. 8º §3º II): separador ";" e BOM UTF-8, como o Excel em português espera.
func (h *Handlers) ExportarTTDD(w http.ResponseWriter, r *http.Request) {
	f, err := h.filtroTTDD(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	series, err := h.svc.ExportarTTDD(r.Context(), f)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="ttdd.csv"`)
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF}) // BOM UTF-8
	cw := csv.NewWriter(w)
	cw.Comma = ';'
	_ = cw.Write([]string{"Código", "Série documental", "Órgão", "Função", "Subfunção", "Fase corrente", "Fase intermediária",
		"Destinação final", "Observações", "Recomendação da subfunção", "Fonte"})
	for _, c := range series {
		var orgao, funcao, sub, rec, fonte string
		if s := c.Subfuncao; s != nil {
			orgao, funcao, sub, rec = s.Funcao.Orgao.Nome, s.Funcao.Codigo+" "+s.Funcao.Nome, s.Codigo+" "+s.Nome, s.Recomendacao
			fonte = domain.Fonte(s.Funcao.Orgao)
		}
		_ = cw.Write([]string{c.Codigo, c.Descritor, orgao, funcao, sub,
			domain.Fase(c.FaseCorrenteAnos, c.FaseCorrenteCondicao, true), domain.Fase(c.FaseIntermAnos, c.FaseIntermCondicao, false),
			c.Destinacao(), c.Observacoes, rec, fonte})
	}
	cw.Flush()
}

// EstruturaTTDD devolve órgão > função > subfunção com a contagem de séries.
func (h *Handlers) EstruturaTTDD(w http.ResponseWriter, r *http.Request) {
	e, err := h.svc.EstruturaTTDD(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, e)
}

// codigoSerie lê o {codigo} da rota (400 se não for um código TTDD).
func codigoSerie(r *http.Request) (string, error) {
	codigo := strings.TrimSpace(chi.URLParam(r, "codigo"))
	if !domain.CodigoTTDDValido(codigo) {
		return "", apperrors.BadRequest("código TTDD inválido")
	}
	return codigo, nil
}

func (h *Handlers) GetTTDD(w http.ResponseWriter, r *http.Request) {
	codigo, err := codigoSerie(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	c, err := h.svc.GetTTDD(r.Context(), codigo)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, c)
}

// HistoricoTTDD devolve as mudanças da série (valores anteriores, revogação).
func (h *Handlers) HistoricoTTDD(w http.ResponseWriter, r *http.Request) {
	codigo, err := codigoSerie(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	hist, err := h.svc.HistoricoTTDD(r.Context(), codigo)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, hist)
}

// list lê os filtros: ?q=, ?codigo_ttdd= (série exata), ?prefixo_ttdd=
// (órgão, função ou subfunção — ex.: 2.0 = a secretaria), ?codigo_processual=
// e, só para a gestão, ?situacao= e ?ultima=true (a versão mais recente de
// cada código).
func (h *Handlers) list(w http.ResponseWriter, r *http.Request, incluirInativos bool) {
	p := httputil.Page(r, h.maxPageSize)
	f := domain.Filter{
		Query: httputil.Query(r, "q", 200), CodigoTTDD: httputil.Query(r, "codigo_ttdd", 32),
		CodigoProcessual: httputil.Query(r, "codigo_processual", 64), IncluirInativos: incluirInativos,
		PrefixoTTDD: httputil.Query(r, "prefixo_ttdd", 32), Situacao: strings.ToUpper(httputil.Query(r, "situacao", 16)),
		Ultima: httputil.Query(r, "ultima", 5) == "true",
	}
	if f.PrefixoTTDD != "" && !domain.CodigoTTDDValido(f.PrefixoTTDD) {
		h.fail(w, r, apperrors.BadRequest("prefixo TTDD inválido (ex.: 2.0, 2.0.01 ou 2.0.01.00)"))
		return
	}
	if f.Situacao != "" && !domain.SituacaoValida(f.Situacao) {
		h.fail(w, r, apperrors.BadRequest("situação inválida (RASCUNHO, EM_VALIDACAO ou HOMOLOGADO)"))
		return
	}
	items, total, err := h.svc.List(r.Context(), f, p)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WritePage(w, items, p, total)
}

func (h *Handlers) ListPublic(w http.ResponseWriter, r *http.Request) { h.list(w, r, false) }
func (h *Handlers) ListAdmin(w http.ResponseWriter, r *http.Request)  { h.list(w, r, true) }

// secretarias: procedimentos por secretaria (órgão da TTDD).
func (h *Handlers) secretarias(w http.ResponseWriter, r *http.Request, gestao bool) {
	out, err := h.svc.ProcedimentosPorOrgao(r.Context(), gestao)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, out)
}

func (h *Handlers) SecretariasPublic(w http.ResponseWriter, r *http.Request) {
	h.secretarias(w, r, false)
}

func (h *Handlers) SecretariasAdmin(w http.ResponseWriter, r *http.Request) {
	h.secretarias(w, r, true)
}

// organograma: a TTDD por secretaria, função e subfunção, com contagens.
func (h *Handlers) organograma(w http.ResponseWriter, r *http.Request, gestao bool) {
	out, err := h.svc.Organograma(r.Context(), gestao)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, out)
}

func (h *Handlers) OrganogramaPublic(w http.ResponseWriter, r *http.Request) {
	h.organograma(w, r, false)
}

func (h *Handlers) OrganogramaAdmin(w http.ResponseWriter, r *http.Request) {
	h.organograma(w, r, true)
}

func (h *Handlers) get(w http.ResponseWriter, r *http.Request, incluirInativo bool) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	wf, err := h.svc.Get(r.Context(), id, incluirInativo)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, wf)
}

func (h *Handlers) GetPublic(w http.ResponseWriter, r *http.Request) { h.get(w, r, false) }
func (h *Handlers) GetAdmin(w http.ResponseWriter, r *http.Request)  { h.get(w, r, true) }

type documentoRequest struct {
	NomeDocumento         string `json:"nome_documento" validate:"required,max=150"`
	Obrigatorio           bool   `json:"obrigatorio"`
	Formato               string `json:"formato" validate:"required,oneof=NATO_DIGITAL EXTERNO_DIGITALIZADO"`
	TipoAssinatura        string `json:"tipo_assinatura" validate:"required,oneof=INDIVIDUAL CONJUNTA_MULTINIVEL EM_BLOCO"`
	ExigeConferenciaCopia bool   `json:"exige_conferencia_copia"`
	ModeloMinutaPadraoURL string `json:"modelo_minuta_padrao_url" validate:"omitempty,max=2000"`
	// ModeloID liga a peça a um modelo da biblioteca (ADR 024).
	ModeloID *uuid.UUID `json:"modelo_id"`
}

type transicaoRequest struct {
	DestinoOrdem          int    `json:"destino_ordem" validate:"required,min=1"`
	CondicaoTransicao     string `json:"condicao_transicao" validate:"required,max=255"`
	IsDevolucaoDiligencia bool   `json:"is_devolucao_diligencia"`
	DescricaoDiligencia   string `json:"descricao_diligencia" validate:"max=2000"`
}

type etapaRequest struct {
	Ordem                   int                `json:"ordem" validate:"required,min=1"`
	UnidadeAdministrativa   string             `json:"unidade_administrativa" validate:"required,max=64"`
	NomeSetor               string             `json:"nome_setor" validate:"required,max=150"`
	AtribuicoesSetor        string             `json:"atribuicoes_setor" validate:"required,max=5000"`
	PrazoSLAEmDias          int                `json:"prazo_sla_em_dias" validate:"min=0,max=3650"`
	ManterAbertoAposRemessa bool               `json:"manter_aberto_apos_remessa"`
	Documentos              []documentoRequest `json:"documentos" validate:"max=30,dive"`
	Transicoes              []transicaoRequest `json:"transicoes" validate:"max=10,dive"`
}

type createRequest struct {
	CodigoProcessual string         `json:"codigo_processual" validate:"required,max=64"`
	Versao           int            `json:"versao" validate:"omitempty,min=1"`
	Titulo           string         `json:"titulo" validate:"required,max=255"`
	Objetivo         string         `json:"objetivo" validate:"required,max=5000"`
	PublicoAlvo      string         `json:"publico_alvo" validate:"required,max=255"`
	NivelAcesso      string         `json:"nivel_acesso" validate:"required,oneof=PUBLICO RESTRITO SIGILOSO"`
	HipoteseLegal    string         `json:"hipotese_legal_restricao" validate:"max=255"`
	CodigoTTDD       string         `json:"codigo_ttdd" validate:"required,max=32"`
	Etapas           []etapaRequest `json:"etapas" validate:"required,min=1,max=50,dive"`
	// Rascunho grava para validação, sem publicar (ADR 028).
	Rascunho bool `json:"rascunho"`
}

func (req createRequest) toDomain() domain.Workflow {
	w := domain.Workflow{
		CodigoProcessual: req.CodigoProcessual, Versao: req.Versao, Titulo: req.Titulo, Objetivo: req.Objetivo,
		PublicoAlvo: req.PublicoAlvo, NivelAcesso: domain.NivelAcesso(req.NivelAcesso), HipoteseLegal: req.HipoteseLegal,
		CodigoTTDD: req.CodigoTTDD,
	}
	if req.Rascunho {
		w.Situacao = domain.SituacaoRascunho
	}
	for _, e := range req.Etapas {
		etapa := domain.Etapa{Ordem: e.Ordem, UnidadeAdministrativa: e.UnidadeAdministrativa, NomeSetor: e.NomeSetor,
			AtribuicoesSetor: e.AtribuicoesSetor, PrazoSLAEmDias: e.PrazoSLAEmDias, ManterAbertoAposRemessa: e.ManterAbertoAposRemessa}
		for _, d := range e.Documentos {
			etapa.Documentos = append(etapa.Documentos, domain.EtapaDocumento{NomeDocumento: d.NomeDocumento, Obrigatorio: d.Obrigatorio,
				Formato: domain.FormatoDocumento(d.Formato), TipoAssinatura: domain.TipoAssinatura(d.TipoAssinatura),
				ExigeConferenciaCopia: d.ExigeConferenciaCopia, ModeloMinutaPadraoURL: d.ModeloMinutaPadraoURL, ModeloID: d.ModeloID})
		}
		for _, t := range e.Transicoes {
			etapa.Transicoes = append(etapa.Transicoes, domain.EtapaTransicao{DestinoOrdem: t.DestinoOrdem,
				CondicaoTransicao: t.CondicaoTransicao, IsDevolucaoDiligencia: t.IsDevolucaoDiligencia, DescricaoDiligencia: t.DescricaoDiligencia})
		}
		w.Etapas = append(w.Etapas, etapa)
	}
	return w
}

func (h *Handlers) Create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	identity, _ := auth.IdentityFromContext(r.Context())
	wf, err := h.svc.Create(r.Context(), identity, req.toDomain())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, wf)
}

// NovaVersao cria a versão seguinte do procedimento {id} com o conteúdo
// enviado (o código processual e a versão vêm do original) e desativa as
// demais versões.
func (h *Handlers) NovaVersao(w http.ResponseWriter, r *http.Request) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req createRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	identity, _ := auth.IdentityFromContext(r.Context())
	wf, err := h.svc.NovaVersao(r.Context(), identity, id, req.toDomain())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteCreated(w, wf)
}

func (h *Handlers) setAtivo(w http.ResponseWriter, r *http.Request, ativo bool) {
	id, err := httputil.UUIDParam(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	wf, err := h.svc.SetAtivo(r.Context(), id, ativo)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, wf)
}

func (h *Handlers) Ativar(w http.ResponseWriter, r *http.Request)    { h.setAtivo(w, r, true) }
func (h *Handlers) Desativar(w http.ResponseWriter, r *http.Request) { h.setAtivo(w, r, false) }

type chatRequest struct {
	Query string `json:"query" validate:"required,min=3,max=500"`
}

func (h *Handlers) Chat(w http.ResponseWriter, r *http.Request) {
	var req chatRequest
	if err := httputil.Bind(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	resp, err := h.svc.Perguntar(r.Context(), req.Query)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, resp)
}

// maxCarga limita o arquivo da TTDD (a tabela inteira tem ~600 KB).
const maxCarga = 10 << 20

type cargaRequest struct {
	Formato  string `json:"formato" validate:"required,oneof=csv json"`
	Conteudo string `json:"conteudo" validate:"required"`
	// Hash devolvido pela simulação: aplicar exige o mesmo arquivo.
	Hash string `json:"hash" validate:"omitempty,len=64"`
}

func (h *Handlers) carga(w http.ResponseWriter, r *http.Request, aplicar bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxCarga)
	var req cargaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.fail(w, r, apperrors.BadRequest("corpo inválido ou acima de 10 MB"))
		return
	}
	if err := httputil.Validate(req); err != nil {
		h.fail(w, r, err)
		return
	}
	out, err := h.svc.ImpactoTTDD(r.Context(), req.Formato, req.Conteudo, req.Hash, aplicar)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httputil.WriteOK(w, out)
}

// SimularCarga mostra o que a nova TTDD muda, sem gravar (ADR 022).
func (h *Handlers) SimularCarga(w http.ResponseWriter, r *http.Request) { h.carga(w, r, false) }

// AplicarCarga grava a nova TTDD (o mesmo arquivo simulado — hash).
func (h *Handlers) AplicarCarga(w http.ResponseWriter, r *http.Request) { h.carga(w, r, true) }
