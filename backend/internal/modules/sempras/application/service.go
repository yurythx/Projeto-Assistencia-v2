// Package application contém os casos de uso do SEMPRAS (ADR 030). Cada
// escrita grava o dado, o evento (outbox) e a auditoria na MESMA transação
// (ADR 016); o tempo real é publicado depois do commit, sem dado pessoal.
package application

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/yurythx/projeto-assistencia-v2/internal/domain/errors"
	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/domain"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/audit"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/auth"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/database"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/modkit"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/outbox"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/storage"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/ws"
)

// EntidadeSlug é a entidade da estrutura real cujas unidades o módulo
// atende quando quem consulta alcança tudo (deploy/estrutura).
const EntidadeSlug = "sempras"

// Eventos de domínio — só identificadores e estado (o Egress pode
// repassá-los a webhooks; ADR 030 §7).
const (
	EventCidadaoCadastrado     = "sempras.cidadao.cadastrado"
	EventAtendimentoAtualizado = "sempras.atendimento.atualizado"
)

// FrameFila é o tipo do frame de tempo real da fila.
const FrameFila = "sempras.fila"

// TopicFila é o tópico WebSocket da fila de uma unidade.
func TopicFila(unidade uuid.UUID) string { return "sempras:fila:" + unidade.String() }

// Publisher é a porta para o tempo real (implementada por *ws.Hub).
type Publisher interface {
	Publish(ctx context.Context, topic, frameType string, data any) error
}

// acoes: ação da fila -> nome da ação na auditoria.
var acoes = map[string]string{
	"chamar":   "sempras.fila.chamado",
	"iniciar":  "sempras.fila.iniciado",
	"concluir": "sempras.fila.concluido",
	"ausente":  "sempras.fila.nao_compareceu",
	"cancelar": "sempras.fila.cancelado",
}

// Service orquestra os casos de uso.
type Service struct {
	pool         *pgxpool.Pool
	repo         domain.Repository
	outbox       *outbox.Writer
	hub          Publisher
	store        storage.Provider
	bucket       string
	maxDocBytes  int64
	uploadExpiry time.Duration
	logger       *slog.Logger
	now          func() time.Time
}

// NewService cria o serviço.
func NewService(pool *pgxpool.Pool, repo domain.Repository, ob *outbox.Writer, hub Publisher, store storage.Provider, bucket string, maxDocBytes int64, uploadExpiry time.Duration, logger *slog.Logger) *Service {
	return &Service{pool: pool, repo: repo, outbox: ob, hub: hub, store: store, bucket: bucket, maxDocBytes: maxDocBytes,
		uploadExpiry: uploadExpiry, logger: logger, now: time.Now}
}

// WithClock troca o relógio (testes).
func (s *Service) WithClock(now func() time.Time) *Service {
	s.now = now
	return s
}

// dia é a data no fuso do processo (TZ=America/Cuiaba na imagem).
func (s *Service) dia() string { return s.now().In(time.Local).Format(time.DateOnly) }

// MapError traduz os erros de domínio para erros de aplicação (HTTP).
func MapError(err error) error {
	var v *domain.ValidationError
	switch {
	case errors.As(err, &v):
		return apperrors.Validation(v.Msg)
	case errors.Is(err, domain.ErrNotFound):
		return apperrors.NotFound("registro não encontrado")
	case errors.Is(err, domain.ErrServicoIndisponivel):
		return apperrors.NotFound("esta unidade não oferta este serviço")
	case errors.Is(err, domain.ErrForbidden):
		return apperrors.Forbidden(err.Error())
	case errors.Is(err, domain.ErrInvalidState), errors.Is(err, domain.ErrFilaVazia), errors.Is(err, domain.ErrJaNaFila),
		errors.Is(err, domain.ErrDocumentoDuplicado), errors.Is(err, domain.ErrPedidoDecidido), errors.Is(err, domain.ErrBeneficioEstado),
		errors.Is(err, domain.ErrEncaminhamentoDecidido), errors.Is(err, domain.ErrPaifJaAtivo), errors.Is(err, domain.ErrPaifJaExcluido),
		errors.Is(err, domain.ErrConselhoEstado), errors.Is(err, domain.ErrPaefiEstado), errors.Is(err, domain.ErrMseEstado),
		errors.Is(err, domain.ErrNotificacaoEstado), errors.Is(err, domain.ErrAcolhimentoEstado),
		errors.Is(err, domain.ErrCasaMulherEstado), errors.Is(err, domain.ErrCasaMulherJaAtivo), errors.Is(err, domain.ErrOficioRespondido),
		errors.Is(err, domain.ErrAcolhimentoAtivo), errors.Is(err, domain.ErrVisitaEncerrada), errors.Is(err, domain.ErrRelatorioEntregue):
		return apperrors.Conflict(err.Error())
	case errors.Is(err, domain.ErrUnidadeInativa), errors.Is(err, domain.ErrAtendimentoOutro), errors.Is(err, domain.ErrMesmaUnidade):
		return apperrors.Validation(err.Error())
	}
	return err
}

// unidade carrega a unidade, exige alguma das permissões nela e que ela
// oferte o serviço do caso de uso (ADR 031 §2). Quem não alcança a unidade
// com permissão nenhuma do módulo recebe "não encontrado"; a unidade que
// não oferta o serviço também (o mesmo contrato de um módulo desligado).
func (s *Service) unidade(ctx context.Context, db database.DBTX, identity auth.Identity, id uuid.UUID, servico string, perms ...auth.Permission) (domain.Unidade, error) {
	u, err := s.repo.GetUnidade(ctx, db, id)
	if err != nil {
		return domain.Unidade{}, err
	}
	if !u.PodeAlguma(identity, domain.TodasAsPermissoes...) {
		return domain.Unidade{}, domain.ErrNotFound
	}
	if !u.PodeAlguma(identity, perms...) {
		return domain.Unidade{}, domain.ErrForbidden
	}
	if !u.Oferta(servico) {
		return domain.Unidade{}, domain.ErrServicoIndisponivel
	}
	return u, nil
}

func (s *Service) publish(ctx context.Context, a domain.Atendimento) {
	data := map[string]any{"atendimento_id": a.ID, "status": a.Status, "senha": a.SenhaExibida()}
	if err := s.hub.Publish(ctx, TopicFila(a.UnidadeID), FrameFila, data); err != nil {
		s.logger.Warn("sempras: falha ao publicar a fila em tempo real (já persistida)", slog.Any("error", err))
	}
}

func (s *Service) evento(ctx context.Context, tx pgx.Tx, a domain.Atendimento) error {
	return s.outbox.Write(ctx, tx, EventAtendimentoAtualizado, "sempras_atendimento", a.ID.String(), uuid.Nil,
		map[string]string{"id": a.ID.String(), "unidade_id": a.UnidadeID.String(), "status": a.Status})
}

// ---------------------------------------------------------------- unidades

// Unidades lista as unidades onde identity atua (seletor da tela), com o
// tipo e os serviços de cada uma — é o que monta as abas do frontend.
func (s *Service) Unidades(ctx context.Context, identity auth.Identity) ([]domain.Unidade, error) {
	return s.unidadesQueAtendem(ctx, domain.AreaDe(identity, domain.TodasAsPermissoes...))
}

// UnidadesParaConfigurar lista todas as unidades da SEMPRAS que identity
// gere — inclusive as sem tipo, que ainda não atendem (ADR 031).
func (s *Service) UnidadesParaConfigurar(ctx context.Context, identity auth.Identity) ([]domain.Unidade, error) {
	area := domain.AreaDe(identity, auth.PermSemprasGestao)
	if area.Vazia() {
		return nil, domain.ErrForbidden
	}
	return s.repo.ListUnidades(ctx, s.pool, area, EntidadeSlug)
}

// ConfigurarUnidade define o tipo da unidade e se o painel de chamada mostra
// o nome (ADR 031). Tipo "" tira a configuração: a unidade deixa de atender.
// É da gestão — muda os serviços que toda a equipe da unidade enxerga.
func (s *Service) ConfigurarUnidade(ctx context.Context, identity auth.Identity, id uuid.UUID, tipo string, painelExibeNome bool) (domain.Unidade, error) {
	if tipo != "" && !slices.Contains(domain.TiposUnidade, tipo) {
		return domain.Unidade{}, &domain.ValidationError{Msg: "tipo de unidade inválido"}
	}
	var u domain.Unidade
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		if u, err = s.repo.GetUnidade(ctx, tx, id); err != nil {
			return err
		}
		if !u.PodeAlguma(identity, domain.TodasAsPermissoes...) {
			return domain.ErrNotFound
		}
		if !u.Pode(identity, auth.PermSemprasGestao) {
			return domain.ErrForbidden
		}
		if err := s.repo.ConfigurarUnidade(ctx, tx, id, tipo, painelExibeNome); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.unidade.configurada", "unidade", id.String(),
			map[string]any{"tipo": u.Tipo, "painel_exibe_nome": u.PainelExibeNome},
			map[string]any{"tipo": tipo, "painel_exibe_nome": painelExibeNome})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.Unidade{}, err
	}
	u.Tipo, u.PainelExibeNome = tipo, painelExibeNome && tipo != ""
	u.Servicos, u.Demandas = domain.ServicosDoTipo(tipo), domain.DemandasDoTipo(tipo)
	return u, nil
}

// unidadesQueAtendem lista as unidades da área que têm tipo (ADR 031): a
// Sede e os setores administrativos da SEMPRAS não aparecem no seletor,
// no painel nem no RMA.
func (s *Service) unidadesQueAtendem(ctx context.Context, area domain.Area) ([]domain.Unidade, error) {
	todas, err := s.repo.ListUnidades(ctx, s.pool, area, EntidadeSlug)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Unidade, 0, len(todas))
	for _, u := range todas {
		if u.Tipo != "" {
			out = append(out, u)
		}
	}
	return out, nil
}

// ----------------------------------------------------------------- munícipe

