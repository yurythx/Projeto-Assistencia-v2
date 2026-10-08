// Package application contém os casos de uso do Atlas.
package application

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/atlas/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/audit"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/notificacoes"
	"github.com/yurythx/projeto-nexus/internal/platform/outbox"
	"github.com/yurythx/projeto-nexus/internal/platform/storage"
)

// Eventos emitidos.
const (
	EventWorkflowCreated     = "atlas.workflow.created"
	EventWorkflowActivated   = "atlas.workflow.activated"
	EventWorkflowDeactivated = "atlas.workflow.deactivated"
)

// Grounding: quantos candidatos o índice entrega (de cada tipo) e quantas
// fontes, no máximo, sustentam uma resposta.
const (
	maxCandidatos = 10
	maxFontes     = 3
	// margemFontes: além da melhor fonte, só entram as que ficam até 0,15
	// abaixo dela — fontes que só passaram do limiar por palavras genéricas
	// ("documentos", "legislação") não poluem a resposta.
	margemFontes = 0.15
)

// Service implementa os casos de uso.
type Service struct {
	pool       *pgxpool.Pool
	repo       domain.Repository
	outbox     *outbox.Writer
	assistente domain.Assistente     // nil = síntese canônica, sem IA
	avisos     *notificacoes.Service // entrega em tempo real dos avisos (WithNotificacoes)
	store      storage.Provider      // arquivos da biblioteca de modelos (WithStorage)
	bucket     string
	logger     *slog.Logger
	now        func() time.Time
}

// NewService cria o serviço. assistente pode ser nil (IA desligada).
func NewService(pool *pgxpool.Pool, repo domain.Repository, ob *outbox.Writer, assistente domain.Assistente, logger *slog.Logger) *Service {
	return &Service{pool: pool, repo: repo, outbox: ob, assistente: assistente, logger: logger, now: time.Now}
}

// MapError traduz erros de domínio.
func MapError(err error) error {
	var inv domain.InvalidError
	switch {
	case errors.As(err, &inv):
		return apperrors.Validation(inv.Msg)
	case errors.Is(err, domain.ErrNotFound):
		return apperrors.NotFound("procedimento não encontrado")
	case errors.Is(err, domain.ErrTTDDNotFound):
		return apperrors.NotFound("classificação TTDD não encontrada")
	case errors.Is(err, domain.ErrDuplicate):
		return apperrors.Conflict("já existe procedimento com este código e versão")
	}
	return err
}

// ListTTDD lista a Tabela de Temporalidade.
func (s *Service) ListTTDD(ctx context.Context, f domain.FiltroTTDD, p pagination.Params) ([]domain.ClassificacaoTTDD, int64, error) {
	items, total, err := s.repo.ListTTDD(ctx, s.pool, f, p)
	return items, total, MapError(err)
}

// maxExportacao limita a exportação (a TTDD inteira tem ~1.700 séries).
const maxExportacao = 20000

// ExportarTTDD devolve todas as séries que atendem ao filtro (para CSV),
// percorrendo as páginas no teto da paginação.
func (s *Service) ExportarTTDD(ctx context.Context, f domain.FiltroTTDD) ([]domain.ClassificacaoTTDD, error) {
	var out []domain.ClassificacaoTTDD
	for page := 1; len(out) < maxExportacao; page++ {
		p := pagination.New(page, pagination.AbsoluteMaxPageSize, pagination.AbsoluteMaxPageSize)
		items, total, err := s.repo.ListTTDD(ctx, s.pool, f, p)
		if err != nil {
			return nil, MapError(err)
		}
		out = append(out, items...)
		if int64(len(out)) >= total || len(items) == 0 {
			break
		}
	}
	return out, nil
}

// EstruturaTTDD devolve a árvore órgão > função > subfunção da TTDD.
func (s *Service) EstruturaTTDD(ctx context.Context) ([]domain.EstruturaTTDD, error) {
	return s.repo.EstruturaTTDD(ctx, s.pool)
}

// ProcedimentosPorOrgao: procedimentos de cada secretaria (órgão da TTDD).
// Rascunhos e em validação só aparecem para a gestão.
func (s *Service) ProcedimentosPorOrgao(ctx context.Context, gestao bool) ([]domain.ProcedimentosOrgao, error) {
	out, err := s.repo.ProcedimentosPorOrgao(ctx, s.pool)
	if !gestao {
		for i := range out {
			out[i].EmValidacao, out[i].Rascunhos = 0, 0
		}
	}
	return out, MapError(err)
}

