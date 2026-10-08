package application

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
	"github.com/yurythx/projeto-nexus/internal/modules/atlas/domain"
	"github.com/yurythx/projeto-nexus/internal/platform/database"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
	"github.com/yurythx/projeto-nexus/internal/platform/outbox"
)

// fakeRepo entrega procedimentos e séries fixos ao grounding.
type fakeRepo struct {
	domain.Repository
	procs   []domain.Workflow
	series  []domain.ClassificacaoTTDD
	err     error
	errTTDD error
	errProc error
}

func (f fakeRepo) Candidatos(context.Context, database.DBTX, string, int) ([]domain.Workflow, error) {
	return f.procs, f.errProc
}

func (f fakeRepo) CandidatosTTDD(context.Context, database.DBTX, string, int) ([]domain.ClassificacaoTTDD, error) {
	return f.series, f.errTTDD
}

// ModelosDaSerie: as séries do fake não têm modelos ligados.
func (f fakeRepo) ModelosDaSerie(context.Context, database.DBTX, string) ([]domain.Modelo, error) {
	return nil, nil
}

func (f fakeRepo) ListTTDD(context.Context, database.DBTX, domain.FiltroTTDD, pagination.Params) ([]domain.ClassificacaoTTDD, int64, error) {
	return nil, 0, f.err
}

type fakeAssistente struct {
	answer   string
	err      error
	contexto []string
}

func (f *fakeAssistente) Responder(_ context.Context, _ string, ctx []string) (string, error) {
	f.contexto = ctx
	return f.answer, f.err
}

func serie(codigo, descritor string, corrente, interm int, dest domain.DestinacaoFinal) domain.ClassificacaoTTDD {
	return domain.ClassificacaoTTDD{Codigo: codigo, Descritor: descritor, FaseCorrenteAnos: &corrente, FaseIntermAnos: &interm, DestinacaoFinal: &dest}
}

var (
	pasta        = serie("2.0.07.00.00", "Pasta funcional de Servidores Ativos, inativos e aposentados", 1, 99, domain.DestinacaoEliminacao)
	organogramas = serie("2.0.01.00.01", "Organogramas", 1, 1, domain.DestinacaoEliminacao)
)