// Buscar procura por CPF/NIS (11 dígitos) ou por trecho do nome (3+
// letras). O histórico contado é só o da área de quem busca.
func (s *Service) Buscar(ctx context.Context, identity auth.Identity, q string) ([]domain.Resumo, error) {
	q = strings.TrimSpace(q)
	var (
		achados []domain.Cidadao
		err     error
	)
	switch d := domain.Digitos(q); {
	case len(d) == 11 && !strings.ContainsFunc(q, unicode.IsLetter):
		achados, err = s.repo.BuscarPorDocumento(ctx, s.pool, d)
	case utf8.RuneCountInString(q) >= 3:
		achados, err = s.repo.BuscarPorNome(ctx, s.pool, q, 20)
	default:
		return nil, &domain.ValidationError{Msg: "informe o CPF ou o NIS (11 dígitos) ou ao menos 3 letras do nome"}
	}
	if err != nil {
		return nil, err
	}
	out := []domain.Resumo{}
	if len(achados) == 0 {
		return out, nil
	}
	ids := make([]uuid.UUID, 0, len(achados))
	for _, c := range achados {
		ids = append(ids, c.ID)
	}
	hist, err := s.repo.HistoricoNaArea(ctx, s.pool, ids, domain.AreaDe(identity, domain.TodasAsPermissoes...))
	if err != nil {
		return nil, err
	}
	for _, c := range achados {
		out = append(out, c.Resumir(hist[c.ID]))
	}
	return out, nil
}

// camposAlterados lista os campos que mudaram — a auditoria guarda isso, e
// não os valores (dado sensível numa trilha imutável).
func camposAlterados(antes, depois domain.Cidadao) []string {
	pares := []struct {
		nome string
		a, b string
	}{
		{"nome", antes.Nome, depois.Nome}, {"nome_social", antes.NomeSocial, depois.NomeSocial},
		{"cpf", antes.CPF, depois.CPF}, {"nis", antes.NIS, depois.NIS},
		{"data_nascimento", antes.DataNascimento, depois.DataNascimento}, {"nome_mae", antes.NomeMae, depois.NomeMae},
		{"sexo", antes.Sexo, depois.Sexo},
		{"telefone", antes.Telefone, depois.Telefone}, {"endereco", antes.Endereco, depois.Endereco},
	}
	out := []string{}
	for _, p := range pares {
		if p.a != p.b {
			out = append(out, p.nome)
		}
	}
	return out
}

// Cadastrar grava um novo munícipe.
func (s *Service) Cadastrar(ctx context.Context, identity auth.Identity, in domain.CidadaoInput) (domain.Cidadao, error) {
	now := s.now()
	c, err := in.Normalizar(now)
	if err != nil {
		return domain.Cidadao{}, err
	}
	c.ID, c.CreatedAt, c.UpdatedAt = uuid.New(), now, now
	err = database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		if err := s.repo.InsertCidadao(ctx, tx, c, identity.UserID); err != nil {
			return err
		}
		if err := s.outbox.Write(ctx, tx, EventCidadaoCadastrado, "sempras_cidadao", c.ID.String(), uuid.Nil,
			map[string]string{"id": c.ID.String()}); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.cidadao.criado", "sempras_cidadao", c.ID.String(), nil,
			map[string]any{"campos": camposAlterados(domain.Cidadao{}, c)})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.Cidadao{}, err
	}
	return c, nil
}

// Atualizar regrava o cadastro (auditoria com os campos alterados). Os 4
// perfis do módulo podem editar (ROADMAP_SEMPRAS.md, matriz de acesso) —
// o cadastro não é de uma unidade, então o gate é "alguma permissão do
// módulo", igual ao de Prontuario/Painel, não uma unidade específica.
func (s *Service) Atualizar(ctx context.Context, identity auth.Identity, id uuid.UUID, in domain.CidadaoInput) (domain.Cidadao, error) {
	if domain.AreaDe(identity, domain.TodasAsPermissoes...).Vazia() {
		return domain.Cidadao{}, domain.ErrForbidden
	}
	now := s.now()
	c, err := in.Normalizar(now)
	if err != nil {
		return domain.Cidadao{}, err
	}
	err = database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		antes, err := s.repo.GetCidadao(ctx, tx, id, true)
		if err != nil {
			return err
		}
		c.ID, c.CreatedAt, c.UpdatedAt = id, antes.CreatedAt, now
		if err := s.repo.UpdateCidadao(ctx, tx, c); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.cidadao.atualizado", "sempras_cidadao", id.String(), nil,
			map[string]any{"campos": camposAlterados(antes, c)})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.Cidadao{}, err
	}
	return c, nil
}

// Cidadao devolve o cadastro (identificação, para conferir e editar).
func (s *Service) Cidadao(ctx context.Context, id uuid.UUID) (domain.Cidadao, error) {
	return s.repo.GetCidadao(ctx, s.pool, id, false)
}

// montarProntuario carrega cadastro, atendimentos e evoluções da área de
// identity, ocultando o texto dos registros psicológicos sem sigilo (§5).
// Compartilhado por Prontuario (leitura de trabalho) e SolicitarAcesso
// (exercício do direito de acesso, §8) — mesmos dados, auditoria diferente.
func (s *Service) montarProntuario(ctx context.Context, tx pgx.Tx, identity auth.Identity, id uuid.UUID, area domain.Area) (domain.Prontuario, error) {
	var p domain.Prontuario
	var err error
	if p.Cidadao, err = s.repo.GetCidadao(ctx, tx, id, false); err != nil {
		return domain.Prontuario{}, err
	}
	if p.Atendimentos, err = s.repo.AtendimentosNaArea(ctx, tx, id, area); err != nil {
		return domain.Prontuario{}, err
	}
	if p.Registros, err = s.repo.RegistrosNaArea(ctx, tx, id, area); err != nil {
		return domain.Prontuario{}, err
	}
	if p.RedeApoio, err = s.repo.ListContatosRedeApoio(ctx, tx, id); err != nil {
		return domain.Prontuario{}, err
	}
	if p.Beneficios, err = s.repo.BeneficiosNaArea(ctx, tx, id, area); err != nil {
		return domain.Prontuario{}, err
	}
	if p.Encaminhamentos, err = s.repo.EncaminhamentosDoCidadaoNaArea(ctx, tx, id, area); err != nil {
		return domain.Prontuario{}, err
	}
	if p.PaifVinculos, err = s.repo.PaifVinculosNaArea(ctx, tx, id, area); err != nil {
		return domain.Prontuario{}, err
	}
	if p.ConselhoAtendimentos, err = s.repo.ConselhoAtendimentosNaArea(ctx, tx, id, area); err != nil {
		return domain.Prontuario{}, err
	}
	if p.PaefiCasos, err = s.repo.PaefiCasosNaArea(ctx, tx, id, area); err != nil {
		return domain.Prontuario{}, err
	}
	if p.MseMedidas, err = s.repo.MseMedidasNaArea(ctx, tx, id, area); err != nil {
		return domain.Prontuario{}, err
	}
	if p.NotificacoesCreas, err = s.repo.NotificacoesCreasNaArea(ctx, tx, id, area); err != nil {
		return domain.Prontuario{}, err
	}
	if p.OficiosCras, err = s.repo.OficiosCrasNaArea(ctx, tx, id, area); err != nil {
		return domain.Prontuario{}, err
	}
	if p.EncaminhamentosExternos, err = s.repo.EncaminhamentosExternosNaArea(ctx, tx, id, area); err != nil {
		return domain.Prontuario{}, err
	}
	if p.CasaMulherCasos, err = s.repo.CasosCasaMulherNaArea(ctx, tx, id, area); err != nil {
		return domain.Prontuario{}, err
	}
	if p.Abordagens, err = s.repo.AbordagensNaArea(ctx, tx, id, area); err != nil {
		return domain.Prontuario{}, err
	}
	// O perfil de rua (saúde, uso de drogas) e os documentos são do
	// munícipe, não de uma unidade — então o filtro é o tipo: só sai para
	// quem atua num Centro POP (ADR 031 §4, necessidade de saber).
	unidadesDaArea, err := s.repo.ListUnidades(ctx, tx, area, EntidadeSlug)
	if err != nil {
		return domain.Prontuario{}, err
	}
	p.CentroPopDocumentos = []domain.DocumentoCentroPop{}
	if slices.ContainsFunc(unidadesDaArea, func(u domain.Unidade) bool { return u.Oferta(domain.ServicoCentroPop) }) {
		switch perfil, err := s.repo.GetCentroPopPerfil(ctx, tx, id); {
		case errors.Is(err, domain.ErrNotFound):
			// perfil ainda não preenchido — fica nil, não é erro.
		case err != nil:
			return domain.Prontuario{}, err
		default:
			p.CentroPopPerfil = &perfil
		}
		if p.CentroPopDocumentos, err = s.repo.ListDocumentosCentroPop(ctx, tx, id); err != nil {
			return domain.Prontuario{}, err
		}
	}
	if p.OutrasUnidades, err = s.repo.AcompanhamentosForaDaArea(ctx, tx, id, area); err != nil {
		return domain.Prontuario{}, err
	}
	if p.RegistrosOrigem, err = s.repo.RegistrosDeOrigem(ctx, tx, id, area); err != nil {
		return domain.Prontuario{}, err
	}
	if p.CasaAbrigoAcolhimentos, err = s.repo.AcolhimentosNaArea(ctx, tx, id, area); err != nil {
		return domain.Prontuario{}, err
	}
	for i := range p.CasaAbrigoAcolhimentos {
		if err := s.completarAcolhimento(ctx, tx, &p.CasaAbrigoAcolhimentos[i]); err != nil {
			return domain.Prontuario{}, err
		}
	}
	unidades := map[uuid.UUID]domain.Unidade{}
	for i, r := range p.Registros {
		if r.Tipo != domain.TipoPsicologico {
			continue
		}
		u, ok := unidades[r.UnidadeID]
		if !ok {
			if u, err = s.repo.GetUnidade(ctx, tx, r.UnidadeID); err != nil {
				return domain.Prontuario{}, err
			}
			unidades[r.UnidadeID] = u
		}
		if !u.PodePsicologia(identity) {
			p.Registros[i] = r.Ocultar()
		}
	}
	return p, nil
}