// Organograma da TTDD com as contagens de cada nível. Rascunhos e em
// validação só para a gestão.
func (s *Service) Organograma(ctx context.Context, gestao bool) ([]domain.OrgaoOrganograma, error) {
	out, err := s.repo.Organograma(ctx, s.pool)
	if err != nil {
		return nil, MapError(err)
	}
	return domain.MontarOrganograma(out, gestao), nil
}

// GetTTDD devolve uma classificação.
func (s *Service) GetTTDD(ctx context.Context, codigo string) (domain.ClassificacaoTTDD, error) {
	c, err := s.repo.GetTTDD(ctx, s.pool, codigo)
	return c, MapError(err)
}

// HistoricoTTDD devolve as mudanças da série (prazos anteriores, revogação
// e restabelecimento), da mais recente à mais antiga.
func (s *Service) HistoricoTTDD(ctx context.Context, codigo string) ([]domain.HistoricoTTDD, error) {
	if _, err := s.repo.GetTTDD(ctx, s.pool, codigo); err != nil {
		return nil, MapError(err)
	}
	h, err := s.repo.HistoricoTTDD(ctx, s.pool, codigo)
	return h, MapError(err)
}

// List lista procedimentos (inativos só com f.IncluirInativos — a rota de
// gestão exige atlas:manage).
func (s *Service) List(ctx context.Context, f domain.Filter, p pagination.Params) ([]domain.Workflow, int64, error) {
	items, total, err := s.repo.List(ctx, s.pool, f, p)
	return items, total, MapError(err)
}

// Get devolve o procedimento completo. Desativado só aparece para a gestão.
func (s *Service) Get(ctx context.Context, id uuid.UUID, incluirInativo bool) (domain.Workflow, error) {
	w, err := s.repo.Get(ctx, s.pool, id, false)
	if err == nil && !w.Ativo && !incluirInativo {
		err = domain.ErrNotFound
	}
	return w, MapError(err)
}

// Search alimenta a Busca Global.
func (s *Service) Search(ctx context.Context, query string, limit int) ([]domain.Workflow, []float64, error) {
	return s.repo.Search(ctx, s.pool, query, limit)
}

// Create cadastra um procedimento (atlas:manage exigido na rota) com
// evento e auditoria na mesma transação.
func (s *Service) Create(ctx context.Context, identity auth.Identity, w domain.Workflow) (domain.Workflow, error) {
	w.Normalize()
	if w.Versao == 0 {
		w.Versao = 1
	}
	if err := w.Validate(); err != nil {
		return domain.Workflow{}, MapError(err)
	}
	var out domain.Workflow
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) (err error) {
		out, err = s.inserir(ctx, tx, identity, w, nil)
		return err
	})
	return out, MapError(err)
}

// NovaVersao cadastra a versão seguinte do procedimento `origem` (mesmo
// código processual, conteúdo de w) e desativa as outras versões ativas,
// tudo na mesma transação: a consulta pública nunca vê duas versões
// vigentes nem nenhuma.
func (s *Service) NovaVersao(ctx context.Context, identity auth.Identity, origem uuid.UUID, w domain.Workflow) (domain.Workflow, error) {
	var (
		out      domain.Workflow
		enviadas []notificacoes.Enviada
	)
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		base, err := s.repo.Get(ctx, tx, origem, true)
		if err != nil {
			return err
		}
		out, enviadas, err = s.novaVersaoTx(ctx, tx, identity, base, w)
		return err
	})
	if err == nil {
		s.entregar(ctx, enviadas)
	}
	return out, MapError(err)
}

// novaVersaoTx publica, na transação, a versão seguinte de base com o
// conteúdo de w e desativa as demais versões (eventos e auditoria), e
// grava os avisos da versão nova (entregues pelo chamador depois do commit).
func (s *Service) novaVersaoTx(ctx context.Context, tx pgx.Tx, identity auth.Identity, base, w domain.Workflow) (domain.Workflow, []notificacoes.Enviada, error) {
	maior, err := s.repo.MaxVersao(ctx, tx, base.CodigoProcessual)
	if err != nil {
		return w, nil, err
	}
	w.Normalize()
	w.CodigoProcessual, w.Versao = base.CodigoProcessual, maior+1
	if err := w.Validate(); err != nil {
		return w, nil, err
	}
	out, err := s.inserir(ctx, tx, identity, w, modelosDe(base))
	if err != nil || !out.Ativo {
		// Rascunho: não substitui a versão em vigor nem avisa (ADR 028).
		return out, nil, err
	}
	ids, err := s.repo.DesativarVersoes(ctx, tx, out.CodigoProcessual, out.ID)
	if err != nil {
		return out, nil, err
	}
	for _, id := range ids {
		antiga, err := s.repo.Get(ctx, tx, id, false)
		if err != nil {
			return out, nil, err
		}
		if err := s.substituida(ctx, tx, antiga, out); err != nil {
			return out, nil, err
		}
	}
	enviadas, err := s.avisarNovaVersao(ctx, tx, identity, base, out)
	return out, enviadas, err
}

