// Package application contém os casos de uso do SEMPRAS (ADR 030). Cada
// escrita grava o dado, o evento (outbox) e a auditoria na MESMA transação
// (ADR 016); o tempo real é publicado depois do commit, sem dado pessoal.
package application

import (
	"context"
	"errors"
	"log/slog"
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
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/outbox"
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
	pool   *pgxpool.Pool
	repo   domain.Repository
	outbox *outbox.Writer
	hub    Publisher
	logger *slog.Logger
	now    func() time.Time
}

// NewService cria o serviço.
func NewService(pool *pgxpool.Pool, repo domain.Repository, ob *outbox.Writer, hub Publisher, logger *slog.Logger) *Service {
	return &Service{pool: pool, repo: repo, outbox: ob, hub: hub, logger: logger, now: time.Now}
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
	case errors.Is(err, domain.ErrForbidden):
		return apperrors.Forbidden(err.Error())
	case errors.Is(err, domain.ErrInvalidState), errors.Is(err, domain.ErrFilaVazia), errors.Is(err, domain.ErrJaNaFila),
		errors.Is(err, domain.ErrDocumentoDuplicado):
		return apperrors.Conflict(err.Error())
	case errors.Is(err, domain.ErrUnidadeInativa), errors.Is(err, domain.ErrAtendimentoOutro):
		return apperrors.Validation(err.Error())
	}
	return err
}

// unidade carrega a unidade e exige alguma das permissões nela. Quem não
// alcança a unidade com permissão nenhuma do módulo recebe "não encontrado".
func (s *Service) unidade(ctx context.Context, db database.DBTX, identity auth.Identity, id uuid.UUID, perms ...auth.Permission) (domain.Unidade, error) {
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

// Unidades lista as unidades onde identity atua (seletor da tela).
func (s *Service) Unidades(ctx context.Context, identity auth.Identity) ([]domain.Unidade, error) {
	return s.repo.ListUnidades(ctx, s.pool, domain.AreaDe(identity, domain.TodasAsPermissoes...), EntidadeSlug)
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

// Atualizar regrava o cadastro (auditoria com os campos alterados).
func (s *Service) Atualizar(ctx context.Context, identity auth.Identity, id uuid.UUID, in domain.CidadaoInput) (domain.Cidadao, error) {
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
		if p.Cidadao, err = s.repo.GetCidadao(ctx, tx, id, false); err != nil {
			return err
		}
		if p.Atendimentos, err = s.repo.AtendimentosNaArea(ctx, tx, id, area); err != nil {
			return err
		}
		if p.Registros, err = s.repo.RegistrosNaArea(ctx, tx, id, area); err != nil {
			return err
		}
		unidades := map[uuid.UUID]domain.Unidade{}
		for i, r := range p.Registros {
			if r.Tipo != domain.TipoPsicologico {
				continue
			}
			u, ok := unidades[r.UnidadeID]
			if !ok {
				if u, err = s.repo.GetUnidade(ctx, tx, r.UnidadeID); err != nil {
					return err
				}
				unidades[r.UnidadeID] = u
			}
			if !u.PodePsicologia(identity) {
				p.Registros[i] = r.Ocultar()
			}
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
		u, err := s.unidade(ctx, tx, identity, in.UnidadeID, auth.PermSemprasAtendimento)
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
	CidadaoID        uuid.UUID
	Tipo, Prioridade string
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
		u, err := s.unidade(ctx, tx, identity, unidade, auth.PermSemprasRecepcao, auth.PermSemprasAtendimento)
		if err != nil {
			return err
		}
		if !u.Ativa {
			return domain.ErrUnidadeInativa
		}
		if c, err = s.repo.GetCidadao(ctx, tx, in.CidadaoID, false); err != nil {
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
			map[string]any{"status": a.Status, "unidade_id": u.ID, "cidadao_id": a.CidadaoID, "tipo": a.Tipo, "prioridade": a.Prioridade})
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
	if _, err := s.unidade(ctx, s.pool, identity, unidade, domain.TodasAsPermissoes...); err != nil {
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
		u, err := s.unidade(ctx, tx, identity, unidade, auth.PermSemprasAtendimento)
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
		u, err := s.unidade(ctx, tx, identity, a.UnidadeID, perms...)
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
	unidades, err := s.repo.ListUnidades(ctx, s.pool, area, EntidadeSlug)
	if err != nil {
		return nil, err
	}
	return s.repo.Painel(ctx, s.pool, unidades, s.dia())
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
	if _, err := s.unidade(ctx, s.pool, client.Identity(), id, domain.TodasAsPermissoes...); err != nil {
		return ws.ErrTopicForbidden
	}
	return nil
}

// Inbound recusa frames do navegador: a fila só muda pela API.
func (s *Service) Inbound(context.Context, ws.ClientInfo, ws.Frame) error {
	return errors.New("o SEMPRAS não aceita frames do navegador")
}