// Prontuario é a ficha: cadastro, atendimentos e evoluções da área de quem
// consulta. Toda abertura é auditada (ADR 030 §6) — sem auditoria, sem
// ficha. Registros psicológicos sem o sigilo saem sem o texto (§5).
func (s *Service) Prontuario(ctx context.Context, identity auth.Identity, id uuid.UUID) (domain.Prontuario, error) {
	area := domain.AreaDe(identity, auth.PermSemprasAtendimento)
	if area.Vazia() {
		return domain.Prontuario{}, domain.ErrForbidden
	}
	var p domain.Prontuario
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		if p, err = s.montarProntuario(ctx, tx, identity, id, area); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.prontuario.visualizado", "sempras_cidadao", id.String(), nil, nil)
		e.Metadata = map[string]any{"atendimentos": len(p.Atendimentos), "registros": len(p.Registros)}
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.Prontuario{}, err
	}
	return p, nil
}

// EvolucaoInput é uma nova evolução do prontuário.
type EvolucaoInput struct {
	UnidadeID     uuid.UUID
	AtendimentoID *uuid.UUID
	Tipo, Texto   string
}

// RegistrarEvolucao inclui uma evolução (social com sempras:atendimento;
// psicológica só com o sigilo da psicologia na unidade).
func (s *Service) RegistrarEvolucao(ctx context.Context, identity auth.Identity, cidadao uuid.UUID, in EvolucaoInput) (domain.Registro, error) {
	r, err := domain.NovoRegistro(cidadao, in.UnidadeID, in.AtendimentoID, in.Tipo, in.Texto, identity.UserID)
	if err != nil {
		return domain.Registro{}, err
	}
	r.CreatedAt, r.AutorNome = s.now(), identity.Name
	err = database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		u, err := s.unidade(ctx, tx, identity, in.UnidadeID, domain.ServicoProntuario, auth.PermSemprasAtendimento)
		if err != nil {
			return err
		}
		if !u.PodeTipo(identity, r.Tipo) {
			return domain.ErrForbidden
		}
		if !u.Ativa {
			return domain.ErrUnidadeInativa
		}
		if _, err := s.repo.GetCidadao(ctx, tx, cidadao, false); err != nil {
			return err
		}
		if in.AtendimentoID != nil {
			a, err := s.repo.GetAtendimento(ctx, tx, *in.AtendimentoID, false)
			if err != nil {
				return err
			}
			if a.CidadaoID != cidadao || a.UnidadeID != u.ID {
				return domain.ErrAtendimentoOutro
			}
		}
		if err := s.repo.InsertRegistro(ctx, tx, r); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.registro.criado", "sempras_registro", r.ID.String(), nil,
			map[string]any{"cidadao_id": cidadao, "unidade_id": u.ID, "tipo": r.Tipo})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.Registro{}, err
	}
	return r, nil
}

// --------------------------------------------------------------------- fila

// EntradaInput é a inclusão de um munícipe na fila.
type EntradaInput struct {
	CidadaoID               uuid.UUID
	Tipo, Prioridade        string
	Demanda, DemandaDetalhe string
}

// Incluir põe o munícipe na fila da unidade (recepção ou técnico), com a
// senha do dia.
func (s *Service) Incluir(ctx context.Context, identity auth.Identity, unidade uuid.UUID, in EntradaInput) (domain.FilaItem, error) {
	a, err := domain.NovaEntrada(in.CidadaoID, unidade, in.Tipo, in.Prioridade, identity.UserID, s.dia())
	if err != nil {
		return domain.FilaItem{}, err
	}
	a.CreatedAt = s.now()
	var c domain.Cidadao
	err = database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		u, err := s.unidade(ctx, tx, identity, unidade, domain.ServicoFila, auth.PermSemprasRecepcao, auth.PermSemprasAtendimento)
		if err != nil {
			return err
		}
		if !u.Ativa {
			return domain.ErrUnidadeInativa
		}
		if c, err = s.repo.GetCidadao(ctx, tx, in.CidadaoID, false); err != nil {
			return err
		}
		if a.Demanda, a.DemandaDetalhe, err = u.ValidarEntrada(c, in.Demanda, in.DemandaDetalhe); err != nil {
			return err
		}
		if a.Senha, err = s.repo.ProximaSenha(ctx, tx, u.ID, a.Dia); err != nil {
			return err
		}
		if err := s.repo.InsertAtendimento(ctx, tx, a); err != nil {
			return err
		}
		if err := s.evento(ctx, tx, a); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.fila.incluido", "sempras_atendimento", a.ID.String(), nil,
			map[string]any{"status": a.Status, "unidade_id": u.ID, "cidadao_id": a.CidadaoID, "tipo": a.Tipo, "prioridade": a.Prioridade,
				"demanda": a.Demanda})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.FilaItem{}, err
	}
	s.publish(ctx, a)
	return domain.FilaItem{Atendimento: a, SenhaExibida: a.SenhaExibida(), Nome: c.NomeExibicao()}, nil
}

// Fila devolve as entradas do dia da unidade.
func (s *Service) Fila(ctx context.Context, identity auth.Identity, unidade uuid.UUID) ([]domain.FilaItem, error) {
	if _, err := s.unidade(ctx, s.pool, identity, unidade, domain.ServicoFila, domain.TodasAsPermissoes...); err != nil {
		return nil, err
	}
	return s.repo.Fila(ctx, s.pool, unidade, s.dia())
}

// transitar grava a mudança de estado com evento e auditoria.
func (s *Service) transitar(ctx context.Context, tx pgx.Tx, a *domain.Atendimento, acao string, identity auth.Identity, local string) error {
	antes := a.Status
	if err := a.Transitar(acao, identity.UserID, local, s.now()); err != nil {
		return err
	}
	if err := s.repo.UpdateAtendimento(ctx, tx, *a); err != nil {
		return err
	}
	if err := s.evento(ctx, tx, *a); err != nil {
		return err
	}
	e := audit.Meta(ctx, acoes[acao], "sempras_atendimento", a.ID.String(),
		map[string]any{"status": antes}, map[string]any{"status": a.Status, "local": a.Local})
	return audit.NewWriter(tx).Record(ctx, e)
}

// ChamarProximo chama o próximo da fila que identity pode atender
// (prioridade, depois chegada; sem corrida entre técnicos).
func (s *Service) ChamarProximo(ctx context.Context, identity auth.Identity, unidade uuid.UUID, local string) (domain.FilaItem, error) {
	var (
		a domain.Atendimento
		c domain.Cidadao
	)
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		u, err := s.unidade(ctx, tx, identity, unidade, domain.ServicoFila, auth.PermSemprasAtendimento)
		if err != nil {
			return err
		}
		tipos := []string{domain.TipoSocial}
		if u.PodePsicologia(identity) {
			tipos = append(tipos, domain.TipoPsicologico)
		}
		if a, err = s.repo.ProximoAguardando(ctx, tx, u.ID, s.dia(), tipos); err != nil {
			return err
		}
		if c, err = s.repo.GetCidadao(ctx, tx, a.CidadaoID, false); err != nil {
			return err
		}
		return s.transitar(ctx, tx, &a, "chamar", identity, local)
	})
	if err != nil {
		return domain.FilaItem{}, err
	}
	s.publish(ctx, a)
	return domain.FilaItem{Atendimento: a, SenhaExibida: a.SenhaExibida(), Nome: c.NomeExibicao()}, nil
}

// Agir aplica uma ação (chamar, iniciar, concluir, ausente, cancelar) a uma
// entrada. Cancelar cabe à recepção, ao técnico e à gestão; as demais, a
// quem atende o tipo (o psicológico, só a psicologia).
func (s *Service) Agir(ctx context.Context, identity auth.Identity, id uuid.UUID, acao, local string) (domain.Atendimento, error) {
	if _, ok := acoes[acao]; !ok {
		return domain.Atendimento{}, domain.ErrInvalidState
	}
	var a domain.Atendimento
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		if a, err = s.repo.GetAtendimento(ctx, tx, id, true); err != nil {
			return err
		}
		perms := []auth.Permission{auth.PermSemprasRecepcao, auth.PermSemprasAtendimento, auth.PermSemprasGestao}
		if acao != "cancelar" {
			perms = []auth.Permission{auth.PermSemprasAtendimento}
		}
		u, err := s.unidade(ctx, tx, identity, a.UnidadeID, domain.ServicoFila, perms...)
		if err != nil {
			return err
		}
		if acao != "cancelar" && !u.PodeTipo(identity, a.Tipo) {
			return domain.ErrForbidden
		}
		return s.transitar(ctx, tx, &a, acao, identity, local)
	})
	if err != nil {
		return domain.Atendimento{}, err
	}
	s.publish(ctx, a)
	return a, nil
}

// Painel são os números do dia das unidades que identity gere.
func (s *Service) Painel(ctx context.Context, identity auth.Identity) ([]domain.PainelUnidade, error) {
	area := domain.AreaDe(identity, auth.PermSemprasGestao)
	if area.Vazia() {
		return nil, domain.ErrForbidden
	}
	unidades, err := s.unidadesQueAtendem(ctx, area)
	if err != nil {
		return nil, err
	}
	return s.repo.Painel(ctx, s.pool, unidades, s.dia())
}

// removeObjetos apaga objetos órfãos do storage — melhor esforço, nunca
// bloqueia a exclusão do metadado (mesmo padrão do módulo files).
func (s *Service) removeObjetos(ctx context.Context, keys []string) {
	for _, k := range keys {
		if err := s.store.Delete(ctx, s.bucket, k); err != nil {
			s.logger.Warn("sempras: objeto órfão não removido do storage", slog.String("key", k), slog.Any("error", err))
		}
	}
}