// substituida registra (evento e auditoria) a versão desativada por `nova`.
func (s *Service) substituida(ctx context.Context, tx pgx.Tx, antiga, nova domain.Workflow) error {
	// O evento leva a versão que substituiu: o Trâmite avisa os processos
	// abertos que seguem a antiga (ADR 027).
	payload := summary(antiga)
	payload["substituido_por"], payload["versao_nova"] = nova.ID.String(), nova.Versao
	if err := s.outbox.Write(ctx, tx, EventWorkflowDeactivated, "atlas_workflow", antiga.ID.String(), uuid.Nil, payload); err != nil {
		return err
	}
	return audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, EventWorkflowDeactivated, "atlas_workflow", antiga.ID.String(),
		map[string]any{"ativo": true}, map[string]any{"ativo": false, "substituido_por": nova.ID.String(), "versao": nova.Versao}))
}

// inserir grava o procedimento (já validado) ativo, com evento e auditoria,
// na transação: a série da TTDD tem de existir e estar vigente, e as peças
// só apontam para modelos ativos (ou já ligados à versão anterior).
func (s *Service) inserir(ctx context.Context, tx pgx.Tx, identity auth.Identity, w domain.Workflow, modelosAnteriores map[uuid.UUID]bool) (domain.Workflow, error) {
	// Sem situação, o cadastro pela tela publica direto (homologado); a
	// importação como rascunho chega com RASCUNHO e fica inativa (ADR 028).
	if w.Situacao == "" {
		w.Situacao = domain.SituacaoHomologado
	}
	w.ID, w.Ativo = uuid.New(), w.Situacao == domain.SituacaoHomologado
	if identity.UserID != uuid.Nil {
		uid := identity.UserID
		w.CreatedBy = &uid
	}
	for i := range w.Etapas {
		w.Etapas[i].ID = uuid.New()
		for j := range w.Etapas[i].Documentos {
			w.Etapas[i].Documentos[j].ID = uuid.New()
		}
		for j := range w.Etapas[i].Transicoes {
			w.Etapas[i].Transicoes[j].ID = uuid.New()
		}
	}
	situacao, err := s.repo.LockTTDD(ctx, tx, w.CodigoTTDD)
	if err != nil {
		return w, err
	}
	switch situacao {
	case domain.TTDDInexistente:
		return w, domain.InvalidError{Msg: "código TTDD inexistente na Tabela de Temporalidade: " + w.CodigoTTDD}
	case domain.TTDDRevogada:
		return w, domain.InvalidError{Msg: "a série " + w.CodigoTTDD + " foi revogada na TTDD em vigor: enquadre o procedimento numa série vigente"}
	}
	if err := s.conferirModelos(ctx, tx, w, modelosAnteriores); err != nil {
		return w, err
	}
	if err := s.repo.Insert(ctx, tx, w); err != nil {
		return w, err
	}
	out, err := s.repo.Get(ctx, tx, w.ID, false)
	if err != nil {
		return w, err
	}
	if err := s.event(ctx, tx, EventWorkflowCreated, out); err != nil {
		return w, err
	}
	return out, audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, EventWorkflowCreated, "atlas_workflow", out.ID.String(), nil, summary(out)))
}