// O assistente responde SÓ sobre a TTDD (ADR 021).
func TestPerguntar(t *testing.T) {
	pool := dbtest.Pool(t) // a consulta é auditada
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()
	repo := fakeRepo{series: []domain.ClassificacaoTTDD{organogramas, pasta}}
	const pergunta = "  Qual o prazo de guarda da pasta funcional de servidores?  "
	svc := func(ia domain.Assistente, r fakeRepo) *Service {
		return NewService(pool, r, outbox.NewWriter("t"), ia, logger)
	}

	t.Run("IA responde só com a série acima do limiar", func(t *testing.T) {
		ia := &fakeAssistente{answer: "Orientação do modelo"}
		r, err := svc(ia, repo).Perguntar(ctx, pergunta)
		if err != nil || r.Mode != ModoIA || r.Answer != "Orientação do modelo" || r.Refused {
			t.Fatalf("resposta = %+v, %v", r, err)
		}
		if len(ia.contexto) != 1 || !strings.Contains(ia.contexto[0], "99 anos") || len(r.Sources) != 1 ||
			r.Sources[0].Tipo != FonteTTDD || r.Sources[0].Codigo != "2.0.07.00.00" {
			t.Fatalf("contexto enviado ao modelo: %+v / fontes %+v", ia.contexto, r.Sources)
		}
	})
	t.Run("modelo reconhece pedido fora do objetivo", func(t *testing.T) {
		ia := &fakeAssistente{answer: domain.MensagemForaDoObjetivo}
		r, err := svc(ia, repo).Perguntar(ctx, pergunta)
		if err != nil || !r.Refused || r.Mode != ModoRecusada || r.Answer != domain.MensagemForaDoObjetivo || len(r.Sources) != 0 {
			t.Fatalf("resposta = %+v, %v", r, err)
		}
	})
	for nome, ia := range map[string]domain.Assistente{
		"IA indisponível cai na síntese":   &fakeAssistente{err: errors.New("timeout")},
		"IA desligada na configuração":     &fakeAssistente{err: domain.ErrIADesligada},
		"sem IA configurada usa a síntese": nil,
	} {
		t.Run(nome, func(t *testing.T) {
			r, err := svc(ia, repo).Perguntar(ctx, pergunta)
			if err != nil || r.Mode != ModoSintese || !strings.Contains(r.Answer, "2.0.07.00.00") || !strings.Contains(r.Answer, "eliminação") {
				t.Fatalf("resposta = %+v, %v", r, err)
			}
		})
	}
	// Recusas sem chamar o modelo: assunto fora do objetivo, ou sobre
	// temporalidade sem série correspondente (nenhuma ou abaixo do limiar).
	for pergunta, want := range map[string]string{
		"receita de bolo de cenoura":                  domain.MensagemForaDoObjetivo,
		"Como tramitar um processo de pregão?":        domain.MensagemSemFonte, // fluxo, mas sem procedimento
		"Me ajuda a escrever um ofício?":              domain.MensagemForaDoObjetivo,
		"qual o prazo de guarda dos alvarás de obra?": domain.MensagemSemFonte,
		"organogramas antigos da secretaria de obras": domain.MensagemForaDoObjetivo, // palavra solta não basta
		"prazo para eliminar os mapas de obras":       domain.MensagemSemFonte,
	} {
		ia := &fakeAssistente{answer: "não deveria"}
		r, err := svc(ia, repo).Perguntar(ctx, pergunta)
		if err != nil || !r.Refused || r.Mode != ModoRecusada || r.Answer != want || ia.contexto != nil || len(r.Sources) != 0 {
			t.Errorf("%q: %+v, %v (modelo chamado: %v)", pergunta, r, err, ia.contexto != nil)
		}
	}
	t.Run("só as séries perto da melhor entram como fonte", func(t *testing.T) {
		aposentados := serie("2.0.07.00.09", "Servidores aposentados", 1, 1, domain.DestinacaoEliminacao)
		r, err := svc(nil, fakeRepo{series: []domain.ClassificacaoTTDD{aposentados, pasta}}).Perguntar(ctx, "pasta funcional servidores aposentados")
		if err != nil || len(r.Sources) != 1 || r.Sources[0].Codigo != "2.0.07.00.00" || strings.Contains(r.Answer, "2.0.07.00.09") {
			t.Fatalf("fonte abaixo da margem entrou: %+v", r.Sources)
		}
	})
	t.Run("empate: a série de nome mais específico vem antes", func(t *testing.T) {
		longa := serie("2.0.02.02.17", "Relatório de fiscal: em termos de guarda, processos de licitação e outros documentos", 1, 1, domain.DestinacaoGuardaPermanente)
		curta := serie("2.0.02.01.00", "Processos de licitação", 1, 1, domain.DestinacaoGuardaPermanente)
		r, err := svc(nil, fakeRepo{series: []domain.ClassificacaoTTDD{longa, curta}}).Perguntar(ctx, "processos de licitação")
		if err != nil || len(r.Sources) != 2 || r.Sources[0].Codigo != "2.0.02.01.00" {
			t.Fatalf("ordem das fontes: %+v %v", r.Sources, err)
		}
	})
	t.Run("falha do repositório propaga", func(t *testing.T) {
		for _, r := range []fakeRepo{{errTTDD: errors.New("db")}, {errProc: errors.New("db")}} {
			if _, err := svc(nil, r).Perguntar(ctx, pergunta); err == nil {
				t.Fatal("esperado erro")
			}
		}
	})
	// Fluxos (ADR 023): o procedimento do início ao fim; a pergunta escolhe o
	// tipo de fonte quando há candidato dele acima do limiar.
	pregao := domain.Workflow{ID: uuid.New(), CodigoProcessual: "ADM.LIC.001", Titulo: "Pregão Eletrônico", Objetivo: "Aquisição de bens comuns",
		Versao: 1, NivelAcesso: domain.NivelPublico, Etapas: []domain.Etapa{{Ordem: 1, NomeSetor: "Licitações", UnidadeAdministrativa: "LIC",
			AtribuicoesSetor: "Conduzir o certame", PrazoSLAEmDias: 10}}}
	pregaoSerie := serie("2.0.02.00.07", "Processos relativos a Pregão Eletrônico", 1, 4, domain.DestinacaoGuardaPermanente)
	ambos := fakeRepo{procs: []domain.Workflow{pregao}, series: []domain.ClassificacaoTTDD{pregaoSerie}}
	for q, want := range map[string][]string{
		"Como funciona o fluxo do pregão eletrônico?":                {"procedimento"},
		"Qual o prazo de guarda dos processos de pregão eletrônico?": {"ttdd"},
		"Quais as etapas e o prazo de guarda do pregão eletrônico?":  {"procedimento", "ttdd"},
		"pregão eletrônico": {"procedimento", "ttdd"},
	} {
		r, err := svc(nil, ambos).Perguntar(ctx, q)
		var tipos []string
		for _, f := range r.Sources {
			tipos = append(tipos, f.Tipo)
		}
		if err != nil || r.Refused || strings.Join(tipos, ",") != strings.Join(want, ",") {
			t.Errorf("%q: fontes %v (quero %v) %v", q, tipos, want, err)
		}
	}
	r, err := svc(nil, ambos).Perguntar(ctx, "Como funciona o fluxo do pregão eletrônico?")
	if err != nil || r.Sources[0].ID == nil || *r.Sources[0].ID != pregao.ID || !strings.Contains(r.Answer, "Fluxo em 1 etapa") ||
		!strings.Contains(r.Answer, "1. Licitações (LIC) — prazo: 10 dias") {
		t.Fatalf("fluxo: %+v %v", r, err)
	}
	// Pergunta de fluxo sem procedimento acima do limiar: a série responde.
	if r, err := svc(nil, fakeRepo{series: []domain.ClassificacaoTTDD{organogramas}}).Perguntar(ctx, "fluxo dos organogramas"); err != nil ||
		len(r.Sources) != 1 || r.Sources[0].Tipo != FonteTTDD {
		t.Fatalf("fluxo sem procedimento: %+v %v", r, err)
	}
}

func TestMapError(t *testing.T) {
	for in, want := range map[error]string{
		domain.InvalidError{Msg: "x"}: "x",
		domain.ErrNotFound:            "procedimento não encontrado",
		domain.ErrTTDDNotFound:        "classificação TTDD não encontrada",
		domain.ErrDuplicate:           "já existe procedimento com este código e versão",
	} {
		if got := MapError(in); got == nil || !strings.Contains(got.Error(), want) {
			t.Fatalf("MapError(%v) = %v", in, got)
		}
	}
	other := errors.New("outro")
	if MapError(other) != other || MapError(nil) != nil {
		t.Fatal("erros desconhecidos passam intactos")
	}
}

// A consulta é auditada: sem conseguir gravar a trilha, não há resposta.
func TestPerguntarFalhaNaAuditoria(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // o repositório falso ignora o contexto; a auditoria não
	svc := NewService(pool, fakeRepo{series: []domain.ClassificacaoTTDD{pasta}}, outbox.NewWriter("t"), nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	if _, err := svc.Perguntar(ctx, "prazo de guarda da pasta funcional"); err == nil {
		t.Fatal("falha ao auditar a consulta deve falhar a resposta")
	}
}