// --------------------------------------------------------------- Centro POP

// AtualizarCentroPopPerfil grava (upsert) o perfil de rua do munícipe —
// um por cidadão.
func (s *Service) AtualizarCentroPopPerfil(ctx context.Context, identity auth.Identity, cidadao, unidade uuid.UUID,
	situacao string, tempoRuaAnos *float64, motivo string, possuiDoenca bool, doencaDescricao string,
	usaMedicacao bool, medicacaoDescricao string, usoDrogas bool, drogasDescricao string) (domain.CentroPopPerfil, error) {
	p, err := domain.NovoCentroPopPerfil(cidadao, situacao, tempoRuaAnos, motivo, possuiDoenca, doencaDescricao,
		usaMedicacao, medicacaoDescricao, usoDrogas, drogasDescricao)
	if err != nil {
		return domain.CentroPopPerfil{}, err
	}
	p.AtualizadoEm = s.now()
	err = database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		u, err := s.unidade(ctx, tx, identity, unidade, domain.ServicoCentroPop, auth.PermSemprasAtendimento)
		if err != nil {
			return err
		}
		if !u.Ativa {
			return domain.ErrUnidadeInativa
		}
		if _, err := s.repo.GetCidadao(ctx, tx, cidadao, false); err != nil {
			return err
		}
		if err := s.repo.UpsertCentroPopPerfil(ctx, tx, p, identity.UserID); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.centro_pop.perfil_atualizado", "sempras_centro_pop_perfil", cidadao.String(), nil,
			map[string]any{"situacao": situacao, "motivo_vulnerabilidade": motivo})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.CentroPopPerfil{}, err
	}
	return p, nil
}

// RegistrarEncaminhamentoExterno registra que o munícipe foi encaminhado
// pra fora do sistema (CAPS AD, albergue, saúde, polícia etc.).
func (s *Service) RegistrarEncaminhamentoExterno(ctx context.Context, identity auth.Identity, cidadao, unidade uuid.UUID, destino, prioridade, motivo string) (domain.EncaminhamentoExterno, error) {
	e, err := domain.NovoEncaminhamentoExterno(cidadao, unidade, destino, prioridade, motivo)
	if err != nil {
		return domain.EncaminhamentoExterno{}, err
	}
	e.CreatedAt, e.AtualizadoEm = s.now(), s.now()
	err = database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		u, err := s.unidade(ctx, tx, identity, unidade, domain.ServicoCentroPop, auth.PermSemprasAtendimento)
		if err != nil {
			return err
		}
		if !u.Ativa {
			return domain.ErrUnidadeInativa
		}
		if _, err := s.repo.GetCidadao(ctx, tx, cidadao, false); err != nil {
			return err
		}
		if err := s.repo.InsertEncaminhamentoExterno(ctx, tx, e, identity.UserID); err != nil {
			return err
		}
		ev := audit.Meta(ctx, "sempras.encaminhamento_externo.registrado", "sempras_encaminhamento_externo", e.ID.String(), nil,
			map[string]any{"cidadao_id": cidadao, "unidade_id": unidade, "destino": destino, "prioridade": prioridade})
		return audit.NewWriter(tx).Record(ctx, ev)
	})
	if err != nil {
		return domain.EncaminhamentoExterno{}, err
	}
	return e, nil
}

// TransitarEncaminhamentoExterno muda o status (gerenciado por quem
// atende, sem fluxo de aceite — ver domain.EncaminhamentoExterno).
func (s *Service) TransitarEncaminhamentoExterno(ctx context.Context, identity auth.Identity, id uuid.UUID, status, observacoes string) (domain.EncaminhamentoExterno, error) {
	var e domain.EncaminhamentoExterno
	now := s.now()
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		if e, err = s.repo.GetEncaminhamentoExterno(ctx, tx, id, true); err != nil {
			return err
		}
		if _, err := s.unidade(ctx, tx, identity, e.UnidadeID, domain.ServicoCentroPop, auth.PermSemprasAtendimento); err != nil {
			return err
		}
		antes := e.Status
		if err := e.Transitar(status, observacoes, identity.UserID, now); err != nil {
			return err
		}
		if err := s.repo.UpdateEncaminhamentoExterno(ctx, tx, e); err != nil {
			return err
		}
		ev := audit.Meta(ctx, "sempras.encaminhamento_externo.transitado", "sempras_encaminhamento_externo", id.String(),
			map[string]any{"status": antes}, map[string]any{"status": e.Status})
		return audit.NewWriter(tx).Record(ctx, ev)
	})
	if err != nil {
		return domain.EncaminhamentoExterno{}, err
	}
	return e, nil
}

// documentoCentroPopPrefix é o prefixo dos objetos no bucket — um
// "subdiretório" por cidadão (mesmo padrão dos documentos da Casa Abrigo).
func documentoCentroPopPrefix(cidadao uuid.UUID) string {
	return "sempras/centro-pop/" + cidadao.String()
}

// UploadDocumentoCentroPopResult é o ticket de upload + o metadado pendente.
type UploadDocumentoCentroPopResult struct {
	Documento domain.DocumentoCentroPop `json:"documento"`
	Upload    modkit.UploadTicket       `json:"upload"`
}

// StartUploadDocumentoCentroPop cria o metadado pendente e a URL de
// upload direto ao object storage.
func (s *Service) StartUploadDocumentoCentroPop(ctx context.Context, identity auth.Identity, cidadao, unidade uuid.UUID, tipo, filename, contentType string, size int64) (UploadDocumentoCentroPopResult, error) {
	if size <= 0 || size > s.maxDocBytes {
		return UploadDocumentoCentroPopResult{}, &domain.ValidationError{Msg: "tamanho de arquivo acima do limite permitido"}
	}
	if _, err := s.unidade(ctx, s.pool, identity, unidade, domain.ServicoCentroPop, auth.PermSemprasAtendimento); err != nil {
		return UploadDocumentoCentroPopResult{}, err
	}
	if _, err := s.repo.GetCidadao(ctx, s.pool, cidadao, false); err != nil {
		return UploadDocumentoCentroPopResult{}, err
	}
	ticket, err := modkit.NewUpload(ctx, s.store, s.bucket, documentoCentroPopPrefix(cidadao), filename, contentType, documentoTypes, s.uploadExpiry)
	if err != nil {
		return UploadDocumentoCentroPopResult{}, err
	}
	d, err := domain.NovoDocumentoCentroPop(cidadao, unidade, tipo, filename, ticket.ObjectKey, ticket.Headers["Content-Type"])
	if err != nil {
		return UploadDocumentoCentroPopResult{}, err
	}
	d.CreatedAt = s.now()
	err = database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		if err := s.repo.InsertDocumentoCentroPop(ctx, tx, d, identity.UserID); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.centro_pop.documento_upload_iniciado", "sempras_centro_pop_documento", d.ID.String(), nil,
			map[string]any{"cidadao_id": cidadao, "tipo": tipo})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return UploadDocumentoCentroPopResult{}, err
	}
	return UploadDocumentoCentroPopResult{Documento: d, Upload: ticket}, nil
}

// ConfirmUploadDocumentoCentroPop confere o objeto enviado e publica o documento.
func (s *Service) ConfirmUploadDocumentoCentroPop(ctx context.Context, identity auth.Identity, id uuid.UUID) (domain.DocumentoCentroPop, error) {
	d, err := s.repo.GetDocumentoCentroPop(ctx, s.pool, id)
	if err != nil {
		return domain.DocumentoCentroPop{}, err
	}
	if _, err := s.unidade(ctx, s.pool, identity, d.UnidadeID, domain.ServicoCentroPop, auth.PermSemprasAtendimento); err != nil {
		return domain.DocumentoCentroPop{}, err
	}
	if d.Status == domain.DocumentoStatusReady {
		return d, nil
	}
	info, err := modkit.ConfirmUpload(ctx, s.store, s.bucket, d.ObjectKey, documentoCentroPopPrefix(d.CidadaoID), s.maxDocBytes, documentoTypes)
	if err != nil {
		return domain.DocumentoCentroPop{}, err
	}
	err = database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		if err := s.repo.UpdateDocumentoCentroPopStatus(ctx, tx, id, domain.DocumentoStatusReady, info.Size, info.ContentType); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.centro_pop.documento_confirmado", "sempras_centro_pop_documento", id.String(), nil,
			map[string]any{"cidadao_id": d.CidadaoID, "size_bytes": info.Size})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.DocumentoCentroPop{}, err
	}
	d.Status, d.SizeBytes, d.ContentType = domain.DocumentoStatusReady, info.Size, info.ContentType
	return d, nil
}

// DownloadURLDocumentoCentroPop devolve uma URL pré-assinada de leitura (curta).
func (s *Service) DownloadURLDocumentoCentroPop(ctx context.Context, identity auth.Identity, id uuid.UUID) (string, domain.DocumentoCentroPop, error) {
	d, err := s.repo.GetDocumentoCentroPop(ctx, s.pool, id)
	if err != nil {
		return "", domain.DocumentoCentroPop{}, err
	}
	if d.Status != domain.DocumentoStatusReady {
		return "", domain.DocumentoCentroPop{}, domain.ErrNotFound
	}
	if _, err := s.unidade(ctx, s.pool, identity, d.UnidadeID, domain.ServicoCentroPop, auth.PermSemprasAtendimento); err != nil {
		return "", domain.DocumentoCentroPop{}, err
	}
	url, err := s.store.PresignedGetURL(ctx, s.bucket, d.ObjectKey, 5*time.Minute)
	if err != nil {
		return "", domain.DocumentoCentroPop{}, apperrors.DependencyUnavailable("armazenamento indisponível").WithCause(err)
	}
	e := audit.Meta(ctx, "sempras.centro_pop.documento_baixado", "sempras_centro_pop_documento", id.String(), nil, nil)
	_ = audit.NewWriter(s.pool).Record(ctx, e)
	return url, d, nil
}