// SetAtivo ativa ou desativa um procedimento (atlas:manage na rota).
// Repetir o estado atual não regrava nem emite evento.
func (s *Service) SetAtivo(ctx context.Context, id uuid.UUID, ativo bool) (domain.Workflow, error) {
	var out domain.Workflow
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		before, err := s.repo.Get(ctx, tx, id, true)
		if err != nil {
			return err
		}
		if before.Ativo == ativo {
			out = before
			return nil
		}
		if ativo && before.Situacao != domain.SituacaoHomologado {
			return domain.InvalidError{Msg: "procedimento em validação: homologue-o para publicar"}
		}
		if err := s.repo.SetAtivo(ctx, tx, id, ativo); err != nil {
			return err
		}
		if out, err = s.repo.Get(ctx, tx, id, false); err != nil {
			return err
		}
		event := EventWorkflowDeactivated
		if ativo {
			event = EventWorkflowActivated
		}
		if err := s.event(ctx, tx, event, out); err != nil {
			return err
		}
		return audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, event, "atlas_workflow", id.String(),
			map[string]any{"ativo": before.Ativo}, map[string]any{"ativo": out.Ativo}))
	})
	return out, MapError(err)
}

func (s *Service) event(ctx context.Context, tx pgx.Tx, eventType string, w domain.Workflow) error {
	return s.outbox.Write(ctx, tx, eventType, "atlas_workflow", w.ID.String(), uuid.Nil, summary(w))
}

func summary(w domain.Workflow) map[string]any {
	return map[string]any{"id": w.ID.String(), "codigo_processual": w.CodigoProcessual, "versao": w.Versao,
		"titulo": w.Titulo, "codigo_ttdd": w.CodigoTTDD, "nivel_acesso": w.NivelAcesso, "ativo": w.Ativo, "etapas": len(w.Etapas)}
}

// Tipos de fonte do assistente: um fluxo homologado (procedimento) ou uma
// série da TTDD (ADRs 021 e 023).
const (
	FonteProcedimento = "procedimento"
	FonteTTDD         = "ttdd"
)

// Fonte é o procedimento ou a série da TTDD que sustentou a resposta.
type Fonte struct {
	Tipo       string     `json:"tipo"`
	ID         *uuid.UUID `json:"id,omitempty"`
	Codigo     string     `json:"codigo"`
	Titulo     string     `json:"titulo"`
	Relevancia float64    `json:"relevancia"`
}

// Modos de resposta do assistente.
const (
	ModoIA       = "ia"       // redigida pelo modelo de linguagem
	ModoSintese  = "sintese"  // síntese canônica (IA desligada ou indisponível)
	ModoRecusada = "recusada" // fora do objetivo ou sem fonte acima do limiar
)

// Resposta do assistente.
type Resposta struct {
	Answer      string    `json:"answer"`
	Score       float64   `json:"score"`
	Refused     bool      `json:"refused"`
	Mode        string    `json:"mode"`
	Sources     []Fonte   `json:"sources"`
	GeneratedAt time.Time `json:"generated_at"`
}

// recusar devolve a recusa canônica: fora do objetivo da IA, ou — se a
// pergunta fala de fluxo ou de temporalidade — sem fonte correspondente.
// Coincidência fraca de palavras com uma fonte não torna o assunto válido.
func recusar(resp *Resposta, pergunta string) {
	resp.Answer, resp.Refused, resp.Mode, resp.Sources = domain.MensagemForaDoObjetivo, true, ModoRecusada, []Fonte{}
	if domain.SobreTemporalidade(pergunta) || domain.SobreFluxo(pergunta) {
		resp.Answer = domain.MensagemSemFonte
	}
}

type candidato struct {
	fonte   Fonte
	sintese string
	serie   string // série da TTDD (a própria ou a do procedimento)
}

// focar mantém só o tipo de fonte que a pergunta pede, quando ele tem
// candidato acima do limiar: "como funciona o fluxo do pregão" responde
// com o procedimento (que já traz a temporalidade); "prazo de guarda do
// pregão", com a série. Pergunta que fala dos dois mantém os dois.
func focar(hits []candidato, pergunta string) []candidato {
	tipo := ""
	switch fluxo, ttdd := domain.SobreFluxo(pergunta), domain.SobreTemporalidade(pergunta); {
	case fluxo && !ttdd:
		tipo = FonteProcedimento
	case ttdd && !fluxo:
		tipo = FonteTTDD
	default:
		return hits
	}
	var out []candidato
	for _, h := range hits {
		if h.fonte.Tipo == tipo {
			out = append(out, h)
		}
	}
	if len(out) == 0 || out[0].fonte.Relevancia < domain.LimiarRelevancia {
		return hits
	}
	return out
}