// RemoverDocumentoCentroPop remove o metadado e, fora da transação, o objeto no storage.
func (s *Service) RemoverDocumentoCentroPop(ctx context.Context, identity auth.Identity, id uuid.UUID) error {
	var key string
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		d, err := s.repo.GetDocumentoCentroPop(ctx, tx, id)
		if err != nil {
			return err
		}
		if _, err := s.unidade(ctx, tx, identity, d.UnidadeID, domain.ServicoCentroPop, auth.PermSemprasAtendimento); err != nil {
			return err
		}
		key = d.ObjectKey
		if err := s.repo.DeleteDocumentoCentroPop(ctx, tx, id); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.centro_pop.documento_removido", "sempras_centro_pop_documento", id.String(), nil,
			map[string]any{"cidadao_id": d.CidadaoID})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err == nil {
		s.removeObjetos(ctx, []string{key})
	}
	return err
}

// Rma é o Relatório Mensal de Atendimento (RMA/MDS) das unidades que
// identity gere, numa competência ("AAAA-MM").
func (s *Service) Rma(ctx context.Context, identity auth.Identity, competencia string) ([]domain.RmaUnidade, error) {
	area := domain.AreaDe(identity, auth.PermSemprasGestao)
	if area.Vazia() {
		return nil, domain.ErrForbidden
	}
	inicio, fim, err := domain.CompetenciaMes(competencia)
	if err != nil {
		return nil, err
	}
	unidades, err := s.unidadesQueAtendem(ctx, area)
	if err != nil {
		return nil, err
	}
	return s.repo.Rma(ctx, s.pool, unidades, inicio, fim)
}

// ---------------------------------------------- atendimentos coletivos (SCFV)

// RegistrarAtendimentoColetivo grava um evento de grupo (SCFV) da
// unidade — não é por munícipe.
func (s *Service) RegistrarAtendimentoColetivo(ctx context.Context, identity auth.Identity, unidade uuid.UUID, dataEvento time.Time, categoria string, participantes int, descricao string) (domain.AtendimentoColetivo, error) {
	a, err := domain.NovoAtendimentoColetivo(unidade, dataEvento, categoria, participantes, descricao)
	if err != nil {
		return domain.AtendimentoColetivo{}, err
	}
	err = database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		u, err := s.unidade(ctx, tx, identity, unidade, domain.ServicoColetivo, auth.PermSemprasAtendimento)
		if err != nil {
			return err
		}
		if !u.Ativa {
			return domain.ErrUnidadeInativa
		}
		if err := s.repo.InsertAtendimentoColetivo(ctx, tx, a, identity.UserID); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.atendimento_coletivo.registrado", "sempras_atendimento_coletivo", a.ID.String(), nil,
			map[string]any{"unidade_id": unidade, "categoria": a.Categoria, "participantes": a.Participantes})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.AtendimentoColetivo{}, err
	}
	return a, nil
}

// ListarAtendimentosColetivos lista os eventos de grupo de uma unidade
// num mês de competência ("2026-10").
func (s *Service) ListarAtendimentosColetivos(ctx context.Context, identity auth.Identity, unidade uuid.UUID, competencia string) ([]domain.AtendimentoColetivo, error) {
	inicio, fim, err := domain.CompetenciaMes(competencia)
	if err != nil {
		return nil, err
	}
	if _, err := s.unidade(ctx, s.pool, identity, unidade, domain.ServicoColetivo, auth.PermSemprasAtendimento); err != nil {
		return nil, err
	}
	return s.repo.ListAtendimentosColetivos(ctx, s.pool, unidade, inicio, fim)
}

// ------------------------------------------------------- direitos do titular

// camposAnonimizados nomeia os campos zerados pela anonimização — a
// auditoria guarda isso, nunca os valores (mesmo padrão de camposAlterados).
// data_nascimento fica de fora: é preservado (CHECK de identificação da
// tabela, ver infrastructure.AnonimizarCidadao).
var camposAnonimizados = []string{"nome", "nome_social", "cpf", "nis", "nome_mae", "telefone", "endereco"}

// SolicitarAcesso atende o direito de acesso (LGPD art. 18, II): exporta o
// prontuário do titular (mesma área e mesmo sigilo psicológico de
// Prontuario) e registra o pedido já concluído — quem atende resolve na
// hora, com o titular presente.
func (s *Service) SolicitarAcesso(ctx context.Context, identity auth.Identity, cidadao uuid.UUID, justificativa string) (domain.Prontuario, error) {
	area := domain.AreaDe(identity, auth.PermSemprasAtendimento)
	if area.Vazia() {
		return domain.Prontuario{}, domain.ErrForbidden
	}
	p, err := domain.NovoPedido(cidadao, identity.UserID, domain.PedidoAcesso, justificativa)
	if err != nil {
		return domain.Prontuario{}, err
	}
	now := s.now()
	p.Status, p.DecididoPor, p.DecididoEm, p.CreatedAt = domain.PedidoStatusConcluido, &identity.UserID, &now, now
	var pr domain.Prontuario
	err = database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		if pr, err = s.montarProntuario(ctx, tx, identity, cidadao, area); err != nil {
			return err
		}
		if err := s.repo.InsertPedido(ctx, tx, p); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.titular.acesso", "sempras_pedido_titular", p.ID.String(), nil,
			map[string]any{"cidadao_id": cidadao, "atendimentos": len(pr.Atendimentos), "registros": len(pr.Registros)})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.Prontuario{}, err
	}
	return pr, nil
}

// SolicitarPedido registra correção ou anonimização a pedido do titular —
// acesso tem rota própria (SolicitarAcesso), que já devolve os dados.
// Correção nasce concluída (a edição em si é um PUT /cidadaos/{id} à
// parte); anonimização nasce pendente, à espera da gestão (§8).
func (s *Service) SolicitarPedido(ctx context.Context, identity auth.Identity, cidadao uuid.UUID, tipo, justificativa string) (domain.PedidoTitular, error) {
	if tipo != domain.PedidoCorrecao && tipo != domain.PedidoAnonimizacao {
		return domain.PedidoTitular{}, &domain.ValidationError{Msg: "tipo de pedido inválido (correcao ou anonimizacao)"}
	}
	if domain.AreaDe(identity, auth.PermSemprasAtendimento).Vazia() {
		return domain.PedidoTitular{}, domain.ErrForbidden
	}
	p, err := domain.NovoPedido(cidadao, identity.UserID, tipo, justificativa)
	if err != nil {
		return domain.PedidoTitular{}, err
	}
	if tipo == domain.PedidoCorrecao {
		now := s.now()
		p.Status, p.DecididoPor, p.DecididoEm = domain.PedidoStatusConcluido, &identity.UserID, &now
	}
	p.CreatedAt = s.now()
	err = database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := s.repo.GetCidadao(ctx, tx, cidadao, false); err != nil {
			return err
		}
		if err := s.repo.InsertPedido(ctx, tx, p); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.titular.pedido_criado", "sempras_pedido_titular", p.ID.String(), nil,
			map[string]any{"cidadao_id": cidadao, "tipo": tipo})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.PedidoTitular{}, err
	}
	return p, nil
}

// ListarPedidosPendentes é a fila de decisão da gestão — exige cobertura
// TOTAL (não um recorte de unidade): o cadastro do titular é
// intersecretaria, a decisão de anonimizá-lo também precisa ser.
func (s *Service) ListarPedidosPendentes(ctx context.Context, identity auth.Identity) ([]domain.PedidoTitular, error) {
	if !domain.AreaDe(identity, auth.PermSemprasGestao).All {
		return nil, domain.ErrForbidden
	}
	return s.repo.ListPedidosPendentes(ctx, s.pool)
}

// DecidirPedido aprova ou nega uma anonimização pendente. Aprovar zera os
// campos identificadores do cadastro (fica o registro do serviço
// prestado); negar só grava a justificativa — o cadastro não muda.
func (s *Service) DecidirPedido(ctx context.Context, identity auth.Identity, id uuid.UUID, aprovar bool, justificativa string) (domain.PedidoTitular, error) {
	if !domain.AreaDe(identity, auth.PermSemprasGestao).All {
		return domain.PedidoTitular{}, domain.ErrForbidden
	}
	just, err := domain.JustificativaDecisaoValida(justificativa, !aprovar)
	if err != nil {
		return domain.PedidoTitular{}, err
	}
	var p domain.PedidoTitular
	now := s.now()
	err = database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		if p, err = s.repo.GetPedido(ctx, tx, id, true); err != nil {
			return err
		}
		if p.Tipo != domain.PedidoAnonimizacao || p.Status != domain.PedidoStatusPendente {
			return domain.ErrPedidoDecidido
		}
		status := domain.PedidoStatusNegado
		if aprovar {
			status = domain.PedidoStatusConcluido
			if err := s.repo.AnonimizarCidadao(ctx, tx, p.CidadaoID); err != nil {
				return err
			}
			ea := audit.Meta(ctx, "sempras.cidadao.anonimizado", "sempras_cidadao", p.CidadaoID.String(), nil,
				map[string]any{"campos": camposAnonimizados, "pedido_id": p.ID})
			if err := audit.NewWriter(tx).Record(ctx, ea); err != nil {
				return err
			}
		}
		if err := s.repo.DecidirPedido(ctx, tx, id, status, identity.UserID, just, now); err != nil {
			return err
		}
		p.Status, p.JustificativaDecisao, p.DecididoPor, p.DecididoEm = status, just, &identity.UserID, &now
		e := audit.Meta(ctx, "sempras.titular.pedido_decidido", "sempras_pedido_titular", id.String(), nil,
			map[string]any{"aprovado": aprovar})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.PedidoTitular{}, err
	}
	return p, nil
}

// ------------------------------------------------------------ rede de apoio

// AdicionarContatoRedeApoio inclui uma pessoa de apoio do munícipe.
func (s *Service) AdicionarContatoRedeApoio(ctx context.Context, identity auth.Identity, cidadao uuid.UUID, nome, parentesco, telefone string) (domain.ContatoRedeApoio, error) {
	if domain.AreaDe(identity, auth.PermSemprasAtendimento).Vazia() {
		return domain.ContatoRedeApoio{}, domain.ErrForbidden
	}
	c, err := domain.NovoContatoRedeApoio(cidadao, nome, parentesco, telefone)
	if err != nil {
		return domain.ContatoRedeApoio{}, err
	}
	c.CreatedAt = s.now()
	err = database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := s.repo.GetCidadao(ctx, tx, cidadao, false); err != nil {
			return err
		}
		if err := s.repo.InsertContatoRedeApoio(ctx, tx, c, identity.UserID); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.rede_apoio.adicionado", "sempras_rede_apoio", c.ID.String(), nil,
			map[string]any{"cidadao_id": cidadao})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.ContatoRedeApoio{}, err
	}
	return c, nil
}

// RemoverContatoRedeApoio remove um contato da rede de apoio.
func (s *Service) RemoverContatoRedeApoio(ctx context.Context, identity auth.Identity, id uuid.UUID) error {
	if domain.AreaDe(identity, auth.PermSemprasAtendimento).Vazia() {
		return domain.ErrForbidden
	}
	return database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		c, err := s.repo.GetContatoRedeApoio(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.repo.DeleteContatoRedeApoio(ctx, tx, id); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.rede_apoio.removido", "sempras_rede_apoio", id.String(), nil,
			map[string]any{"cidadao_id": c.CidadaoID})
		return audit.NewWriter(tx).Record(ctx, e)
	})
}

// -------------------------------------------------- benefícios eventuais

// SolicitarBeneficio registra um pedido de benefício eventual (LOAS art.
// 22) — o solicitante pode ser outra pessoa da família do beneficiário.
func (s *Service) SolicitarBeneficio(ctx context.Context, identity auth.Identity, beneficiario uuid.UUID, solicitante *uuid.UUID, unidade uuid.UUID, tipo, formaAcesso string, quantidade int, observacao string) (domain.Beneficio, error) {
	b, err := domain.NovoBeneficio(beneficiario, solicitante, unidade, tipo, formaAcesso, quantidade, observacao)
	if err != nil {
		return domain.Beneficio{}, err
	}
	b.DataSolicitacao = s.now()
	err = database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		u, err := s.unidade(ctx, tx, identity, unidade, domain.ServicoBeneficio, auth.PermSemprasAtendimento)
		if err != nil {
			return err
		}
		if !u.Ativa {
			return domain.ErrUnidadeInativa
		}
		if _, err := s.repo.GetCidadao(ctx, tx, beneficiario, false); err != nil {
			return err
		}
		if solicitante != nil {
			if _, err := s.repo.GetCidadao(ctx, tx, *solicitante, false); err != nil {
				return err
			}
		}
		if err := s.repo.InsertBeneficio(ctx, tx, b, identity.UserID); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.beneficio.solicitado", "sempras_beneficio", b.ID.String(), nil,
			map[string]any{"beneficiario_id": beneficiario, "unidade_id": unidade, "tipo": tipo})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.Beneficio{}, err
	}
	return b, nil
}

// DecidirBeneficio concede, nega ou suspende — exige atendimento na
// unidade que recebeu o pedido (quem decide é quem está na ponta, não a
// gestão central).
func (s *Service) DecidirBeneficio(ctx context.Context, identity auth.Identity, id uuid.UUID, acao, motivo string) (domain.Beneficio, error) {
	var b domain.Beneficio
	now := s.now()
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		if b, err = s.repo.GetBeneficio(ctx, tx, id, true); err != nil {
			return err
		}
		if _, err := s.unidade(ctx, tx, identity, b.UnidadeID, domain.ServicoBeneficio, auth.PermSemprasAtendimento); err != nil {
			return err
		}
		antes := b.Status
		if err := b.Decidir(acao, motivo, identity.UserID, now); err != nil {
			return err
		}
		if err := s.repo.UpdateBeneficio(ctx, tx, b); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.beneficio."+acao, "sempras_beneficio", id.String(),
			map[string]any{"status": antes}, map[string]any{"status": b.Status})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.Beneficio{}, err
	}
	return b, nil
}

// --------------------------------------------------------- encaminhamentos

// Encaminhar passa o caso de uma unidade a outra (ex.: CRAS → CREAS). Exige
// atendimento na unidade de origem; a de destino só precisa existir e
// estar ativa — quem envia não precisa ter acesso a ela.
func (s *Service) Encaminhar(ctx context.Context, identity auth.Identity, cidadao, origem, destino uuid.UUID, motivo string) (domain.Encaminhamento, error) {
	e, err := domain.NovoEncaminhamento(cidadao, origem, destino, motivo)
	if err != nil {
		return domain.Encaminhamento{}, err
	}
	e.CreatedAt = s.now()
	err = database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		uOrigem, err := s.unidade(ctx, tx, identity, origem, domain.ServicoEncaminhamento, auth.PermSemprasAtendimento)
		if err != nil {
			return err
		}
		if !uOrigem.Ativa {
			return domain.ErrUnidadeInativa
		}
		uDestino, err := s.repo.GetUnidade(ctx, tx, destino)
		if err != nil {
			return err
		}
		if !uDestino.Oferta(domain.ServicoEncaminhamento) {
			return domain.ErrServicoIndisponivel
		}
		if !uDestino.Ativa {
			return domain.ErrUnidadeInativa
		}
		if _, err := s.repo.GetCidadao(ctx, tx, cidadao, false); err != nil {
			return err
		}
		if err := s.repo.InsertEncaminhamento(ctx, tx, e, identity.UserID); err != nil {
			return err
		}
		ev := audit.Meta(ctx, "sempras.encaminhamento.criado", "sempras_encaminhamento", e.ID.String(), nil,
			map[string]any{"cidadao_id": cidadao, "unidade_origem_id": origem, "unidade_destino_id": destino})
		return audit.NewWriter(tx).Record(ctx, ev)
	})
	if err != nil {
		return domain.Encaminhamento{}, err
	}
	return e, nil
}

// EncaminhamentosPendentes é a fila de casos recebidos por uma unidade.
func (s *Service) EncaminhamentosPendentes(ctx context.Context, identity auth.Identity, unidade uuid.UUID) ([]domain.Encaminhamento, error) {
	if _, err := s.unidade(ctx, s.pool, identity, unidade, domain.ServicoEncaminhamento, auth.PermSemprasAtendimento); err != nil {
		return nil, err
	}
	return s.repo.ListEncaminhamentosPendentes(ctx, s.pool, unidade)
}

// DecidirEncaminhamento aceita ou recusa — exige atendimento na unidade de
// destino (quem recebe decide, não quem enviou).
func (s *Service) DecidirEncaminhamento(ctx context.Context, identity auth.Identity, id uuid.UUID, aceitar bool, observacao string) (domain.Encaminhamento, error) {
	var e domain.Encaminhamento
	now := s.now()
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		if e, err = s.repo.GetEncaminhamento(ctx, tx, id, true); err != nil {
			return err
		}
		if _, err := s.unidade(ctx, tx, identity, e.UnidadeDestinoID, domain.ServicoEncaminhamento, auth.PermSemprasAtendimento); err != nil {
			return err
		}
		antes := e.Status
		if err := e.Decidir(aceitar, observacao, identity.UserID, now); err != nil {
			return err
		}
		if err := s.repo.UpdateEncaminhamento(ctx, tx, e); err != nil {
			return err
		}
		ev := audit.Meta(ctx, "sempras.encaminhamento.decidido", "sempras_encaminhamento", id.String(),
			map[string]any{"status": antes}, map[string]any{"status": e.Status})
		return audit.NewWriter(tx).Record(ctx, ev)
	})
	if err != nil {
		return domain.Encaminhamento{}, err
	}
	return e, nil
}

// ------------------------------------------------------------------- PAIF

// IncluirPaif vincula a família ao Serviço de Proteção e Atendimento
// Integral à Família do CRAS. ErrPaifJaAtivo se já houver vínculo ativo.
func (s *Service) IncluirPaif(ctx context.Context, identity auth.Identity, cidadao, unidade uuid.UUID, tipoAtendimento string, extremaPobreza, bolsaFamilia, bpc bool, scfv int) (domain.PaifVinculo, error) {
	v, err := domain.NovoPaifVinculo(cidadao, unidade, tipoAtendimento, extremaPobreza, bolsaFamilia, bpc, scfv)
	if err != nil {
		return domain.PaifVinculo{}, err
	}
	v.DataInclusao = s.now()
	err = database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		u, err := s.unidade(ctx, tx, identity, unidade, domain.ServicoPaif, auth.PermSemprasAtendimento)
		if err != nil {
			return err
		}
		if !u.Ativa {
			return domain.ErrUnidadeInativa
		}
		if _, err := s.repo.GetCidadao(ctx, tx, cidadao, false); err != nil {
			return err
		}
		if err := s.repo.InsertPaifVinculo(ctx, tx, v, identity.UserID); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.paif.incluido", "sempras_paif_vinculo", v.ID.String(), nil,
			map[string]any{"cidadao_id": cidadao, "unidade_id": unidade})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.PaifVinculo{}, err
	}
	return v, nil
}

// ExcluirPaif encerra um vínculo PAIF ativo (exige atendimento na unidade do vínculo).
func (s *Service) ExcluirPaif(ctx context.Context, identity auth.Identity, id uuid.UUID, motivo string) (domain.PaifVinculo, error) {
	var v domain.PaifVinculo
	now := s.now()
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		if v, err = s.repo.GetPaifVinculo(ctx, tx, id, true); err != nil {
			return err
		}
		if _, err := s.unidade(ctx, tx, identity, v.UnidadeID, domain.ServicoPaif, auth.PermSemprasAtendimento); err != nil {
			return err
		}
		if err := v.Excluir(motivo, identity.UserID, now); err != nil {
			return err
		}
		if err := s.repo.UpdatePaifVinculo(ctx, tx, v); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.paif.excluido", "sempras_paif_vinculo", id.String(), nil,
			map[string]any{"cidadao_id": v.CidadaoID})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.PaifVinculo{}, err
	}
	return v, nil
}