// Perguntar responde SÓ sobre o acervo do Atlas — fluxos homologados e a
// TTDD oficial —, com grounding estrito: só procedimentos e séries com
// relevância >= domain.LimiarRelevancia sustentam a resposta (até
// maxFontes); abaixo disso, a recusa canônica. Qualquer outro assunto
// recebe "foge do objetivo da IA", decidido aqui (determinístico) e
// reforçado no prompt do modelo. Exige atlas:read (na rota).
func (s *Service) Perguntar(ctx context.Context, pergunta string) (Resposta, error) {
	pergunta = strings.TrimSpace(pergunta)
	resp := Resposta{Sources: []Fonte{}, GeneratedAt: s.now()}

	procs, err := s.repo.Candidatos(ctx, s.pool, pergunta, maxCandidatos)
	if err != nil {
		return resp, MapError(err)
	}
	series, err := s.repo.CandidatosTTDD(ctx, s.pool, pergunta, maxCandidatos)
	if err != nil {
		return resp, MapError(err)
	}
	var hits []candidato
	for _, w := range procs {
		if r := domain.RelevanciaProcedimento(w, pergunta); r > 0 {
			id := w.ID
			hits = append(hits, candidato{Fonte{Tipo: FonteProcedimento, ID: &id, Codigo: w.CodigoProcessual, Titulo: w.Titulo, Relevancia: r},
				domain.SinteseFluxo(w), w.CodigoTTDD})
		}
	}
	for _, c := range series {
		if r := domain.RelevanciaTTDD(c, pergunta); r > 0 {
			hits = append(hits, candidato{Fonte{Tipo: FonteTTDD, Codigo: c.Codigo, Titulo: c.Descritor, Relevancia: r}, domain.SinteseTTDD(c), c.Codigo})
		}
	}
	// Empate: o nome mais curto (mais específico) vem antes — o PDF tem
	// descritores com notas longas coladas que casam com muitas palavras.
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].fonte.Relevancia != hits[j].fonte.Relevancia {
			return hits[i].fonte.Relevancia > hits[j].fonte.Relevancia
		}
		return utf8.RuneCountInString(hits[i].fonte.Titulo) < utf8.RuneCountInString(hits[j].fonte.Titulo)
	})
	melhor := "" // nome da fonte mais relevante (para o filtro de pedidos)
	if len(hits) > 0 {
		resp.Score, melhor = hits[0].fonte.Relevancia, hits[0].fonte.Titulo
	}

	var contexto []string
	for _, h := range focar(hits, pergunta) {
		if h.fonte.Relevancia < domain.LimiarRelevancia || h.fonte.Relevancia < resp.Score-margemFontes || len(contexto) == maxFontes {
			break
		}
		modelos, err := s.repo.ModelosDaSerie(ctx, s.pool, h.serie)
		if err != nil {
			return resp, MapError(err)
		}
		contexto = append(contexto, domain.ComModelos(h.sintese, h.serie, modelos))
		resp.Sources = append(resp.Sources, h.fonte)
	}
	sintese := strings.Join(contexto, "\n\n")

	switch {
	case domain.PedidoForaDoObjetivo(pergunta, melhor):
		recusar(&resp, "")
	case len(contexto) == 0:
		recusar(&resp, pergunta)
	case s.assistente == nil:
		resp.Answer, resp.Mode = sintese, ModoSintese
	default:
		answer, err := s.assistente.Responder(ctx, pergunta, contexto)
		switch {
		case err == nil && domain.ForaDoObjetivo(answer):
			// O modelo reconheceu um pedido fora do objetivo (ex.: a fonte
			// casou por palavras, mas a pergunta pede outra coisa).
			recusar(&resp, "")
		case err == nil:
			resp.Answer, resp.Mode = answer, ModoIA
		case errors.Is(err, domain.ErrIADesligada):
			resp.Answer, resp.Mode = sintese, ModoSintese
		default:
			s.logger.WarnContext(ctx, "atlas: assistente de IA indisponível, usando a síntese canônica", "error", err)
			resp.Answer, resp.Mode = sintese, ModoSintese
		}
	}

	// Trilha da consulta sem o texto da pergunta (minimização, LGPD art.
	// 6º III): ela pode trazer dados pessoais de terceiros.
	codigos := make([]string, len(resp.Sources))
	for i, f := range resp.Sources {
		codigos[i] = f.Codigo
	}
	if err := audit.NewWriter(s.pool).Record(ctx, audit.Meta(ctx, "atlas.assistente.consulta", "atlas_assistente", "", nil,
		map[string]any{"modo": resp.Mode, "relevancia": resp.Score, "fontes": codigos, "tamanho_pergunta": len([]rune(pergunta))})); err != nil {
		return Resposta{}, err
	}
	return resp, nil
}