// ------------------------------------------------------- Conselho Tutelar

// RegistrarConselhoAtendimento abre um atendimento do Conselho Tutelar a
// uma criança/adolescente (nasce em_andamento, sem medida).
func (s *Service) RegistrarConselhoAtendimento(ctx context.Context, identity auth.Identity, crianca, unidade uuid.UUID, respNome, respCPF, respTelefone, origemDemanda, motivoViolacao, observacoes string) (domain.ConselhoAtendimento, error) {
	c, err := domain.NovoConselhoAtendimento(crianca, unidade, respNome, respCPF, respTelefone, origemDemanda, motivoViolacao, observacoes)
	if err != nil {
		return domain.ConselhoAtendimento{}, err
	}
	c.CreatedAt = s.now()
	err = database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		u, err := s.unidade(ctx, tx, identity, unidade, domain.ServicoConselho, auth.PermSemprasAtendimento)
		if err != nil {
			return err
		}
		if !u.Ativa {
			return domain.ErrUnidadeInativa
		}
		if _, err := s.repo.GetCidadao(ctx, tx, crianca, false); err != nil {
			return err
		}
		if err := s.repo.InsertConselhoAtendimento(ctx, tx, c, identity.UserID); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.conselho.registrado", "sempras_conselho_atendimento", c.ID.String(), nil,
			map[string]any{"crianca_id": crianca, "unidade_id": unidade, "protocolo": c.Protocolo})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.ConselhoAtendimento{}, err
	}
	return c, nil
}

// TransitarConselhoAtendimento aplica medida (Art. 101 ECA), encaminha ao
// MP ou arquiva — exige atendimento na unidade do caso.
func (s *Service) TransitarConselhoAtendimento(ctx context.Context, identity auth.Identity, id uuid.UUID, acao, medida string) (domain.ConselhoAtendimento, error) {
	var c domain.ConselhoAtendimento
	now := s.now()
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		if c, err = s.repo.GetConselhoAtendimento(ctx, tx, id, true); err != nil {
			return err
		}
		if _, err := s.unidade(ctx, tx, identity, c.UnidadeID, domain.ServicoConselho, auth.PermSemprasAtendimento); err != nil {
			return err
		}
		antes := c.Status
		if err := c.Transitar(acao, medida, identity.UserID, now); err != nil {
			return err
		}
		if err := s.repo.UpdateConselhoAtendimento(ctx, tx, c); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.conselho."+acao, "sempras_conselho_atendimento", id.String(),
			map[string]any{"status": antes}, map[string]any{"status": c.Status, "medida_protetiva": c.MedidaProtetiva})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.ConselhoAtendimento{}, err
	}
	return c, nil
}

// ------------------------------------------------------------------- PAEFI

// RegistrarPaefiCaso abre um caso do PAEFI para a família/indivíduo (nasce
// em_acompanhamento).
func (s *Service) RegistrarPaefiCaso(ctx context.Context, identity auth.Identity, cidadao, unidade uuid.UUID, violacoes []string, membrosVulneraveis, observacoes string) (domain.PaefiCaso, error) {
	p, err := domain.NovoPaefiCaso(cidadao, unidade, violacoes, membrosVulneraveis, observacoes)
	if err != nil {
		return domain.PaefiCaso{}, err
	}
	p.DataAbertura = s.now()
	err = database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		u, err := s.unidade(ctx, tx, identity, unidade, domain.ServicoPaefi, auth.PermSemprasAtendimento)
		if err != nil {
			return err
		}
		if !u.Ativa {
			return domain.ErrUnidadeInativa
		}
		if _, err := s.repo.GetCidadao(ctx, tx, cidadao, false); err != nil {
			return err
		}
		if err := s.repo.InsertPaefiCaso(ctx, tx, p, identity.UserID); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.paefi.registrado", "sempras_paefi_caso", p.ID.String(), nil,
			map[string]any{"cidadao_id": cidadao, "unidade_id": unidade, "caso_codigo": p.CasoCodigo})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.PaefiCaso{}, err
	}
	return p, nil
}

// MarcarPiaElaborado registra a elaboração do PIA de um caso em acompanhamento.
func (s *Service) MarcarPiaElaborado(ctx context.Context, identity auth.Identity, id uuid.UUID) (domain.PaefiCaso, error) {
	var p domain.PaefiCaso
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		if p, err = s.repo.GetPaefiCaso(ctx, tx, id, true); err != nil {
			return err
		}
		if _, err := s.unidade(ctx, tx, identity, p.UnidadeID, domain.ServicoPaefi, auth.PermSemprasAtendimento); err != nil {
			return err
		}
		if err := p.MarcarPiaElaborado(identity.UserID); err != nil {
			return err
		}
		if err := s.repo.UpdatePaefiCaso(ctx, tx, p); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.paefi.pia_elaborado", "sempras_paefi_caso", id.String(), nil, nil)
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.PaefiCaso{}, err
	}
	return p, nil
}

// EncerrarPaefiCaso desliga o caso do PAEFI — exige motivo.
func (s *Service) EncerrarPaefiCaso(ctx context.Context, identity auth.Identity, id uuid.UUID, motivo string) (domain.PaefiCaso, error) {
	var p domain.PaefiCaso
	now := s.now()
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		if p, err = s.repo.GetPaefiCaso(ctx, tx, id, true); err != nil {
			return err
		}
		if _, err := s.unidade(ctx, tx, identity, p.UnidadeID, domain.ServicoPaefi, auth.PermSemprasAtendimento); err != nil {
			return err
		}
		if err := p.Encerrar(motivo, identity.UserID, now); err != nil {
			return err
		}
		if err := s.repo.UpdatePaefiCaso(ctx, tx, p); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.paefi.encerrado", "sempras_paefi_caso", id.String(), nil,
			map[string]any{"motivo_encerramento": p.MotivoEncerramento})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.PaefiCaso{}, err
	}
	return p, nil
}

// ------------------------------------------- Centro POP: abordagem social

// RegistrarAbordagem registra uma abordagem social da equipe do Centro POP.
func (s *Service) RegistrarAbordagem(ctx context.Context, identity auth.Identity, unidade uuid.UUID, in domain.AbordagemInput) (domain.Abordagem, error) {
	a, err := domain.NovaAbordagem(unidade, in, s.now())
	if err != nil {
		return domain.Abordagem{}, err
	}
	err = database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		u, err := s.unidade(ctx, tx, identity, unidade, domain.ServicoCentroPop, auth.PermSemprasAtendimento)
		if err != nil {
			return err
		}
		if !u.Ativa {
			return domain.ErrUnidadeInativa
		}
		if a.CidadaoID != nil {
			if _, err := s.repo.GetCidadao(ctx, tx, *a.CidadaoID, false); err != nil {
				return err
			}
		}
		if err := s.repo.InsertAbordagem(ctx, tx, a, identity.UserID); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.abordagem.registrada", "sempras_abordagem", a.ID.String(), nil,
			map[string]any{"unidade_id": unidade, "cidadao_id": a.CidadaoID, "resultado": a.Resultado})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.Abordagem{}, err
	}
	return a, nil
}

// ListarAbordagens lista as abordagens da unidade num mês ("2026-10").
func (s *Service) ListarAbordagens(ctx context.Context, identity auth.Identity, unidade uuid.UUID, competencia string) ([]domain.Abordagem, error) {
	inicio, fim, err := domain.CompetenciaMes(competencia)
	if err != nil {
		return nil, err
	}
	if _, err := s.unidade(ctx, s.pool, identity, unidade, domain.ServicoCentroPop, auth.PermSemprasAtendimento); err != nil {
		return nil, err
	}
	return s.repo.ListAbordagens(ctx, s.pool, unidade, inicio, fim)
}

// ---------------------------------------------------------- Casa da Mulher

// RegistrarCasoCasaMulher abre o caso de acompanhamento da mulher (nasce ativo).
func (s *Service) RegistrarCasoCasaMulher(ctx context.Context, identity auth.Identity, cidadao, unidade uuid.UUID, in domain.CasoCasaMulherInput) (domain.CasoCasaMulher, error) {
	c, err := domain.NovoCasoCasaMulher(cidadao, unidade, in, s.now())
	if err != nil {
		return domain.CasoCasaMulher{}, err
	}
	err = database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		u, err := s.unidade(ctx, tx, identity, unidade, domain.ServicoCasaMulher, auth.PermSemprasAtendimento)
		if err != nil {
			return err
		}
		if !u.Ativa {
			return domain.ErrUnidadeInativa
		}
		if _, err := s.repo.GetCidadao(ctx, tx, cidadao, false); err != nil {
			return err
		}
		if err := s.repo.InsertCasoCasaMulher(ctx, tx, c, identity.UserID); err != nil {
			return err
		}
		// Sem dado do caso na trilha: só identificadores e o risco.
		e := audit.Meta(ctx, "sempras.casa_mulher.registrado", "sempras_casa_mulher_caso", c.ID.String(), nil,
			map[string]any{"cidadao_id": cidadao, "unidade_id": unidade, "caso_codigo": c.CasoCodigo, "nivel_risco": c.NivelRisco})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.CasoCasaMulher{}, err
	}
	return c, nil
}

// AtualizarCasoCasaMulher regrava a avaliação de um caso ativo.
func (s *Service) AtualizarCasoCasaMulher(ctx context.Context, identity auth.Identity, id uuid.UUID, in domain.CasoCasaMulherInput) (domain.CasoCasaMulher, error) {
	var c domain.CasoCasaMulher
	now := s.now()
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		if c, err = s.repo.GetCasoCasaMulher(ctx, tx, id, true); err != nil {
			return err
		}
		if _, err := s.unidade(ctx, tx, identity, c.UnidadeID, domain.ServicoCasaMulher, auth.PermSemprasAtendimento); err != nil {
			return err
		}
		antes := c.NivelRisco
		if err := c.Atualizar(in, identity.UserID, now); err != nil {
			return err
		}
		if err := s.repo.UpdateCasoCasaMulher(ctx, tx, c); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.casa_mulher.atualizado", "sempras_casa_mulher_caso", id.String(),
			map[string]any{"nivel_risco": antes}, map[string]any{"nivel_risco": c.NivelRisco, "abrigada": c.Abrigada})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.CasoCasaMulher{}, err
	}
	return c, nil
}

// EncerrarCasoCasaMulher fecha o caso — exige motivo.
func (s *Service) EncerrarCasoCasaMulher(ctx context.Context, identity auth.Identity, id uuid.UUID, motivo string) (domain.CasoCasaMulher, error) {
	var c domain.CasoCasaMulher
	now := s.now()
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		if c, err = s.repo.GetCasoCasaMulher(ctx, tx, id, true); err != nil {
			return err
		}
		if _, err := s.unidade(ctx, tx, identity, c.UnidadeID, domain.ServicoCasaMulher, auth.PermSemprasAtendimento); err != nil {
			return err
		}
		if err := c.Encerrar(motivo, identity.UserID, now); err != nil {
			return err
		}
		if err := s.repo.UpdateCasoCasaMulher(ctx, tx, c); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.casa_mulher.encerrado", "sempras_casa_mulher_caso", id.String(), nil,
			map[string]any{"motivo_encerramento": c.MotivoEncerramento})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.CasoCasaMulher{}, err
	}
	return c, nil
}

// --------------------------------------------------------------------- MSE

// RegistrarMseMedida abre o acompanhamento de uma medida socioeducativa em
// meio aberto (nasce em_cumprimento).
func (s *Service) RegistrarMseMedida(ctx context.Context, identity auth.Identity, adolescente, unidade uuid.UUID, numeroProcesso, varaOrigem, atoInfracional, tipoMedida, instituicaoParceira string) (domain.MseMedida, error) {
	m, err := domain.NovaMseMedida(adolescente, unidade, numeroProcesso, varaOrigem, atoInfracional, tipoMedida, instituicaoParceira)
	if err != nil {
		return domain.MseMedida{}, err
	}
	m.DataInicio = s.now()
	err = database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		u, err := s.unidade(ctx, tx, identity, unidade, domain.ServicoMse, auth.PermSemprasAtendimento)
		if err != nil {
			return err
		}
		if !u.Ativa {
			return domain.ErrUnidadeInativa
		}
		if _, err := s.repo.GetCidadao(ctx, tx, adolescente, false); err != nil {
			return err
		}
		if err := s.repo.InsertMseMedida(ctx, tx, m, identity.UserID); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.mse.registrada", "sempras_mse_medida", m.ID.String(), nil,
			map[string]any{"adolescente_id": adolescente, "unidade_id": unidade, "tipo_medida": m.TipoMedida})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.MseMedida{}, err
	}
	return m, nil
}

// ConcluirMseMedida encerra o acompanhamento com um dos desfechos (cumprida,
// descumprida ou extinta).
func (s *Service) ConcluirMseMedida(ctx context.Context, identity auth.Identity, id uuid.UUID, resultado string) (domain.MseMedida, error) {
	var m domain.MseMedida
	now := s.now()
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		if m, err = s.repo.GetMseMedida(ctx, tx, id, true); err != nil {
			return err
		}
		if _, err := s.unidade(ctx, tx, identity, m.UnidadeID, domain.ServicoMse, auth.PermSemprasAtendimento); err != nil {
			return err
		}
		if err := m.Concluir(resultado, identity.UserID, now); err != nil {
			return err
		}
		if err := s.repo.UpdateMseMedida(ctx, tx, m); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.mse.concluida", "sempras_mse_medida", id.String(), nil,
			map[string]any{"status": m.Status})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.MseMedida{}, err
	}
	return m, nil
}

// ----------------------------------------------------- notificações (CREAS)

// RegistrarNotificacaoCreas registra um ofício/requisição recebido de um
// ente externo sobre uma vítima já cadastrada (nasce recebida).
func (s *Service) RegistrarNotificacaoCreas(ctx context.Context, identity auth.Identity, vitima, unidade uuid.UUID, origemEntidade, assunto string, prazoResposta *time.Time) (domain.NotificacaoCreas, error) {
	n, err := domain.NovaNotificacaoCreas(vitima, unidade, origemEntidade, assunto, prazoResposta)
	if err != nil {
		return domain.NotificacaoCreas{}, err
	}
	n.DataEntrada = s.now()
	err = database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		u, err := s.unidade(ctx, tx, identity, unidade, domain.ServicoNotificacao, auth.PermSemprasAtendimento)
		if err != nil {
			return err
		}
		if !u.Ativa {
			return domain.ErrUnidadeInativa
		}
		if _, err := s.repo.GetCidadao(ctx, tx, vitima, false); err != nil {
			return err
		}
		if err := s.repo.InsertNotificacaoCreas(ctx, tx, n, identity.UserID); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.creas_notificacao.registrada", "sempras_creas_notificacao", n.ID.String(), nil,
			map[string]any{"vitima_id": vitima, "unidade_id": unidade, "protocolo": n.Protocolo, "origem_entidade": n.OrigemEntidade})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.NotificacaoCreas{}, err
	}
	return n, nil
}

// TransitarNotificacaoCreas inicia diligência, registra a resposta ou arquiva a notificação.
func (s *Service) TransitarNotificacaoCreas(ctx context.Context, identity auth.Identity, id uuid.UUID, acao, resposta string) (domain.NotificacaoCreas, error) {
	var n domain.NotificacaoCreas
	now := s.now()
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		if n, err = s.repo.GetNotificacaoCreas(ctx, tx, id, true); err != nil {
			return err
		}
		if _, err := s.unidade(ctx, tx, identity, n.UnidadeID, domain.ServicoNotificacao, auth.PermSemprasAtendimento); err != nil {
			return err
		}
		antes := n.Status
		if err := n.Transitar(acao, resposta, identity.UserID, now); err != nil {
			return err
		}
		if err := s.repo.UpdateNotificacaoCreas(ctx, tx, n); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.creas_notificacao."+acao, "sempras_creas_notificacao", id.String(),
			map[string]any{"status": antes}, map[string]any{"status": n.Status})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.NotificacaoCreas{}, err
	}
	return n, nil
}

// -------------------------------------------------------- ofícios (CRAS)

// RegistrarOficioCras registra um ofício recebido de um ente externo
// sobre uma família acompanhada pelo CRAS (nasce 'pendente').
func (s *Service) RegistrarOficioCras(ctx context.Context, identity auth.Identity, cidadao, unidade uuid.UUID, orgaoEmissor, numeroOficio string, dataOficio *time.Time, assunto string) (domain.OficioCras, error) {
	o, err := domain.NovoOficioCras(cidadao, unidade, orgaoEmissor, numeroOficio, dataOficio, assunto)
	if err != nil {
		return domain.OficioCras{}, err
	}
	o.DataRecebimento = s.now()
	err = database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		u, err := s.unidade(ctx, tx, identity, unidade, domain.ServicoOficio, auth.PermSemprasAtendimento)
		if err != nil {
			return err
		}
		if !u.Ativa {
			return domain.ErrUnidadeInativa
		}
		if _, err := s.repo.GetCidadao(ctx, tx, cidadao, false); err != nil {
			return err
		}
		if err := s.repo.InsertOficioCras(ctx, tx, o, identity.UserID); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.cras_oficio.registrado", "sempras_cras_oficio", o.ID.String(), nil,
			map[string]any{"cidadao_id": cidadao, "unidade_id": unidade, "orgao_emissor": o.OrgaoEmissor, "numero_oficio": o.NumeroOficio})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.OficioCras{}, err
	}
	return o, nil
}

// ResponderOficioCras registra a resposta de um ofício pendente.
func (s *Service) ResponderOficioCras(ctx context.Context, identity auth.Identity, id uuid.UUID, numeroResposta string) (domain.OficioCras, error) {
	var o domain.OficioCras
	now := s.now()
	err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		if o, err = s.repo.GetOficioCras(ctx, tx, id, true); err != nil {
			return err
		}
		if _, err := s.unidade(ctx, tx, identity, o.UnidadeID, domain.ServicoOficio, auth.PermSemprasAtendimento); err != nil {
			return err
		}
		if err := o.Responder(numeroResposta, now, identity.UserID); err != nil {
			return err
		}
		if err := s.repo.UpdateOficioCras(ctx, tx, o); err != nil {
			return err
		}
		e := audit.Meta(ctx, "sempras.cras_oficio.respondido", "sempras_cras_oficio", id.String(),
			map[string]any{"status": domain.OficioStatusPendente}, map[string]any{"status": o.Status, "numero_resposta": o.NumeroResposta})
		return audit.NewWriter(tx).Record(ctx, e)
	})
	if err != nil {
		return domain.OficioCras{}, err
	}
	return o, nil
}

// Authorize é o autorizador de tópicos WebSocket: "sempras:fila:<unidade>"
// para quem tem qualquer permissão do módulo na unidade.
func (s *Service) Authorize(ctx context.Context, client ws.ClientInfo, topic string) error {
	idStr, ok := strings.CutPrefix(topic, "sempras:fila:")
	if !ok {
		return ws.ErrTopicForbidden
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return ws.ErrTopicForbidden
	}
	if _, err := s.unidade(ctx, s.pool, client.Identity(), id, domain.ServicoFila, domain.TodasAsPermissoes...); err != nil {
		return ws.ErrTopicForbidden
	}
	return nil
}

// Inbound recusa frames do navegador: a fila só muda pela API.
func (s *Service) Inbound(context.Context, ws.ClientInfo, ws.Frame) error {
	return errors.New("o SEMPRAS não aceita frames do navegador")
}
