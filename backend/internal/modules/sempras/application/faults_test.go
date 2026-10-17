package application_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/yurythx/projeto-assistencia-v2/internal/domain/errors"
	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/application"
	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/domain"
	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/infrastructure"
	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/transport"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/auth"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/database/dbtest"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/outbox"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/storage/storagetest"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/ws"
)

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

type publisher struct{ err error }

func (p publisher) Publish(context.Context, string, string, any) error { return p.err }

// cenario: uma unidade de cada tipo (ADR 031) e um profissional com todas
// as permissões nelas. unidade é um CRAS (o tipo com mais serviços); os
// serviços especializados usam a unidade do seu tipo.
type cenario struct {
	pool       *pgxpool.Pool
	unidade    uuid.UUID // CRAS
	unidade2   uuid.UUID // CREAS, destino de encaminhamento — identity não tem permissão nela, só nas outras
	unCreas    uuid.UUID
	unPop      uuid.UUID // Centro POP
	unAbrigo   uuid.UUID // Casa Abrigo
	unConselho uuid.UUID // Conselho Tutelar
	unMulher   uuid.UUID // Casa da Mulher
	identity   auth.Identity
	store      *storagetest.Memory
}

// novaUnidade cria uma unidade do tipo na entidade (tipo "" = sem tipo).
func novaUnidade(t *testing.T, pool *pgxpool.Pool, ent uuid.UUID, tipo string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	slug := "un-" + uuid.NewString()[:8]
	var un uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO unidades (entidade_id, nome, slug) VALUES ($1, $2, $2) RETURNING id`, ent, slug).Scan(&un); err != nil {
		t.Fatal(err)
	}
	if tipo != "" {
		if _, err := pool.Exec(ctx, `INSERT INTO sempras_unidades (unidade_id, tipo, painel_exibe_nome) VALUES ($1, $2, true)`, un, tipo); err != nil {
			t.Fatal(err)
		}
	}
	return un
}

func novoCenario(t *testing.T) cenario {
	t.Helper()
	pool := dbtest.Pool(t)
	ctx := context.Background()
	sfx := uuid.NewString()[:8]
	var ent uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO entidades (nome, slug) VALUES ($1, $1) RETURNING id`, "ent-"+sfx).Scan(&ent); err != nil {
		t.Fatal(err)
	}
	c := cenario{pool: pool, store: storagetest.New(),
		unidade:    novaUnidade(t, pool, ent, domain.UnidadeCras),
		unidade2:   novaUnidade(t, pool, ent, domain.UnidadeCreas),
		unCreas:    novaUnidade(t, pool, ent, domain.UnidadeCreas),
		unPop:      novaUnidade(t, pool, ent, domain.UnidadeCentroPop),
		unAbrigo:   novaUnidade(t, pool, ent, domain.UnidadeCasaAbrigo),
		unConselho: novaUnidade(t, pool, ent, domain.UnidadeConselhoTutelar),
		unMulher:   novaUnidade(t, pool, ent, domain.UnidadeCasaMulher),
	}
	c.identity = auth.Identity{UserID: dbtest.User(t, pool), Name: "Profissional"}
	for _, un := range []uuid.UUID{c.unidade, c.unCreas, c.unPop, c.unAbrigo, c.unConselho, c.unMulher} {
		c.identity.Scopes = append(c.identity.Scopes, auth.Scope{EntidadeID: &ent, UnidadeID: &un, Permissions: []string{"sempras:*"}})
	}
	c.identity.Permissions = auth.EffectivePermissions(c.identity.Scopes)
	return c
}

// so é uma identidade com só as permissões dadas em todas as unidades do
// cenário (a não ser a unidade2).
func (c cenario) so(perms ...string) auth.Identity {
	i := auth.Identity{UserID: c.identity.UserID}
	for _, un := range []uuid.UUID{c.unidade, c.unCreas, c.unPop, c.unAbrigo, c.unConselho, c.unMulher} {
		i.Scopes = append(i.Scopes, auth.Scope{UnidadeID: &un, Permissions: perms})
	}
	i.Permissions = auth.EffectivePermissions(i.Scopes)
	return i
}

// casoMulherInput é uma avaliação válida de um caso da Casa da Mulher.
func casoMulherInput() domain.CasoCasaMulherInput {
	return domain.CasoCasaMulherInput{Violencias: []string{"fisica", "psicologica"}, RelacaoAgressor: "ex_conjuge_companheiro",
		NivelRisco: "alto", MedidaProtetivaStatus: "em_analise", Filhos: 2}
}

// casoMulher cadastra uma mulher e abre um caso da Casa da Mulher para ela
// (fora da transação sob teste).
func (c cenario) casoMulher(t *testing.T) domain.CasoCasaMulher {
	t.Helper()
	caso, err := c.limpo().RegistrarCasoCasaMulher(context.Background(), c.identity, c.cadastrar(t).ID, c.unMulher, casoMulherInput())
	if err != nil {
		t.Fatal(err)
	}
	return caso
}

const bucket = "test-bucket"

// identityUnidade2 tem sempras:* só na unidade2 (destino de encaminhamento).
func (c cenario) identityUnidade2() auth.Identity {
	i := auth.Identity{UserID: c.identity.UserID, Scopes: []auth.Scope{{UnidadeID: &c.unidade2, Permissions: []string{"sempras:*"}}}}
	i.Permissions = auth.EffectivePermissions(i.Scopes)
	return i
}

func (c cenario) svc(repo domain.Repository, pub application.Publisher) *application.Service {
	return application.NewService(c.pool, repo, outbox.NewWriter("nexus.sempras"), pub, c.store, bucket, 1<<20, time.Minute, testLogger()).WithClock(time.Now)
}

func (c cenario) limpo() *application.Service {
	return c.svc(infrastructure.NewRepository(), publisher{})
}

func (c cenario) cadastrar(t *testing.T) domain.Cidadao {
	t.Helper()
	cid, err := c.limpo().Cadastrar(context.Background(), c.identity, domain.CidadaoInput{
		Nome: "Pessoa " + uuid.NewString()[:6], DataNascimento: "1990-01-01", NomeMae: "Mãe",
		// O CRAS exige CPF ou NIS na fila (ADR 031 §3); NIS aleatório não colide.
		NIS: fmt.Sprintf("%011d", rand.Int64N(1e11))})
	if err != nil {
		t.Fatal(err)
	}
	return cid
}

func (c cenario) incluir(t *testing.T, tipo string) domain.FilaItem {
	t.Helper()
	item, err := c.limpo().Incluir(context.Background(), c.identity, c.unidade,
		application.EntradaInput{CidadaoID: c.cadastrar(t).ID, Tipo: tipo, Demanda: "atendimento_tecnico"})
	if err != nil {
		t.Fatal(err)
	}
	return item
}

// pedirAnonimizacao cadastra um munícipe e registra um pedido de
// anonimização pendente (fora da transação sob teste — igual a incluir).
func (c cenario) pedirAnonimizacao(t *testing.T) domain.PedidoTitular {
	t.Helper()
	p, err := c.limpo().SolicitarPedido(context.Background(), c.identity, c.cadastrar(t).ID,
		domain.PedidoAnonimizacao, "titular pediu para ser esquecido")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// contatoRedeApoio cadastra um munícipe e um contato da rede de apoio dele
// (fora da transação sob teste — igual a incluir/pedirAnonimizacao).
func (c cenario) contatoRedeApoio(t *testing.T) uuid.UUID {
	t.Helper()
	contato, err := c.limpo().AdicionarContatoRedeApoio(context.Background(), c.identity, c.cadastrar(t).ID, "Maria", "Irmã", "")
	if err != nil {
		t.Fatal(err)
	}
	return contato.ID
}

// beneficio cadastra um munícipe e um benefício eventual solicitado para
// ele (fora da transação sob teste).
func (c cenario) beneficio(t *testing.T) domain.Beneficio {
	t.Helper()
	b, err := c.limpo().SolicitarBeneficio(context.Background(), c.identity, c.cadastrar(t).ID, nil, c.unidade, domain.BeneficioCestaBasica, domain.FormaAcessoDemandaEspontanea, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// encaminhamento cadastra um munícipe e um encaminhamento pendente de
// unidade para unidade2 (fora da transação sob teste).
func (c cenario) encaminhamento(t *testing.T) domain.Encaminhamento {
	t.Helper()
	e, err := c.limpo().Encaminhar(context.Background(), c.identity, c.cadastrar(t).ID, c.unidade, c.unidade2, "precisa de acompanhamento especializado")
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// paif cadastra um munícipe e um vínculo PAIF ativo dele (fora da
// transação sob teste).
func (c cenario) paif(t *testing.T) domain.PaifVinculo {
	t.Helper()
	v, err := c.limpo().IncluirPaif(context.Background(), c.identity, c.cadastrar(t).ID, c.unidade, domain.PaifAtendimentoParticularizado, true, false, false, 2)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// conselho cadastra uma criança e um atendimento do Conselho Tutelar dela
// (fora da transação sob teste).
func (c cenario) conselho(t *testing.T) domain.ConselhoAtendimento {
	t.Helper()
	ca, err := c.limpo().RegistrarConselhoAtendimento(context.Background(), c.identity, c.cadastrar(t).ID, c.unConselho,
		"Mãe da Criança", "", "", "escola", "evasao_escolar", "faltou 20 dias seguidos")
	if err != nil {
		t.Fatal(err)
	}
	return ca
}

// paefi cadastra um munícipe e um caso do PAEFI dele (fora da transação
// sob teste).
func (c cenario) paefi(t *testing.T) domain.PaefiCaso {
	t.Helper()
	p, err := c.limpo().RegistrarPaefiCaso(context.Background(), c.identity, c.cadastrar(t).ID, c.unCreas,
		[]string{"violencia_fisica"}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// mse cadastra um adolescente e uma medida socioeducativa dele (fora da
// transação sob teste).
func (c cenario) mse(t *testing.T) domain.MseMedida {
	t.Helper()
	m, err := c.limpo().RegistrarMseMedida(context.Background(), c.identity, c.cadastrar(t).ID, c.unCreas,
		"123/2026", "Vara da Infância", "furto (art. 155 CP)", domain.MseTipoLiberdadeAssistida, "")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// notificacaoCreas cadastra uma vítima e uma notificação do CREAS sobre
// ela (fora da transação sob teste).
func (c cenario) notificacaoCreas(t *testing.T) domain.NotificacaoCreas {
	t.Helper()
	n, err := c.limpo().RegistrarNotificacaoCreas(context.Background(), c.identity, c.cadastrar(t).ID, c.unCreas,
		"ministerio_publico", "requisição de informações sobre acompanhamento", nil)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// oficioCras cadastra um munícipe e um ofício do CRAS pendente (fora da
// transação sob teste).
func (c cenario) oficioCras(t *testing.T) domain.OficioCras {
	t.Helper()
	o, err := c.limpo().RegistrarOficioCras(context.Background(), c.identity, c.cadastrar(t).ID, c.unidade,
		"Conselho Tutelar", "123/2026", nil, "acompanhamento da família")
	if err != nil {
		t.Fatal(err)
	}
	return o
}

// acolhimento cadastra uma criança e um acolhimento dela na Casa Abrigo
// (fora da transação sob teste).
func (c cenario) acolhimento(t *testing.T) domain.Acolhimento {
	t.Helper()
	a, err := c.limpo().RegistrarAcolhimento(context.Background(), c.identity, c.cadastrar(t).ID, c.unAbrigo,
		application.AcolhimentoInput{Sexo: "masculino", Motivos: []string{"negligencia_abandono"}})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// irmao cadastra uma criança, um acolhimento e um irmão dela (fora da
// transação sob teste).
func (c cenario) irmao(t *testing.T) domain.Irmao {
	t.Helper()
	i, err := c.limpo().AdicionarIrmao(context.Background(), c.identity, c.acolhimento(t).ID, "Irmão da Criança", nil, false, "", "")
	if err != nil {
		t.Fatal(err)
	}
	return i
}

// diagnosticoAcolhido cadastra uma criança, um acolhimento e um
// diagnóstico dela (fora da transação sob teste).
func (c cenario) diagnosticoAcolhido(t *testing.T) domain.Diagnostico {
	t.Helper()
	d, err := c.limpo().AdicionarDiagnostico(context.Background(), c.identity, c.acolhimento(t).ID, "doenca", "diagnóstico teste", "")
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// visitante cadastra uma criança, um acolhimento e uma visita dela (fora
// da transação sob teste).
func (c cenario) visitante(t *testing.T) domain.Visitante {
	t.Helper()
	v, err := c.limpo().AdicionarVisitante(context.Background(), c.identity, c.acolhimento(t).ID, "Maria Visitante", "", "Tia", time.Now(), "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// termo cadastra uma criança, um acolhimento e um termo dela (fora da
// transação sob teste).
func (c cenario) termo(t *testing.T) domain.Termo {
	t.Helper()
	tm, err := c.limpo().RegistrarTermo(context.Background(), c.identity, c.acolhimento(t).ID, "passeio", "Tia Maria", "Tia", "", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return tm
}

// piaMeta cadastra uma criança, um acolhimento e uma meta do PIA dela
// (fora da transação sob teste).
func (c cenario) piaMeta(t *testing.T) domain.PiaMeta {
	t.Helper()
	m, err := c.limpo().AdicionarPiaMeta(context.Background(), c.identity, c.acolhimento(t).ID, domain.PiaMetaPlano, "descrição válida da meta", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// documento cadastra uma criança, um acolhimento e um documento dela já
// confirmado (fora da transação sob teste) — simula o navegador
// enviando o conteúdo direto ao storage antes de confirmar.
func (c cenario) documento(t *testing.T) domain.DocumentoCasaAbrigo {
	t.Helper()
	svc := c.limpo()
	res, err := svc.StartUploadDocumento(context.Background(), c.identity, c.acolhimento(t).ID, domain.DocumentoTipoAcolhido, "doc.pdf", "application/pdf", 8)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.store.Put(context.Background(), bucket, res.Upload.ObjectKey, strings.NewReader("%PDF-1.4"), 8, "application/pdf"); err != nil {
		t.Fatal(err)
	}
	d, err := svc.ConfirmUploadDocumento(context.Background(), c.identity, res.Documento.ID)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// encaminhamentoExterno cadastra um munícipe e um encaminhamento
// externo pendente dele (fora da transação sob teste).
func (c cenario) encaminhamentoExterno(t *testing.T) domain.EncaminhamentoExterno {
	t.Helper()
	e, err := c.limpo().RegistrarEncaminhamentoExterno(context.Background(), c.identity, c.cadastrar(t).ID, c.unPop, "caps_ad", "media", "motivo válido")
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// documentoCentroPop cadastra um munícipe e um documento confirmado
// dele (fora da transação sob teste) — mesmo padrão de documento().
func (c cenario) documentoCentroPop(t *testing.T) domain.DocumentoCentroPop {
	t.Helper()
	svc := c.limpo()
	res, err := svc.StartUploadDocumentoCentroPop(context.Background(), c.identity, c.cadastrar(t).ID, c.unPop, domain.DocumentoCentroPopLaudoSaude, "doc.pdf", "application/pdf", 8)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.store.Put(context.Background(), bucket, res.Upload.ObjectKey, strings.NewReader("%PDF-1.4"), 8, "application/pdf"); err != nil {
		t.Fatal(err)
	}
	d, err := svc.ConfirmUploadDocumentoCentroPop(context.Background(), c.identity, res.Documento.ID)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// globalGestao tem sempras:gestao sem escopo — decidir um pedido do
// titular exige cobertura TOTAL (§8), não uma unidade só.
func (c cenario) globalGestao() auth.Identity {
	g := auth.Identity{UserID: c.identity.UserID, Scopes: []auth.Scope{{Permissions: []string{"sempras:gestao"}}}}
	g.Permissions = auth.EffectivePermissions(g.Scopes)
	return g
}

// global tem todas as permissões do módulo sem escopo — para testar o
// handler sem se preocupar com cobertura (TestHandlersReportFailures).
func (c cenario) global() auth.Identity {
	g := auth.Identity{UserID: c.identity.UserID, Scopes: []auth.Scope{{Permissions: []string{"sempras:*"}}}}
	g.Permissions = auth.EffectivePermissions(g.Scopes)
	return g
}

// Matriz de falhas: cada chamada ao repositório falha (ou envenena a
// transação logo depois) e o caso de uso propaga o erro — nunca devolve
// sucesso com dado gravado sem evento/auditoria, nem ficha sem auditoria.
func TestEveryRepositoryFailureIsPropagated(t *testing.T) {
	c := novoCenario(t)
	ctx := context.Background()
	id := c.identity

	// Prontuário com dois registros psicológicos (o segundo usa o cache da unidade).
	ficha := c.incluir(t, domain.TipoPsicologico)
	for range 2 {
		if _, err := c.limpo().RegistrarEvolucao(ctx, id, ficha.CidadaoID, application.EvolucaoInput{
			UnidadeID: c.unidade, Tipo: domain.TipoPsicologico, Texto: "sigiloso"}); err != nil {
			t.Fatal(err)
		}
	}
	ops := map[string]func(t *testing.T, s *application.Service) error{
		"Unidades": func(_ *testing.T, s *application.Service) error { _, err := s.Unidades(ctx, id); return err },
		"Buscar documento": func(_ *testing.T, s *application.Service) error {
			_, err := s.Buscar(ctx, id, "00000000000")
			return err
		},
		"Buscar nome": func(_ *testing.T, s *application.Service) error {
			_, err := s.Buscar(ctx, id, "Pessoa")
			return err
		},
		"Cadastrar": func(_ *testing.T, s *application.Service) error {
			_, err := s.Cadastrar(ctx, id, domain.CidadaoInput{Nome: "Nova", DataNascimento: "2000-01-01", NomeMae: "M"})
			return err
		},
		"Atualizar": func(_ *testing.T, s *application.Service) error {
			_, err := s.Atualizar(ctx, id, ficha.CidadaoID, domain.CidadaoInput{Nome: "Editada", DataNascimento: "2000-01-01", NomeMae: "M"})
			return err
		},
		"Cidadao": func(_ *testing.T, s *application.Service) error {
			_, err := s.Cidadao(ctx, ficha.CidadaoID)
			return err
		},
		"Prontuario": func(_ *testing.T, s *application.Service) error {
			_, err := s.Prontuario(ctx, id, ficha.CidadaoID)
			return err
		},
		"RegistrarEvolucao": func(_ *testing.T, s *application.Service) error {
			_, err := s.RegistrarEvolucao(ctx, id, ficha.CidadaoID, application.EvolucaoInput{
				UnidadeID: c.unidade, AtendimentoID: &ficha.ID, Tipo: domain.TipoSocial, Texto: "evolução"})
			return err
		},
		"Incluir": func(t *testing.T, s *application.Service) error {
			_, err := s.Incluir(ctx, id, c.unidade, application.EntradaInput{CidadaoID: c.cadastrar(t).ID, Tipo: domain.TipoSocial, Demanda: "atendimento_tecnico"})
			return err
		},
		"Fila": func(_ *testing.T, s *application.Service) error { _, err := s.Fila(ctx, id, c.unidade); return err },
		"ChamarProximo": func(t *testing.T, s *application.Service) error {
			c.incluir(t, domain.TipoSocial)
			_, err := s.ChamarProximo(ctx, id, c.unidade, "Sala 1")
			return err
		},
		"Agir": func(t *testing.T, s *application.Service) error {
			_, err := s.Agir(ctx, id, c.incluir(t, domain.TipoSocial).ID, "chamar", "")
			return err
		},
		"Painel": func(_ *testing.T, s *application.Service) error { _, err := s.Painel(ctx, id); return err },
		"Authorize": func(_ *testing.T, s *application.Service) error {
			client := ws.ClientInfo{Scopes: id.Scopes}
			return s.Authorize(ctx, client, application.TopicFila(c.unidade))
		},
		"SolicitarAcesso": func(_ *testing.T, s *application.Service) error {
			_, err := s.SolicitarAcesso(ctx, id, ficha.CidadaoID, "titular pediu os próprios dados")
			return err
		},
		"SolicitarPedido correcao": func(t *testing.T, s *application.Service) error {
			_, err := s.SolicitarPedido(ctx, id, c.cadastrar(t).ID, domain.PedidoCorrecao, "corrigir telefone")
			return err
		},
		"SolicitarPedido anonimizacao": func(t *testing.T, s *application.Service) error {
			_, err := s.SolicitarPedido(ctx, id, c.cadastrar(t).ID, domain.PedidoAnonimizacao, "titular pediu para ser esquecido")
			return err
		},
		"ListarPedidosPendentes": func(_ *testing.T, s *application.Service) error {
			_, err := s.ListarPedidosPendentes(ctx, c.globalGestao())
			return err
		},
		"DecidirPedido": func(t *testing.T, s *application.Service) error {
			_, err := s.DecidirPedido(ctx, c.globalGestao(), c.pedirAnonimizacao(t).ID, true, "")
			return err
		},
		"AdicionarContatoRedeApoio": func(t *testing.T, s *application.Service) error {
			_, err := s.AdicionarContatoRedeApoio(ctx, id, c.cadastrar(t).ID, "Maria", "Irmã", "66999990000")
			return err
		},
		"RemoverContatoRedeApoio": func(t *testing.T, s *application.Service) error {
			return s.RemoverContatoRedeApoio(ctx, id, c.contatoRedeApoio(t))
		},
		"SolicitarBeneficio": func(t *testing.T, s *application.Service) error {
			_, err := s.SolicitarBeneficio(ctx, id, c.cadastrar(t).ID, nil, c.unidade, domain.BeneficioCestaBasica, domain.FormaAcessoDemandaEspontanea, 1, "")
			return err
		},
		"DecidirBeneficio": func(t *testing.T, s *application.Service) error {
			_, err := s.DecidirBeneficio(ctx, id, c.beneficio(t).ID, "conceder", "")
			return err
		},
		"Encaminhar": func(t *testing.T, s *application.Service) error {
			_, err := s.Encaminhar(ctx, id, c.cadastrar(t).ID, c.unidade, c.unidade2, "precisa de acompanhamento especializado")
			return err
		},
		"EncaminhamentosPendentes": func(t *testing.T, s *application.Service) error {
			c.encaminhamento(t)
			_, err := s.EncaminhamentosPendentes(ctx, c.identityUnidade2(), c.unidade2)
			return err
		},
		"DecidirEncaminhamento": func(t *testing.T, s *application.Service) error {
			_, err := s.DecidirEncaminhamento(ctx, c.identityUnidade2(), c.encaminhamento(t).ID, true, "")
			return err
		},
		"IncluirPaif": func(t *testing.T, s *application.Service) error {
			_, err := s.IncluirPaif(ctx, id, c.cadastrar(t).ID, c.unidade, domain.PaifAtendimentoParticularizado, true, false, false, 2)
			return err
		},
		"ExcluirPaif": func(t *testing.T, s *application.Service) error {
			_, err := s.ExcluirPaif(ctx, id, c.paif(t).ID, "família se mudou do território")
			return err
		},
		"RegistrarConselhoAtendimento": func(t *testing.T, s *application.Service) error {
			_, err := s.RegistrarConselhoAtendimento(ctx, id, c.cadastrar(t).ID, c.unConselho, "Mãe", "", "", "escola", "evasao_escolar", "")
			return err
		},
		"TransitarConselhoAtendimento": func(t *testing.T, s *application.Service) error {
			_, err := s.TransitarConselhoAtendimento(ctx, id, c.conselho(t).ID, "encaminhar_mp", "")
			return err
		},
		"RegistrarPaefiCaso": func(t *testing.T, s *application.Service) error {
			_, err := s.RegistrarPaefiCaso(ctx, id, c.cadastrar(t).ID, c.unCreas, []string{"violencia_fisica"}, "", "")
			return err
		},
		"MarcarPiaElaborado": func(t *testing.T, s *application.Service) error {
			_, err := s.MarcarPiaElaborado(ctx, id, c.paefi(t).ID)
			return err
		},
		"EncerrarPaefiCaso": func(t *testing.T, s *application.Service) error {
			_, err := s.EncerrarPaefiCaso(ctx, id, c.paefi(t).ID, "objetivos_alcancados")
			return err
		},
		"UnidadesParaConfigurar": func(_ *testing.T, s *application.Service) error {
			_, err := s.UnidadesParaConfigurar(ctx, id)
			return err
		},
		"ConfigurarUnidade": func(_ *testing.T, s *application.Service) error {
			_, err := s.ConfigurarUnidade(ctx, id, c.unidade, domain.UnidadeCras, true)
			return err
		},
		"RegistrarAbordagem": func(t *testing.T, s *application.Service) error {
			cid := c.cadastrar(t).ID
			_, err := s.RegistrarAbordagem(ctx, id, c.unPop, domain.AbordagemInput{CidadaoID: &cid, Local: "Praça Brasil",
				Resultado: "orientado", DataAbordagem: time.Now().Add(-time.Hour)})
			return err
		},
		"ListarAbordagens": func(_ *testing.T, s *application.Service) error {
			_, err := s.ListarAbordagens(ctx, id, c.unPop, time.Now().Format("2006-01"))
			return err
		},
		"RegistrarCasoCasaMulher": func(t *testing.T, s *application.Service) error {
			_, err := s.RegistrarCasoCasaMulher(ctx, id, c.cadastrar(t).ID, c.unMulher, casoMulherInput())
			return err
		},
		"AtualizarCasoCasaMulher": func(t *testing.T, s *application.Service) error {
			in := casoMulherInput()
			in.NivelRisco, in.Abrigada = "extremo", true
			_, err := s.AtualizarCasoCasaMulher(ctx, id, c.casoMulher(t).ID, in)
			return err
		},
		"EncerrarCasoCasaMulher": func(t *testing.T, s *application.Service) error {
			_, err := s.EncerrarCasoCasaMulher(ctx, id, c.casoMulher(t).ID, "superacao_violencia")
			return err
		},
		"RegistrarMseMedida": func(t *testing.T, s *application.Service) error {
			_, err := s.RegistrarMseMedida(ctx, id, c.cadastrar(t).ID, c.unCreas, "123/2026", "Vara da Infância",
				"furto (art. 155 CP)", domain.MseTipoLiberdadeAssistida, "")
			return err
		},
		"ConcluirMseMedida": func(t *testing.T, s *application.Service) error {
			_, err := s.ConcluirMseMedida(ctx, id, c.mse(t).ID, domain.MseStatusCumprida)
			return err
		},
		"RegistrarNotificacaoCreas": func(t *testing.T, s *application.Service) error {
			_, err := s.RegistrarNotificacaoCreas(ctx, id, c.cadastrar(t).ID, c.unCreas, "ministerio_publico",
				"requisição de informações sobre acompanhamento", nil)
			return err
		},
		"TransitarNotificacaoCreas": func(t *testing.T, s *application.Service) error {
			_, err := s.TransitarNotificacaoCreas(ctx, id, c.notificacaoCreas(t).ID, "iniciar_diligencia", "")
			return err
		},
		"RegistrarOficioCras": func(t *testing.T, s *application.Service) error {
			_, err := s.RegistrarOficioCras(ctx, id, c.cadastrar(t).ID, c.unidade, "Conselho Tutelar", "123/2026", nil, "")
			return err
		},
		"ResponderOficioCras": func(t *testing.T, s *application.Service) error {
			_, err := s.ResponderOficioCras(ctx, id, c.oficioCras(t).ID, "456/2026")
			return err
		},
		"RegistrarAtendimentoColetivo": func(_ *testing.T, s *application.Service) error {
			_, err := s.RegistrarAtendimentoColetivo(ctx, id, c.unidade, time.Now(), domain.ColetivoSCFV0a6, 10, "")
			return err
		},
		"ListarAtendimentosColetivos": func(_ *testing.T, s *application.Service) error {
			_, err := s.ListarAtendimentosColetivos(ctx, id, c.unidade, "2026-10")
			return err
		},
		"RegistrarAcolhimento": func(t *testing.T, s *application.Service) error {
			_, err := s.RegistrarAcolhimento(ctx, id, c.cadastrar(t).ID, c.unAbrigo,
				application.AcolhimentoInput{Sexo: "masculino", Motivos: []string{"negligencia_abandono"}})
			return err
		},
		"AtualizarAcolhimento": func(t *testing.T, s *application.Service) error {
			_, err := s.AtualizarAcolhimento(ctx, id, c.acolhimento(t).ID,
				application.AcolhimentoInput{Sexo: "masculino", Motivos: []string{"outro"}})
			return err
		},
		"DesligarAcolhimento": func(t *testing.T, s *application.Service) error {
			_, err := s.DesligarAcolhimento(ctx, id, c.acolhimento(t).ID, "família reestruturada")
			return err
		},
		"AdicionarIrmao": func(t *testing.T, s *application.Service) error {
			_, err := s.AdicionarIrmao(ctx, id, c.acolhimento(t).ID, "Irmão da Criança", nil, false, "", "")
			return err
		},
		"RemoverIrmao": func(t *testing.T, s *application.Service) error {
			return s.RemoverIrmao(ctx, id, c.irmao(t).ID)
		},
		"AdicionarDiagnostico": func(t *testing.T, s *application.Service) error {
			_, err := s.AdicionarDiagnostico(ctx, id, c.acolhimento(t).ID, "doenca", "diagnóstico teste", "")
			return err
		},
		"RemoverDiagnostico": func(t *testing.T, s *application.Service) error {
			return s.RemoverDiagnostico(ctx, id, c.diagnosticoAcolhido(t).ID)
		},
		"Rma": func(_ *testing.T, s *application.Service) error {
			_, err := s.Rma(ctx, id, "2026-10")
			return err
		},
		"AdicionarVisitante": func(t *testing.T, s *application.Service) error {
			_, err := s.AdicionarVisitante(ctx, id, c.acolhimento(t).ID, "Maria Visitante", "", "Tia", time.Now(), "", "", "")
			return err
		},
		"RemoverVisitante": func(t *testing.T, s *application.Service) error {
			return s.RemoverVisitante(ctx, id, c.visitante(t).ID)
		},
		"RegistrarTermo": func(t *testing.T, s *application.Service) error {
			_, err := s.RegistrarTermo(ctx, id, c.acolhimento(t).ID, "passeio", "Tia Maria", "Tia", "", time.Now())
			return err
		},
		"RemoverTermo": func(t *testing.T, s *application.Service) error {
			return s.RemoverTermo(ctx, id, c.termo(t).ID)
		},
		"SalvarPiaFicha": func(t *testing.T, s *application.Service) error {
			_, err := s.SalvarPiaFicha(ctx, id, c.acolhimento(t).ID, application.PiaFichaInput{})
			return err
		},
		"AdicionarPiaMeta": func(t *testing.T, s *application.Service) error {
			_, err := s.AdicionarPiaMeta(ctx, id, c.acolhimento(t).ID, domain.PiaMetaPlano, "descrição válida da meta", "", "", nil)
			return err
		},
		"ConcluirPiaMeta": func(t *testing.T, s *application.Service) error {
			_, err := s.ConcluirPiaMeta(ctx, id, c.piaMeta(t).ID, true)
			return err
		},
		"RemoverPiaMeta": func(t *testing.T, s *application.Service) error {
			return s.RemoverPiaMeta(ctx, id, c.piaMeta(t).ID)
		},
		"StartUploadDocumento": func(t *testing.T, s *application.Service) error {
			_, err := s.StartUploadDocumento(ctx, id, c.acolhimento(t).ID, domain.DocumentoTipoAcolhido, "doc.pdf", "application/pdf", 8)
			return err
		},
		"ConfirmUploadDocumento": func(t *testing.T, s *application.Service) error {
			res, err := c.limpo().StartUploadDocumento(ctx, c.identity, c.acolhimento(t).ID, domain.DocumentoTipoAcolhido, "doc.pdf", "application/pdf", 8)
			if err != nil {
				t.Fatal(err)
			}
			if err := c.store.Put(ctx, bucket, res.Upload.ObjectKey, strings.NewReader("%PDF-1.4"), 8, "application/pdf"); err != nil {
				t.Fatal(err)
			}
			_, err = s.ConfirmUploadDocumento(ctx, id, res.Documento.ID)
			return err
		},
		"DownloadURLDocumento": func(t *testing.T, s *application.Service) error {
			_, _, err := s.DownloadURLDocumento(ctx, id, c.documento(t).ID)
			return err
		},
		"RemoverDocumento": func(t *testing.T, s *application.Service) error {
			return s.RemoverDocumento(ctx, id, c.documento(t).ID)
		},
		"AtualizarCentroPopPerfil": func(t *testing.T, s *application.Service) error {
			_, err := s.AtualizarCentroPopPerfil(ctx, id, c.cadastrar(t).ID, c.unPop, "situacao_rua", nil, "outro", false, "", false, "", false, "")
			return err
		},
		"RegistrarEncaminhamentoExterno": func(t *testing.T, s *application.Service) error {
			_, err := s.RegistrarEncaminhamentoExterno(ctx, id, c.cadastrar(t).ID, c.unPop, "caps_ad", "media", "motivo válido")
			return err
		},
		"TransitarEncaminhamentoExterno": func(t *testing.T, s *application.Service) error {
			_, err := s.TransitarEncaminhamentoExterno(ctx, id, c.encaminhamentoExterno(t).ID, "concluido", "")
			return err
		},
		"StartUploadDocumentoCentroPop": func(t *testing.T, s *application.Service) error {
			_, err := s.StartUploadDocumentoCentroPop(ctx, id, c.cadastrar(t).ID, c.unPop, domain.DocumentoCentroPopLaudoSaude, "doc.pdf", "application/pdf", 8)
			return err
		},
		"ConfirmUploadDocumentoCentroPop": func(t *testing.T, s *application.Service) error {
			res, err := c.limpo().StartUploadDocumentoCentroPop(ctx, c.identity, c.cadastrar(t).ID, c.unPop, domain.DocumentoCentroPopLaudoSaude, "doc.pdf", "application/pdf", 8)
			if err != nil {
				t.Fatal(err)
			}
			if err := c.store.Put(ctx, bucket, res.Upload.ObjectKey, strings.NewReader("%PDF-1.4"), 8, "application/pdf"); err != nil {
				t.Fatal(err)
			}
			_, err = s.ConfirmUploadDocumentoCentroPop(ctx, id, res.Documento.ID)
			return err
		},
		"DownloadURLDocumentoCentroPop": func(t *testing.T, s *application.Service) error {
			_, _, err := s.DownloadURLDocumentoCentroPop(ctx, id, c.documentoCentroPop(t).ID)
			return err
		},
		"RemoverDocumentoCentroPop": func(t *testing.T, s *application.Service) error {
			return s.RemoverDocumentoCentroPop(ctx, id, c.documentoCentroPop(t).ID)
		},
	}
	for name, op := range ops {
		probe := &faultRepo{inner: infrastructure.NewRepository()}
		if err := op(t, c.svc(probe, publisher{})); err != nil {
			t.Fatalf("%s sem falha: %v", name, err)
		}
		for i := range probe.trace {
			for _, poison := range []bool{false, true} {
				r := &faultRepo{inner: infrastructure.NewRepository()}
				if poison {
					r.poisonAt = i + 1
				} else {
					r.failAt = i + 1
				}
				if err := op(t, c.svc(r, publisher{})); err == nil && !r.noTx {
					t.Errorf("%s: falha (veneno=%v) na chamada %d (%s) foi engolida", name, poison, i+1, probe.trace[i])
				}
			}
		}
	}
}

// O tempo real é de melhor esforço: a falha ao publicar não desfaz nada.
func TestPublishFailureDoesNotFailTheUseCase(t *testing.T) {
	c := novoCenario(t)
	s := c.svc(infrastructure.NewRepository(), publisher{err: errors.New("hub fora")})
	if _, err := s.Incluir(context.Background(), c.identity, c.unidade,
		application.EntradaInput{CidadaoID: c.cadastrar(t).ID, Tipo: domain.TipoSocial, Demanda: "atendimento_tecnico"}); err != nil {
		t.Fatal(err)
	}
}

func TestMapError(t *testing.T) {
	for err, code := range map[error]int{
		&domain.ValidationError{Msg: "x"}: http.StatusUnprocessableEntity,
		domain.ErrNotFound:                http.StatusNotFound,
		domain.ErrForbidden:               http.StatusForbidden,
		domain.ErrInvalidState:            http.StatusConflict,
		domain.ErrFilaVazia:               http.StatusConflict,
		domain.ErrJaNaFila:                http.StatusConflict,
		domain.ErrDocumentoDuplicado:      http.StatusConflict,
		domain.ErrUnidadeInativa:          http.StatusUnprocessableEntity,
		domain.ErrAtendimentoOutro:        http.StatusUnprocessableEntity,
		domain.ErrPedidoDecidido:          http.StatusConflict,
		domain.ErrBeneficioEstado:         http.StatusConflict,
		domain.ErrEncaminhamentoDecidido:  http.StatusConflict,
		domain.ErrMesmaUnidade:            http.StatusUnprocessableEntity,
		domain.ErrPaifJaAtivo:             http.StatusConflict,
		domain.ErrPaifJaExcluido:          http.StatusConflict,
		domain.ErrConselhoEstado:          http.StatusConflict,
		domain.ErrPaefiEstado:             http.StatusConflict,
		domain.ErrMseEstado:               http.StatusConflict,
		domain.ErrNotificacaoEstado:       http.StatusConflict,
		domain.ErrAcolhimentoEstado:       http.StatusConflict,
		domain.ErrServicoIndisponivel:     http.StatusNotFound,
		domain.ErrCasaMulherEstado:        http.StatusConflict,
		domain.ErrCasaMulherJaAtivo:       http.StatusConflict,
		domain.ErrOficioRespondido:        http.StatusConflict,
	} {
		ae, ok := apperrors.As(application.MapError(err))
		if !ok || ae.Status != code {
			t.Errorf("%v -> %+v", err, ae)
		}
	}
	if err := application.MapError(errBoom); !errors.Is(err, errBoom) {
		t.Error("erro desconhecido passa adiante")
	}
}

// Regras que não dependem de falha do banco.
func TestRegrasDoServico(t *testing.T) {
	c := novoCenario(t)
	ctx := context.Background()
	s := c.limpo()
	if _, err := s.Agir(ctx, c.identity, uuid.New(), "teletransportar", ""); !errors.Is(err, domain.ErrInvalidState) {
		t.Errorf("ação desconhecida: %v", err)
	}
	semPsico := auth.Identity{UserID: c.identity.UserID, Scopes: []auth.Scope{{EntidadeID: c.identity.Scopes[0].EntidadeID,
		UnidadeID: &c.unidade, Permissions: []string{"sempras:recepcao", "sempras:atendimento"}}}}
	if _, err := s.RegistrarEvolucao(ctx, semPsico, c.cadastrar(t).ID, application.EvolucaoInput{
		UnidadeID: c.unidade, Tipo: domain.TipoPsicologico, Texto: "x"}); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("psicológico sem sigilo: %v", err)
	}
	if _, err := s.RegistrarEvolucao(ctx, semPsico, uuid.New(), application.EvolucaoInput{
		UnidadeID: c.unidade, Tipo: "?", Texto: "x"}); err == nil {
		t.Error("registro inválido")
	}
	if _, err := s.Incluir(ctx, semPsico, c.unidade, application.EntradaInput{Tipo: "?"}); err == nil {
		t.Error("entrada inválida")
	}
	if _, err := s.Cadastrar(ctx, semPsico, domain.CidadaoInput{Nome: "x"}); err == nil {
		t.Error("cadastro inválido")
	}
	if _, err := s.Atualizar(ctx, semPsico, uuid.New(), domain.CidadaoInput{Nome: "x"}); err == nil {
		t.Error("edição inválida")
	}
	if _, err := s.Prontuario(ctx, auth.Identity{}, uuid.New()); !errors.Is(err, domain.ErrForbidden) {
		t.Error("ficha sem área")
	}
	if _, err := s.Painel(ctx, semPsico); !errors.Is(err, domain.ErrForbidden) {
		t.Error("painel sem gestão")
	}
	// Só a recepção na unidade: vê a fila, mas não chama.
	recepcao := c.so("sempras:recepcao")
	if _, err := s.ChamarProximo(ctx, recepcao, c.unidade, ""); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("recepção não chama: %v", err)
	}
	if _, err := s.Fila(ctx, recepcao, c.unidade); err != nil {
		t.Errorf("recepção vê a fila: %v", err)
	}
	if err := s.Inbound(ctx, ws.ClientInfo{}, ws.Frame{}); err == nil {
		t.Error("frames do navegador são recusados")
	}

	// Direitos do titular.
	cid := c.cadastrar(t).ID
	if _, err := s.SolicitarPedido(ctx, c.identity, cid, domain.PedidoAcesso, "x"); err == nil {
		t.Error("acesso tem rota própria (SolicitarAcesso), não entra em SolicitarPedido")
	}
	correcao, err := s.SolicitarPedido(ctx, c.identity, cid, domain.PedidoCorrecao, "corrigir telefone")
	if err != nil || correcao.Status != domain.PedidoStatusConcluido {
		t.Errorf("correção devia nascer concluída: %+v, %v", correcao, err)
	}
	// Cobertura escopada (não global) não decide: o cadastro do titular é
	// intersecretaria, a decisão também precisa ser (§8).
	if _, err := s.ListarPedidosPendentes(ctx, c.identity); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("listar pedidos sem cobertura total: %v", err)
	}
	anonimizacao := c.pedirAnonimizacao(t)
	if _, err := s.DecidirPedido(ctx, c.identity, anonimizacao.ID, true, ""); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("decidir sem cobertura total: %v", err)
	}
	if _, err := s.DecidirPedido(ctx, c.globalGestao(), anonimizacao.ID, false, ""); err == nil {
		t.Error("negar exige justificativa")
	}
	negado, err := s.DecidirPedido(ctx, c.globalGestao(), anonimizacao.ID, false, "sem elementos para decidir")
	if err != nil || negado.Status != domain.PedidoStatusNegado {
		t.Errorf("negar: %+v, %v", negado, err)
	}
	if _, err := s.DecidirPedido(ctx, c.globalGestao(), anonimizacao.ID, true, ""); !errors.Is(err, domain.ErrPedidoDecidido) {
		t.Errorf("decidir de novo um pedido já negado: %v", err)
	}

	// Rede de apoio.
	semNada := auth.Identity{}
	if _, err := s.AdicionarContatoRedeApoio(ctx, semNada, cid, "Maria", "", ""); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("adicionar sem permissão: %v", err)
	}
	if _, err := s.AdicionarContatoRedeApoio(ctx, c.identity, cid, "A", "", ""); err == nil {
		t.Error("nome curto demais")
	}
	contato, err := s.AdicionarContatoRedeApoio(ctx, c.identity, cid, "Maria", "Irmã", "66999990000")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RemoverContatoRedeApoio(ctx, semNada, contato.ID); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("remover sem permissão: %v", err)
	}
	if err := s.RemoverContatoRedeApoio(ctx, c.identity, uuid.New()); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("remover contato inexistente: %v", err)
	}
	if err := s.RemoverContatoRedeApoio(ctx, c.identity, contato.ID); err != nil {
		t.Errorf("remover: %v", err)
	}

	// Benefícios eventuais.
	if _, err := s.SolicitarBeneficio(ctx, c.identity, cid, nil, c.unidade, "invalido", domain.FormaAcessoDemandaEspontanea, 1, ""); err == nil {
		t.Error("tipo de benefício inválido")
	}
	if _, err := s.SolicitarBeneficio(ctx, c.identity, cid, nil, c.unidade, domain.BeneficioNatalidade, "invalida", 1, ""); err == nil {
		t.Error("forma de acesso inválida")
	}
	beneficio, err := s.SolicitarBeneficio(ctx, c.identity, cid, nil, c.unidade, domain.BeneficioNatalidade, domain.FormaAcessoDemandaEspontanea, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DecidirBeneficio(ctx, c.identity, beneficio.ID, "negar", ""); err == nil {
		t.Error("negar sem motivo")
	}
	decidido, err := s.DecidirBeneficio(ctx, c.identity, beneficio.ID, "conceder", "")
	if err != nil || decidido.Status != domain.BeneficioStatusConcedido {
		t.Errorf("conceder: %+v, %v", decidido, err)
	}
	// recepção (sem atendimento) não decide.
	if _, err := s.DecidirBeneficio(ctx, recepcao, beneficio.ID, "suspender", "motivo qualquer"); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("recepção não decide benefício: %v", err)
	}

	// Encaminhamentos.
	if _, err := s.Encaminhar(ctx, c.identity, cid, c.unidade, c.unidade, "motivo válido aqui"); !errors.Is(err, domain.ErrMesmaUnidade) {
		t.Errorf("mesma unidade: %v", err)
	}
	enc, err := s.Encaminhar(ctx, c.identity, cid, c.unidade, c.unidade2, "precisa de acompanhamento especializado")
	if err != nil {
		t.Fatal(err)
	}
	// quem está na origem não tem permissão nenhuma no destino — "não
	// encontrado", não "sem permissão" (s.unidade não revela a existência
	// de uma unidade a quem não atua nela de jeito nenhum).
	if _, err := s.DecidirEncaminhamento(ctx, c.identity, enc.ID, true, ""); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("origem não decide: %v", err)
	}
	if _, err := s.EncaminhamentosPendentes(ctx, c.identity, c.unidade2); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("sem permissão nenhuma no destino: %v", err)
	}
	destino := c.identityUnidade2()
	pendentes, err := s.EncaminhamentosPendentes(ctx, destino, c.unidade2)
	if err != nil || len(pendentes) == 0 {
		t.Errorf("fila de encaminhamentos do destino: %+v, %v", pendentes, err)
	}
	decididoEnc, err := s.DecidirEncaminhamento(ctx, destino, enc.ID, true, "vamos acompanhar")
	if err != nil || decididoEnc.Status != domain.EncaminhamentoStatusAceito {
		t.Errorf("aceitar: %+v, %v", decididoEnc, err)
	}
	if _, err := s.DecidirEncaminhamento(ctx, destino, enc.ID, false, ""); !errors.Is(err, domain.ErrEncaminhamentoDecidido) {
		t.Errorf("já decidido: %v", err)
	}

	// PAIF.
	vinculo, err := s.IncluirPaif(ctx, c.identity, cid, c.unidade, domain.PaifAtendimentoParticularizado, true, false, false, 3)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.IncluirPaif(ctx, c.identity, cid, c.unidade, domain.PaifAtendimentoParticularizado, true, false, false, 3); !errors.Is(err, domain.ErrPaifJaAtivo) {
		t.Errorf("já ativo: %v", err)
	}
	if _, err := s.ExcluirPaif(ctx, c.identity, vinculo.ID, ""); err == nil {
		t.Error("motivo obrigatório")
	}
	excluido, err := s.ExcluirPaif(ctx, c.identity, vinculo.ID, "família se mudou do território")
	if err != nil || excluido.Status != domain.PaifStatusExcluido {
		t.Errorf("excluir: %+v, %v", excluido, err)
	}
	if _, err := s.ExcluirPaif(ctx, c.identity, vinculo.ID, "de novo"); !errors.Is(err, domain.ErrPaifJaExcluido) {
		t.Errorf("já excluído: %v", err)
	}
	// depois de excluída, a família pode ser reincluída (novo vínculo).
	if _, err := s.IncluirPaif(ctx, c.identity, cid, c.unidade, domain.PaifAtendimentoParticularizado, true, false, false, 3); err != nil {
		t.Errorf("reincluir após exclusão: %v", err)
	}

	// Conselho Tutelar.
	if _, err := s.RegistrarConselhoAtendimento(ctx, c.identity, cid, c.unConselho, "Mãe", "", "", "origem_invalida", "evasao_escolar", ""); err == nil {
		t.Error("origem inválida")
	}
	caso, err := s.RegistrarConselhoAtendimento(ctx, c.identity, cid, c.unConselho, "Mãe da Criança", "", "", "escola", "evasao_escolar", "faltou 20 dias")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.TransitarConselhoAtendimento(ctx, c.identity, caso.ID, "aplicar_medida", "medida_invalida"); err == nil {
		t.Error("medida inválida")
	}
	decididoCaso, err := s.TransitarConselhoAtendimento(ctx, c.identity, caso.ID, "aplicar_medida", "matricula_escolar_obrigatoria")
	if err != nil || decididoCaso.Status != domain.ConselhoStatusMedidaAplicada {
		t.Errorf("aplicar medida: %+v, %v", decididoCaso, err)
	}
	arquivado, err := s.TransitarConselhoAtendimento(ctx, c.identity, caso.ID, "arquivar", "")
	if err != nil || arquivado.Status != domain.ConselhoStatusArquivado {
		t.Errorf("arquivar: %+v, %v", arquivado, err)
	}
	if _, err := s.TransitarConselhoAtendimento(ctx, c.identity, caso.ID, "encaminhar_mp", ""); !errors.Is(err, domain.ErrConselhoEstado) {
		t.Errorf("já arquivado: %v", err)
	}
	// recepção (sem atendimento) não registra nem decide.
	if _, err := s.RegistrarConselhoAtendimento(ctx, recepcao, cid, c.unConselho, "Mãe", "", "", "escola", "evasao_escolar", ""); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("recepção não registra: %v", err)
	}

	// PAEFI.
	if _, err := s.RegistrarPaefiCaso(ctx, c.identity, cid, c.unCreas, nil, "", ""); err == nil {
		t.Error("sem violações")
	}
	caso2, err := s.RegistrarPaefiCaso(ctx, c.identity, cid, c.unCreas, []string{"violencia_fisica", "negligencia"}, "dois filhos menores", "")
	if err != nil {
		t.Fatal(err)
	}
	comPia, err := s.MarcarPiaElaborado(ctx, c.identity, caso2.ID)
	if err != nil || !comPia.PiaElaborado {
		t.Errorf("PIA elaborado: %+v, %v", comPia, err)
	}
	if _, err := s.EncerrarPaefiCaso(ctx, c.identity, caso2.ID, "motivo_invalido"); err == nil {
		t.Error("motivo de encerramento inválido")
	}
	encerrado, err := s.EncerrarPaefiCaso(ctx, c.identity, caso2.ID, "objetivos_alcancados")
	if err != nil || encerrado.Status != domain.PaefiStatusDesligado {
		t.Errorf("encerrar: %+v, %v", encerrado, err)
	}
	if _, err := s.MarcarPiaElaborado(ctx, c.identity, caso2.ID); !errors.Is(err, domain.ErrPaefiEstado) {
		t.Errorf("já desligado: %v", err)
	}
	if _, err := s.RegistrarPaefiCaso(ctx, recepcao, cid, c.unCreas, []string{"violencia_fisica"}, "", ""); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("recepção não registra PAEFI: %v", err)
	}

	// MSE.
	medida, err := s.RegistrarMseMedida(ctx, c.identity, cid, c.unCreas, "123/2026", "Vara da Infância", "furto (art. 155 CP)", domain.MseTipoLiberdadeAssistida, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ConcluirMseMedida(ctx, c.identity, medida.ID, "resultado_invalido"); err == nil {
		t.Error("resultado inválido")
	}
	concluida, err := s.ConcluirMseMedida(ctx, c.identity, medida.ID, domain.MseStatusCumprida)
	if err != nil || concluida.Status != domain.MseStatusCumprida {
		t.Errorf("concluir: %+v, %v", concluida, err)
	}
	if _, err := s.ConcluirMseMedida(ctx, c.identity, medida.ID, domain.MseStatusExtinta); !errors.Is(err, domain.ErrMseEstado) {
		t.Errorf("já concluída: %v", err)
	}
	if _, err := s.RegistrarMseMedida(ctx, recepcao, cid, c.unCreas, "", "", "furto", domain.MseTipoLiberdadeAssistida, ""); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("recepção não registra MSE: %v", err)
	}

	// Notificações do CREAS.
	notif, err := s.RegistrarNotificacaoCreas(ctx, c.identity, cid, c.unCreas, "ministerio_publico", "requisição de informações", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.TransitarNotificacaoCreas(ctx, c.identity, notif.ID, "responder", ""); err == nil {
		t.Error("resposta obrigatória")
	}
	emDiligencia, err := s.TransitarNotificacaoCreas(ctx, c.identity, notif.ID, "iniciar_diligencia", "")
	if err != nil || emDiligencia.Status != domain.NotificacaoStatusEmDiligencia {
		t.Errorf("iniciar diligência: %+v, %v", emDiligencia, err)
	}
	respondida, err := s.TransitarNotificacaoCreas(ctx, c.identity, notif.ID, "responder", "família está em acompanhamento pelo PAEFI")
	if err != nil || respondida.Status != domain.NotificacaoStatusRespondida {
		t.Errorf("responder: %+v, %v", respondida, err)
	}
	if _, err := s.RegistrarNotificacaoCreas(ctx, recepcao, cid, c.unCreas, "ministerio_publico", "requisição", nil); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("recepção não registra notificação: %v", err)
	}

	// Casa Abrigo.
	if _, err := s.RegistrarAcolhimento(ctx, c.identity, cid, c.unAbrigo, application.AcolhimentoInput{Sexo: "outro", Motivos: []string{"outro"}}); err == nil {
		t.Error("sexo inválido")
	}
	acolhido, err := s.RegistrarAcolhimento(ctx, c.identity, cid, c.unAbrigo,
		application.AcolhimentoInput{Sexo: "feminino", Motivos: []string{"negligencia_abandono", "situacao_rua"}, ResponsavelNome: "Mãe da Criança"})
	if err != nil {
		t.Fatal(err)
	}
	// Atualizar nunca toca status/desligamento (RN-14) — mesmo mandando
	// esses campos "por fora" não haveria onde escrevê-los: AcolhimentoInput
	// não tem esses campos, então não há como contornar a regra pelo PUT geral.
	atualizado, err := s.AtualizarAcolhimento(ctx, c.identity, acolhido.ID,
		application.AcolhimentoInput{Sexo: "feminino", Motivos: []string{"outro"}, Observacoes: "nova observação"})
	if err != nil || atualizado.Status != domain.AcolhimentoStatusAcolhido || atualizado.Observacoes != "nova observação" {
		t.Errorf("atualizar: %+v, %v", atualizado, err)
	}
	if _, err := s.DesligarAcolhimento(ctx, c.identity, acolhido.ID, ""); err == nil {
		t.Error("motivo de desligamento obrigatório")
	}
	desligado, err := s.DesligarAcolhimento(ctx, c.identity, acolhido.ID, "família reestruturada, retorno ao convívio")
	if err != nil || desligado.Status != domain.AcolhimentoStatusDesligado {
		t.Errorf("desligar: %+v, %v", desligado, err)
	}
	if _, err := s.DesligarAcolhimento(ctx, c.identity, acolhido.ID, "de novo"); !errors.Is(err, domain.ErrAcolhimentoEstado) {
		t.Errorf("já desligado: %v", err)
	}
	if _, err := s.RegistrarAcolhimento(ctx, recepcao, cid, c.unAbrigo, application.AcolhimentoInput{Sexo: "masculino", Motivos: []string{"outro"}}); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("recepção não registra acolhimento: %v", err)
	}

	irmao, err := s.AdicionarIrmao(ctx, c.identity, acolhido.ID, "Irmão da Criança", nil, true, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RemoverIrmao(ctx, c.identity, irmao.ID); err != nil {
		t.Errorf("remover irmão: %v", err)
	}

	if _, err := s.AdicionarDiagnostico(ctx, c.identity, acolhido.ID, "doenca_invalida", "x", ""); err == nil {
		t.Error("categoria de diagnóstico inválida")
	}
	diag, err := s.AdicionarDiagnostico(ctx, c.identity, acolhido.ID, "transtorno", "TEA nível 1", "suporte leve")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RemoverDiagnostico(ctx, c.identity, diag.ID); err != nil {
		t.Errorf("remover diagnóstico: %v", err)
	}

	// RMA/MDS.
	if _, err := s.Rma(ctx, c.identity, "data-invalida"); err == nil {
		t.Error("competência inválida")
	}
	if _, err := s.Rma(ctx, recepcao, "2026-10"); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("recepção não vê o RMA: %v", err)
	}
	rma, err := s.Rma(ctx, c.identity, "2026-10")
	if err != nil || len(rma) == 0 {
		t.Errorf("RMA: %+v, %v", rma, err)
	}

	// Casa Abrigo: visitantes, termos e PIA.
	acolh, err := s.RegistrarAcolhimento(ctx, c.identity, cid, c.unAbrigo, application.AcolhimentoInput{Sexo: "masculino", Motivos: []string{"outro"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AdicionarVisitante(ctx, c.identity, acolh.ID, "Maria Visitante", "", "Tia", time.Now(), "25:00", "", ""); err == nil {
		t.Error("hora inválida")
	}
	visita, err := s.AdicionarVisitante(ctx, c.identity, acolh.ID, "Maria Visitante", "", "Tia", time.Now(), "14:00", "16:00", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RemoverVisitante(ctx, c.identity, visita.ID); err != nil {
		t.Errorf("remover visitante: %v", err)
	}

	if _, err := s.RegistrarTermo(ctx, c.identity, acolh.ID, "tipo_invalido", "", "", "", time.Now()); err == nil {
		t.Error("tipo de termo inválido")
	}
	termo, err := s.RegistrarTermo(ctx, c.identity, acolh.ID, "passeio", "Tia Maria", "Tia", "passeio no parque", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RemoverTermo(ctx, c.identity, termo.ID); err != nil {
		t.Errorf("remover termo: %v", err)
	}

	pia, err := s.SalvarPiaFicha(ctx, c.identity, acolh.ID, application.PiaFichaInput{Escola: "Escola Municipal", VacinacaoAtualizada: true})
	if err != nil || pia.Escola != "Escola Municipal" {
		t.Errorf("salvar PIA: %+v, %v", pia, err)
	}
	pia2, err := s.SalvarPiaFicha(ctx, c.identity, acolh.ID, application.PiaFichaInput{Escola: "Escola Estadual"})
	if err != nil || pia2.Escola != "Escola Estadual" {
		t.Errorf("atualizar PIA (upsert): %+v, %v", pia2, err)
	}

	if _, err := s.AdicionarPiaMeta(ctx, c.identity, acolh.ID, "tipo_invalido", "descrição válida", "", "", nil); err == nil {
		t.Error("tipo de meta inválido")
	}
	meta, err := s.AdicionarPiaMeta(ctx, c.identity, acolh.ID, domain.PiaMetaIntervencao, "reduzir episódios de agressividade", "atendimento semanal", "Psicóloga", nil)
	if err != nil {
		t.Fatal(err)
	}
	metaConcluida, err := s.ConcluirPiaMeta(ctx, c.identity, meta.ID, true)
	if err != nil || !metaConcluida.Concluida {
		t.Errorf("concluir meta: %+v, %v", metaConcluida, err)
	}
	if err := s.RemoverPiaMeta(ctx, c.identity, meta.ID); err != nil {
		t.Errorf("remover meta: %v", err)
	}
	if _, err := s.AdicionarVisitante(ctx, recepcao, acolh.ID, "Maria", "", "", time.Now(), "", "", ""); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("recepção não adiciona visitante: %v", err)
	}

	// Documentos (PDF) do acolhido/responsável.
	if _, err := s.StartUploadDocumento(ctx, c.identity, acolh.ID, "tipo_invalido", "doc.pdf", "application/pdf", 8); err == nil {
		t.Error("tipo de documento inválido")
	}
	if _, err := s.StartUploadDocumento(ctx, c.identity, acolh.ID, domain.DocumentoTipoAcolhido, "doc.pdf", "application/pdf", 200<<20); err == nil {
		t.Error("tamanho acima do limite")
	}
	if _, err := s.StartUploadDocumento(ctx, recepcao, acolh.ID, domain.DocumentoTipoAcolhido, "doc.pdf", "application/pdf", 8); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("recepção não envia documento: %v", err)
	}
	up, err := s.StartUploadDocumento(ctx, c.identity, acolh.ID, domain.DocumentoTipoAcolhido, "doc.pdf", "application/pdf", 8)
	if err != nil {
		t.Fatal(err)
	}
	// um arquivo que não é PDF é recusado na confirmação (tipo real conferido no storage).
	if err := c.store.Put(ctx, bucket, up.Upload.ObjectKey, strings.NewReader("não é pdf"), 9, "text/plain"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ConfirmUploadDocumento(ctx, c.identity, up.Documento.ID); err == nil {
		t.Error("tipo real diferente de PDF deveria ser recusado")
	}

	up2, err := s.StartUploadDocumento(ctx, c.identity, acolh.ID, domain.DocumentoTipoResponsavel, "doc2.pdf", "application/pdf", 8)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.store.Put(ctx, bucket, up2.Upload.ObjectKey, strings.NewReader("%PDF-1.4"), 8, "application/pdf"); err != nil {
		t.Fatal(err)
	}
	confirmado, err := s.ConfirmUploadDocumento(ctx, c.identity, up2.Documento.ID)
	if err != nil || confirmado.Status != domain.DocumentoStatusReady {
		t.Errorf("confirmar documento: %+v, %v", confirmado, err)
	}
	// confirmar de novo é idempotente (não reprocessa).
	if _, err := s.ConfirmUploadDocumento(ctx, c.identity, up2.Documento.ID); err != nil {
		t.Errorf("confirmar de novo: %v", err)
	}
	url, _, err := s.DownloadURLDocumento(ctx, c.identity, up2.Documento.ID)
	if err != nil || url == "" {
		t.Errorf("baixar documento: %q, %v", url, err)
	}
	if err := s.RemoverDocumento(ctx, c.identity, up2.Documento.ID); err != nil {
		t.Errorf("remover documento: %v", err)
	}

	// Centro POP.
	semIdentificacao, err := s.Cadastrar(ctx, c.identity, domain.CidadaoInput{Nome: "Cidadão não identificado", Anonimo: true})
	if err != nil {
		t.Fatalf("cadastro anônimo (Centro POP): %v", err)
	}

	if _, err := s.AtualizarCentroPopPerfil(ctx, c.identity, semIdentificacao.ID, c.unPop, "invalida", nil, "outro", false, "", false, "", false, ""); err == nil {
		t.Error("situação inválida")
	}
	if _, err := s.AtualizarCentroPopPerfil(ctx, recepcao, semIdentificacao.ID, c.unPop, "situacao_rua", nil, "outro", false, "", false, "", false, ""); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("recepção não atualiza perfil de rua: %v", err)
	}
	tempoRua := 3.0
	perfil, err := s.AtualizarCentroPopPerfil(ctx, c.identity, semIdentificacao.ID, c.unPop, "situacao_rua", &tempoRua,
		"uso_de_drogas", true, "hipertensão", false, "", true, "uso de crack")
	if err != nil || perfil.Situacao != "situacao_rua" || !perfil.UsoDrogas {
		t.Errorf("atualizar perfil de rua: %+v, %v", perfil, err)
	}
	// upsert: atualizar de novo continua um perfil só.
	perfil2, err := s.AtualizarCentroPopPerfil(ctx, c.identity, semIdentificacao.ID, c.unPop, "albergado", nil,
		"desemprego", false, "", false, "", false, "")
	if err != nil || perfil2.Situacao != "albergado" {
		t.Errorf("atualizar perfil de rua de novo: %+v, %v", perfil2, err)
	}

	if _, err := s.RegistrarEncaminhamentoExterno(ctx, recepcao, semIdentificacao.ID, c.unPop, "caps_ad", "alta", "motivo válido"); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("recepção não registra encaminhamento externo: %v", err)
	}
	encExterno, err := s.RegistrarEncaminhamentoExterno(ctx, c.identity, semIdentificacao.ID, c.unPop, "caps_ad", "alta", "uso abusivo de substâncias")
	if err != nil || encExterno.Status != domain.EncaminhamentoExternoStatusPendente {
		t.Errorf("registrar encaminhamento externo: %+v, %v", encExterno, err)
	}
	encTransitado, err := s.TransitarEncaminhamentoExterno(ctx, c.identity, encExterno.ID, domain.EncaminhamentoExternoStatusEmAtendimento, "aguardando vaga")
	if err != nil || encTransitado.Status != domain.EncaminhamentoExternoStatusEmAtendimento {
		t.Errorf("transitar encaminhamento externo: %+v, %v", encTransitado, err)
	}
	// diferente de Encaminhamento (entre unidades), não há trava de "já
	// decidido" — quem atende pode mudar o status de novo.
	encConcluido, err := s.TransitarEncaminhamentoExterno(ctx, c.identity, encExterno.ID, domain.EncaminhamentoExternoStatusConcluido, "atendido pelo CAPS AD")
	if err != nil || encConcluido.Status != domain.EncaminhamentoExternoStatusConcluido {
		t.Errorf("concluir encaminhamento externo: %+v, %v", encConcluido, err)
	}

	// Documentos do Centro POP — mesma regra de tipo real conferido no storage.
	if _, err := s.StartUploadDocumentoCentroPop(ctx, recepcao, semIdentificacao.ID, c.unPop, domain.DocumentoCentroPopLaudoSaude, "doc.pdf", "application/pdf", 8); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("recepção não envia documento do Centro POP: %v", err)
	}
	upCp, err := s.StartUploadDocumentoCentroPop(ctx, c.identity, semIdentificacao.ID, c.unPop, domain.DocumentoCentroPopLaudoSaude, "laudo.pdf", "application/pdf", 8)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.store.Put(ctx, bucket, upCp.Upload.ObjectKey, strings.NewReader("não é pdf"), 9, "text/plain"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ConfirmUploadDocumentoCentroPop(ctx, c.identity, upCp.Documento.ID); err == nil {
		t.Error("tipo real diferente de PDF deveria ser recusado")
	}
	upCp2, err := s.StartUploadDocumentoCentroPop(ctx, c.identity, semIdentificacao.ID, c.unPop, domain.DocumentoCentroPopIdentificacao, "doc.pdf", "application/pdf", 8)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.store.Put(ctx, bucket, upCp2.Upload.ObjectKey, strings.NewReader("%PDF-1.4"), 8, "application/pdf"); err != nil {
		t.Fatal(err)
	}
	confirmadoCp, err := s.ConfirmUploadDocumentoCentroPop(ctx, c.identity, upCp2.Documento.ID)
	if err != nil || confirmadoCp.Status != domain.DocumentoStatusReady {
		t.Errorf("confirmar documento do Centro POP: %+v, %v", confirmadoCp, err)
	}
	urlCp, _, err := s.DownloadURLDocumentoCentroPop(ctx, c.identity, upCp2.Documento.ID)
	if err != nil || urlCp == "" {
		t.Errorf("baixar documento do Centro POP: %q, %v", urlCp, err)
	}
	if err := s.RemoverDocumentoCentroPop(ctx, c.identity, upCp2.Documento.ID); err != nil {
		t.Errorf("remover documento do Centro POP: %v", err)
	}
}

// Os handlers devolvem 4xx em entrada ruim e 500 (sem vazar o erro) quando
// o serviço falha.
func TestHandlersReportFailures(t *testing.T) {
	c := novoCenario(t)
	down := &faultRepo{inner: infrastructure.NewRepository(), failAt: 1}
	h := transport.NewHandlers(c.svc(down, publisher{}), testLogger())
	r := chi.NewRouter()
	// Identidade global: além das rotas por unidade, cobre /sempras/pedidos
	// (gestao, cobertura total) sem precisar de outro cenário.
	identidade := c.global()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.WithIdentity(req.Context(), identidade)))
		})
	})
	h.RegisterRoutes(r)
	u, x := c.unidade.String(), uuid.NewString()
	for _, tc := range []struct {
		method, path, body string
		want               int
	}{
		{http.MethodGet, "/sempras/unidades", "", 500},
		{http.MethodGet, "/sempras/painel", "", 500},
		{http.MethodGet, "/sempras/cidadaos?q=Pessoa", "", 500},
		{http.MethodPost, "/sempras/cidadaos", `{"nome":"Nova","data_nascimento":"2000-01-01","nome_mae":"M"}`, 500},
		{http.MethodPost, "/sempras/cidadaos", `{`, 400},
		{http.MethodGet, "/sempras/cidadaos/" + x, "", 500},
		{http.MethodGet, "/sempras/cidadaos/zz", "", 400},
		{http.MethodPut, "/sempras/cidadaos/" + x, `{"nome":"Nova","data_nascimento":"2000-01-01","nome_mae":"M"}`, 500},
		{http.MethodPut, "/sempras/cidadaos/zz", `{}`, 400},
		{http.MethodPut, "/sempras/cidadaos/" + x, `{`, 400},
		{http.MethodGet, "/sempras/cidadaos/" + x + "/prontuario", "", 500},
		{http.MethodGet, "/sempras/cidadaos/zz/prontuario", "", 400},
		{http.MethodPost, "/sempras/cidadaos/" + x + "/registros", `{"unidade_id":"` + u + `","tipo":"social","texto":"t"}`, 500},
		{http.MethodPost, "/sempras/cidadaos/zz/registros", `{}`, 400},
		{http.MethodPost, "/sempras/cidadaos/" + x + "/registros", `{`, 400},
		{http.MethodGet, "/sempras/unidades/" + u + "/fila", "", 500},
		{http.MethodGet, "/sempras/unidades/zz/fila", "", 400},
		{http.MethodPost, "/sempras/unidades/" + u + "/fila", `{"cidadao_id":"` + x + `","tipo":"social","demanda":"atendimento_tecnico"}`, 500},
		{http.MethodPost, "/sempras/unidades/zz/fila", `{}`, 400},
		{http.MethodPost, "/sempras/unidades/" + u + "/fila", `{`, 400},
		{http.MethodPost, "/sempras/unidades/" + u + "/fila/proximo", "", 500},
		{http.MethodPost, "/sempras/unidades/zz/fila/proximo", "", 400},
		{http.MethodPost, "/sempras/unidades/" + u + "/fila/proximo", `{`, 400},
		{http.MethodPost, "/sempras/atendimentos/" + x + "/iniciar", "", 500},
		{http.MethodGet, "/sempras/cidadaos/" + x + "/pedidos/acesso?justificativa=titular+pediu+os+dados", "", 500},
		{http.MethodGet, "/sempras/cidadaos/zz/pedidos/acesso", "", 400},
		{http.MethodPost, "/sempras/cidadaos/" + x + "/pedidos", `{"tipo":"correcao","justificativa":"corrigir telefone"}`, 500},
		{http.MethodPost, "/sempras/cidadaos/zz/pedidos", `{}`, 400},
		{http.MethodPost, "/sempras/cidadaos/" + x + "/pedidos", `{`, 400},
		{http.MethodGet, "/sempras/pedidos", "", 500},
		{http.MethodPost, "/sempras/pedidos/" + x + "/decidir", `{"aprovar":true}`, 500},
		{http.MethodPost, "/sempras/pedidos/zz/decidir", `{}`, 400},
		{http.MethodPost, "/sempras/pedidos/" + x + "/decidir", `{`, 400},
		{http.MethodPost, "/sempras/cidadaos/" + x + "/rede-apoio", `{"nome":"Maria"}`, 500},
		{http.MethodPost, "/sempras/cidadaos/zz/rede-apoio", `{}`, 400},
		{http.MethodPost, "/sempras/cidadaos/" + x + "/rede-apoio", `{`, 400},
		{http.MethodDelete, "/sempras/rede-apoio/" + x, "", 500},
		{http.MethodDelete, "/sempras/rede-apoio/zz", "", 400},
		{http.MethodPost, "/sempras/cidadaos/" + x + "/beneficios", `{"unidade_id":"` + u + `","tipo":"cesta_basica","forma_acesso":"demanda_espontanea","quantidade":1}`, 500},
		{http.MethodPost, "/sempras/cidadaos/zz/beneficios", `{}`, 400},
		{http.MethodPost, "/sempras/cidadaos/" + x + "/beneficios", `{`, 400},
		{http.MethodPost, "/sempras/beneficios/" + x + "/decidir", `{"acao":"conceder"}`, 500},
		{http.MethodPost, "/sempras/beneficios/zz/decidir", `{}`, 400},
		{http.MethodPost, "/sempras/cidadaos/" + x + "/encaminhamentos", `{"unidade_origem_id":"` + u + `","unidade_destino_id":"` + x + `","motivo":"precisa de acompanhamento"}`, 500},
		{http.MethodPost, "/sempras/cidadaos/zz/encaminhamentos", `{}`, 400},
		{http.MethodPost, "/sempras/cidadaos/" + x + "/encaminhamentos", `{`, 400},
		{http.MethodGet, "/sempras/unidades/" + u + "/encaminhamentos", "", 500},
		{http.MethodGet, "/sempras/unidades/zz/encaminhamentos", "", 400},
		{http.MethodPost, "/sempras/encaminhamentos/" + x + "/decidir", `{"aceitar":true}`, 500},
		{http.MethodPost, "/sempras/encaminhamentos/zz/decidir", `{`, 400},
		{http.MethodPost, "/sempras/cidadaos/" + x + "/paif", `{"unidade_id":"` + u + `","tipo_atendimento":"particularizado"}`, 500},
		{http.MethodPost, "/sempras/cidadaos/zz/paif", `{}`, 400},
		{http.MethodPost, "/sempras/cidadaos/" + x + "/paif", `{`, 400},
		{http.MethodPost, "/sempras/paif/" + x + "/excluir", `{"motivo":"família se mudou"}`, 500},
		{http.MethodPost, "/sempras/paif/zz/excluir", `{}`, 400},
		{http.MethodPost, "/sempras/cidadaos/" + x + "/conselho-tutelar", `{"unidade_id":"` + u + `","origem_demanda":"escola","motivo_violacao":"evasao_escolar"}`, 500},
		{http.MethodPost, "/sempras/cidadaos/zz/conselho-tutelar", `{}`, 400},
		{http.MethodPost, "/sempras/cidadaos/" + x + "/conselho-tutelar", `{`, 400},
		{http.MethodPost, "/sempras/conselho-tutelar/" + x + "/transitar", `{"acao":"encaminhar_mp"}`, 500},
		{http.MethodPost, "/sempras/conselho-tutelar/zz/transitar", `{}`, 400},
		{http.MethodPost, "/sempras/cidadaos/" + x + "/paefi", `{"unidade_id":"` + u + `","violacoes":["violencia_fisica"]}`, 500},
		{http.MethodPost, "/sempras/cidadaos/zz/paefi", `{}`, 400},
		{http.MethodPost, "/sempras/cidadaos/" + x + "/paefi", `{`, 400},
		{http.MethodPost, "/sempras/paefi/" + x + "/pia", "", 500},
		{http.MethodPost, "/sempras/paefi/zz/pia", "", 400},
		{http.MethodPost, "/sempras/paefi/" + x + "/encerrar", `{"motivo":"objetivos_alcancados"}`, 500},
		{http.MethodPost, "/sempras/paefi/zz/encerrar", `{}`, 400},
		{http.MethodPost, "/sempras/paefi/" + x + "/encerrar", `{`, 400},
		{http.MethodGet, "/sempras/unidades/configuracao", "", 500},
		{http.MethodPut, "/sempras/unidades/" + u + "/configuracao", `{"tipo":"cras"}`, 500},
		{http.MethodPut, "/sempras/unidades/zz/configuracao", `{}`, 400},
		{http.MethodPut, "/sempras/unidades/" + u + "/configuracao", `{`, 400},
		{http.MethodPost, "/sempras/unidades/" + u + "/abordagens", `{"local":"Praça Brasil","resultado":"orientado","data_abordagem":"2026-01-01T00:00:00Z","descricao_pessoa":"homem"}`, 500},
		{http.MethodPost, "/sempras/unidades/zz/abordagens", `{}`, 400},
		{http.MethodPost, "/sempras/unidades/" + u + "/abordagens", `{`, 400},
		{http.MethodGet, "/sempras/unidades/" + u + "/abordagens?mes=2026-10", "", 500},
		{http.MethodGet, "/sempras/unidades/zz/abordagens", "", 400},
		{http.MethodPost, "/sempras/cidadaos/" + x + "/casa-mulher", casoMulherBody(u), 500},
		{http.MethodPost, "/sempras/cidadaos/zz/casa-mulher", `{}`, 400},
		{http.MethodPost, "/sempras/cidadaos/" + x + "/casa-mulher", `{`, 400},
		{http.MethodPut, "/sempras/casa-mulher/" + x, casoMulherBody(""), 500},
		{http.MethodPut, "/sempras/casa-mulher/zz", `{}`, 400},
		{http.MethodPut, "/sempras/casa-mulher/" + x, `{`, 400},
		{http.MethodPost, "/sempras/casa-mulher/" + x + "/encerrar", `{"motivo":"desistencia"}`, 500},
		{http.MethodPost, "/sempras/casa-mulher/zz/encerrar", `{}`, 400},
		{http.MethodPost, "/sempras/casa-mulher/" + x + "/encerrar", `{`, 400},
		{http.MethodPut, "/sempras/casa-abrigo/" + x + "/pia", `{`, 400},
		{http.MethodPost, "/sempras/casa-abrigo-pia-metas/" + x + "/concluir", `{`, 400},
		{http.MethodPost, "/sempras/encaminhamentos/" + x + "/decidir", `{`, 400},
		{http.MethodPost, "/sempras/cidadaos/" + x + "/mse", `{"unidade_id":"` + u + `","ato_infracional":"furto (art. 155 CP)","tipo_medida":"liberdade_assistida"}`, 500},
		{http.MethodPost, "/sempras/cidadaos/zz/mse", `{}`, 400},
		{http.MethodPost, "/sempras/cidadaos/" + x + "/mse", `{`, 400},
		{http.MethodPost, "/sempras/mse/" + x + "/concluir", `{"resultado":"cumprida"}`, 500},
		{http.MethodPost, "/sempras/mse/zz/concluir", `{}`, 400},
		{http.MethodPost, "/sempras/mse/" + x + "/concluir", `{`, 400},
		{http.MethodPost, "/sempras/cidadaos/" + x + "/creas-notificacoes", `{"unidade_id":"` + u + `","origem_entidade":"ministerio_publico","assunto":"requisição de informações"}`, 500},
		{http.MethodPost, "/sempras/cidadaos/zz/creas-notificacoes", `{}`, 400},
		{http.MethodPost, "/sempras/cidadaos/" + x + "/creas-notificacoes", `{`, 400},
		{http.MethodPost, "/sempras/creas-notificacoes/" + x + "/transitar", `{"acao":"iniciar_diligencia"}`, 500},
		{http.MethodPost, "/sempras/creas-notificacoes/zz/transitar", `{}`, 400},
		{http.MethodPost, "/sempras/cidadaos/" + x + "/oficios-cras", `{"unidade_id":"` + u + `","orgao_emissor":"Conselho Tutelar","numero_oficio":"123/2026"}`, 500},
		{http.MethodPost, "/sempras/cidadaos/zz/oficios-cras", `{}`, 400},
		{http.MethodPost, "/sempras/cidadaos/" + x + "/oficios-cras", `{`, 400},
		{http.MethodPost, "/sempras/oficios-cras/" + x + "/responder", `{"numero_resposta":"456/2026"}`, 500},
		{http.MethodPost, "/sempras/oficios-cras/zz/responder", `{}`, 400},
		{http.MethodPost, "/sempras/oficios-cras/" + x + "/responder", `{`, 400},
		{http.MethodPost, "/sempras/cidadaos/" + x + "/casa-abrigo", `{"unidade_id":"` + u + `","sexo":"masculino","motivos":["negligencia_abandono"]}`, 500},
		{http.MethodPost, "/sempras/cidadaos/zz/casa-abrigo", `{}`, 400},
		{http.MethodPost, "/sempras/cidadaos/" + x + "/casa-abrigo", `{`, 400},
		{http.MethodPut, "/sempras/casa-abrigo/" + x, `{"unidade_id":"` + u + `","sexo":"masculino","motivos":["outro"]}`, 500},
		{http.MethodPut, "/sempras/casa-abrigo/zz", `{}`, 400},
		{http.MethodPut, "/sempras/casa-abrigo/" + x, `{`, 400},
		{http.MethodPost, "/sempras/casa-abrigo/" + x + "/desligar", `{"motivo":"família reestruturada"}`, 500},
		{http.MethodPost, "/sempras/casa-abrigo/zz/desligar", `{}`, 400},
		{http.MethodPost, "/sempras/casa-abrigo/" + x + "/desligar", `{`, 400},
		{http.MethodPost, "/sempras/casa-abrigo/" + x + "/irmaos", `{"nome":"Irmão da Criança"}`, 500},
		{http.MethodPost, "/sempras/casa-abrigo/zz/irmaos", `{}`, 400},
		{http.MethodPost, "/sempras/casa-abrigo/" + x + "/irmaos", `{`, 400},
		{http.MethodDelete, "/sempras/casa-abrigo-irmaos/" + x, "", 500},
		{http.MethodDelete, "/sempras/casa-abrigo-irmaos/zz", "", 400},
		{http.MethodPost, "/sempras/casa-abrigo/" + x + "/diagnosticos", `{"categoria":"doenca","diagnostico":"diagnóstico teste"}`, 500},
		{http.MethodPost, "/sempras/casa-abrigo/zz/diagnosticos", `{}`, 400},
		{http.MethodPost, "/sempras/casa-abrigo/" + x + "/diagnosticos", `{`, 400},
		{http.MethodDelete, "/sempras/casa-abrigo-diagnosticos/" + x, "", 500},
		{http.MethodDelete, "/sempras/casa-abrigo-diagnosticos/zz", "", 400},
		{http.MethodGet, "/sempras/relatorios/rma?competencia=2026-10", "", 500},
		{http.MethodPost, "/sempras/unidades/" + u + "/coletivos", `{"data_evento":"2026-10-10T00:00:00Z","categoria":"scfv_0_6","participantes":5}`, 500},
		{http.MethodPost, "/sempras/unidades/zz/coletivos", `{}`, 400},
		{http.MethodPost, "/sempras/unidades/" + u + "/coletivos", `{`, 400},
		{http.MethodGet, "/sempras/unidades/" + u + "/coletivos?mes=2026-10", "", 500},
		{http.MethodGet, "/sempras/unidades/zz/coletivos", "", 400},
		{http.MethodPost, "/sempras/casa-abrigo/" + x + "/visitantes", `{"nome_visitante":"Maria","data_visita":"2026-10-10T00:00:00Z"}`, 500},
		{http.MethodPost, "/sempras/casa-abrigo/zz/visitantes", `{}`, 400},
		{http.MethodPost, "/sempras/casa-abrigo/" + x + "/visitantes", `{`, 400},
		{http.MethodDelete, "/sempras/casa-abrigo-visitantes/" + x, "", 500},
		{http.MethodDelete, "/sempras/casa-abrigo-visitantes/zz", "", 400},
		{http.MethodPost, "/sempras/casa-abrigo/" + x + "/termos", `{"tipo_termo":"passeio","data_termo":"2026-10-10T00:00:00Z"}`, 500},
		{http.MethodPost, "/sempras/casa-abrigo/zz/termos", `{}`, 400},
		{http.MethodPost, "/sempras/casa-abrigo/" + x + "/termos", `{`, 400},
		{http.MethodDelete, "/sempras/casa-abrigo-termos/" + x, "", 500},
		{http.MethodDelete, "/sempras/casa-abrigo-termos/zz", "", 400},
		{http.MethodPut, "/sempras/casa-abrigo/" + x + "/pia", `{"escola":"Escola Municipal"}`, 500},
		{http.MethodPut, "/sempras/casa-abrigo/zz/pia", `{`, 400},
		{http.MethodPost, "/sempras/casa-abrigo/" + x + "/pia-metas", `{"tipo":"plano","descricao":"descrição válida da meta"}`, 500},
		{http.MethodPost, "/sempras/casa-abrigo/zz/pia-metas", `{}`, 400},
		{http.MethodPost, "/sempras/casa-abrigo/" + x + "/pia-metas", `{`, 400},
		{http.MethodPost, "/sempras/casa-abrigo-pia-metas/" + x + "/concluir", `{"concluida":true}`, 500},
		{http.MethodPost, "/sempras/casa-abrigo-pia-metas/zz/concluir", `{`, 400},
		{http.MethodDelete, "/sempras/casa-abrigo-pia-metas/" + x, "", 500},
		{http.MethodDelete, "/sempras/casa-abrigo-pia-metas/zz", "", 400},
		{http.MethodPost, "/sempras/casa-abrigo/" + x + "/documentos/uploads", `{"tipo":"acolhido","filename":"doc.pdf","content_type":"application/pdf","size":8}`, 500},
		{http.MethodPost, "/sempras/casa-abrigo/zz/documentos/uploads", `{}`, 400},
		{http.MethodPost, "/sempras/casa-abrigo/" + x + "/documentos/uploads", `{`, 400},
		{http.MethodPost, "/sempras/casa-abrigo-documentos/" + x + "/confirm", "", 500},
		{http.MethodPost, "/sempras/casa-abrigo-documentos/zz/confirm", "", 400},
		{http.MethodGet, "/sempras/casa-abrigo-documentos/" + x + "/download", "", 500},
		{http.MethodGet, "/sempras/casa-abrigo-documentos/zz/download", "", 400},
		{http.MethodDelete, "/sempras/casa-abrigo-documentos/" + x, "", 500},
		{http.MethodDelete, "/sempras/casa-abrigo-documentos/zz", "", 400},
		{http.MethodPut, "/sempras/cidadaos/" + x + "/centro-pop-perfil", `{"unidade_id":"` + u + `","situacao":"situacao_rua","motivo_vulnerabilidade":"outro"}`, 500},
		{http.MethodPut, "/sempras/cidadaos/zz/centro-pop-perfil", `{}`, 400},
		{http.MethodPut, "/sempras/cidadaos/" + x + "/centro-pop-perfil", `{`, 400},
		{http.MethodPost, "/sempras/cidadaos/" + x + "/encaminhamentos-externos", `{"unidade_id":"` + u + `","destino":"caps_ad","prioridade":"media","motivo":"motivo válido"}`, 500},
		{http.MethodPost, "/sempras/cidadaos/zz/encaminhamentos-externos", `{}`, 400},
		{http.MethodPost, "/sempras/cidadaos/" + x + "/encaminhamentos-externos", `{`, 400},
		{http.MethodPost, "/sempras/encaminhamentos-externos/" + x + "/transitar", `{"status":"concluido"}`, 500},
		{http.MethodPost, "/sempras/encaminhamentos-externos/zz/transitar", `{}`, 400},
		{http.MethodPost, "/sempras/cidadaos/" + x + "/centro-pop-documentos/uploads", `{"unidade_id":"` + u + `","tipo":"laudo_saude","filename":"doc.pdf","content_type":"application/pdf","size":8}`, 500},
		{http.MethodPost, "/sempras/cidadaos/zz/centro-pop-documentos/uploads", `{}`, 400},
		{http.MethodPost, "/sempras/cidadaos/" + x + "/centro-pop-documentos/uploads", `{`, 400},
		{http.MethodPost, "/sempras/centro-pop-documentos/" + x + "/confirm", "", 500},
		{http.MethodPost, "/sempras/centro-pop-documentos/zz/confirm", "", 400},
		{http.MethodGet, "/sempras/centro-pop-documentos/" + x + "/download", "", 500},
		{http.MethodGet, "/sempras/centro-pop-documentos/zz/download", "", 400},
		{http.MethodDelete, "/sempras/centro-pop-documentos/" + x, "", 500},
		{http.MethodDelete, "/sempras/centro-pop-documentos/zz", "", 400},
	} {
		down.calls = 0
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(rec, req)
		if rec.Code != tc.want || strings.Contains(rec.Body.String(), errBoom.Error()) {
			t.Errorf("%s %s: esperado %d, veio %d %s", tc.method, tc.path, tc.want, rec.Code, rec.Body.String())
		}
	}
}

// TestServicoPorTipoDeUnidade: cada unidade só oferta os serviços do seu
// tipo (ADR 031 §2) — o CRAS não abre caso do PAEFI, o CREAS não acolhe,
// e uma unidade sem tipo não atende.
func TestServicoPorTipoDeUnidade(t *testing.T) {
	c := novoCenario(t)
	s := c.limpo()
	ctx := context.Background()
	cid := c.cadastrar(t).ID
	indisponivel := func(nome string, err error) {
		t.Helper()
		if !errors.Is(err, domain.ErrServicoIndisponivel) {
			t.Errorf("%s: esperava serviço indisponível, veio %v", nome, err)
		}
	}

	_, err := s.RegistrarPaefiCaso(ctx, c.identity, cid, c.unidade, []string{"violencia_fisica"}, "", "")
	indisponivel("PAEFI no CRAS", err)
	_, err = s.RegistrarMseMedida(ctx, c.identity, cid, c.unPop, "1/2026", "Vara", "furto", domain.MseTipoLiberdadeAssistida, "")
	indisponivel("MSE no Centro POP", err)
	_, err = s.RegistrarNotificacaoCreas(ctx, c.identity, cid, c.unidade, "ministerio_publico", "requisição", nil)
	indisponivel("notificação no CRAS", err)
	_, err = s.IncluirPaif(ctx, c.identity, cid, c.unCreas, domain.PaifAtendimentoParticularizado, false, false, false, 0)
	indisponivel("PAIF no CREAS", err)
	_, err = s.RegistrarOficioCras(ctx, c.identity, cid, c.unConselho, "MPE", "1/2026", nil, "assunto")
	indisponivel("ofício no Conselho", err)
	_, err = s.RegistrarConselhoAtendimento(ctx, c.identity, cid, c.unidade, "Mãe", "", "", "escola", "evasao_escolar", "")
	indisponivel("medida do Conselho no CRAS", err)
	_, err = s.RegistrarAcolhimento(ctx, c.identity, cid, c.unCreas,
		application.AcolhimentoInput{Sexo: "masculino", Motivos: []string{"negligencia_abandono"}})
	indisponivel("acolhimento no CREAS", err)
	_, err = s.AtualizarCentroPopPerfil(ctx, c.identity, cid, c.unidade, domain.SituacoesRua[0], nil, "outro", false, "", false, "", false, "")
	indisponivel("perfil de rua no CRAS", err)
	_, err = s.SolicitarBeneficio(ctx, c.identity, cid, nil, c.unCreas, domain.BeneficioCestaBasica, domain.FormaAcessoDemandaEspontanea, 1, "")
	indisponivel("benefício no CREAS", err)
	_, err = s.RegistrarAtendimentoColetivo(ctx, c.identity, c.unCreas, time.Now(), domain.CategoriasColetivo[0], 5, "grupo")
	indisponivel("coletivo no CREAS", err)
	_, err = s.Incluir(ctx, c.identity, c.unAbrigo, application.EntradaInput{CidadaoID: cid, Tipo: domain.TipoSocial})
	indisponivel("fila na Casa Abrigo", err)

	// Os serviços comuns valem em todos os tipos.
	if _, err := s.RegistrarEvolucao(ctx, c.identity, cid, application.EvolucaoInput{
		UnidadeID: c.unAbrigo, Tipo: domain.TipoSocial, Texto: "evolução na Casa Abrigo"}); err != nil {
		t.Errorf("evolução na Casa Abrigo: %v", err)
	}

	// Unidade sem tipo (Sede, setor administrativo): não atende nada nem
	// aparece no seletor.
	var ent uuid.UUID
	if err := c.pool.QueryRow(ctx, `SELECT entidade_id FROM unidades WHERE id = $1`, c.unidade).Scan(&ent); err != nil {
		t.Fatal(err)
	}
	sede := novaUnidade(t, c.pool, ent, "")
	gestor := auth.Identity{UserID: c.identity.UserID, Scopes: []auth.Scope{{EntidadeID: &ent, Permissions: []string{"sempras:*"}}}}
	gestor.Permissions = auth.EffectivePermissions(gestor.Scopes)
	_, err = s.Incluir(ctx, gestor, sede, application.EntradaInput{CidadaoID: cid, Tipo: domain.TipoSocial})
	indisponivel("fila na unidade sem tipo", err)
	_, err = s.Encaminhar(ctx, c.identity, cid, c.unidade, sede, "encaminhar para a sede")
	indisponivel("encaminhamento para unidade sem tipo", err)
	us, err := s.Unidades(ctx, gestor)
	if err != nil {
		t.Fatal(err)
	}
	if len(us) == 0 {
		t.Fatal("o gestor da entidade deveria ver as unidades com tipo")
	}
	for _, u := range us {
		if u.ID == sede {
			t.Error("unidade sem tipo não aparece no seletor")
		}
		if len(u.Servicos) == 0 {
			t.Errorf("unidade %s sem serviços", u.Tipo)
		}
	}
}

// TestProntuarioPorTipoDeUnidade: o perfil de rua (saúde, drogas) é do
// munícipe, mas só sai para quem atua num Centro POP (ADR 031 §4).
func TestProntuarioPorTipoDeUnidade(t *testing.T) {
	c := novoCenario(t)
	s := c.limpo()
	ctx := context.Background()
	cid := c.cadastrar(t).ID
	if _, err := s.AtualizarCentroPopPerfil(ctx, c.identity, cid, c.unPop, domain.SituacoesRua[0], nil, "outro",
		false, "", false, "", true, "álcool"); err != nil {
		t.Fatal(err)
	}
	so := func(un uuid.UUID) auth.Identity {
		i := auth.Identity{UserID: c.identity.UserID, Scopes: []auth.Scope{{UnidadeID: &un, Permissions: []string{"sempras:atendimento"}}}}
		i.Permissions = auth.EffectivePermissions(i.Scopes)
		return i
	}
	p, err := s.Prontuario(ctx, so(c.unidade), cid)
	if err != nil {
		t.Fatal(err)
	}
	if p.CentroPopPerfil != nil || len(p.CentroPopDocumentos) != 0 {
		t.Error("o CRAS não lê o perfil de rua")
	}
	if p, err = s.Prontuario(ctx, so(c.unPop), cid); err != nil {
		t.Fatal(err)
	}
	if p.CentroPopPerfil == nil {
		t.Error("o Centro POP lê o perfil de rua")
	}
}

// casoMulherBody é o JSON de uma avaliação válida (unidade só na abertura).
func casoMulherBody(unidade string) string {
	campoUnidade := ""
	if unidade != "" {
		campoUnidade = `"unidade_id":"` + unidade + `",`
	}
	return `{` + campoUnidade + `"violencias":["fisica"],"relacao_agressor":"conjuge_companheiro",` +
		`"nivel_risco":"medio","medida_protetiva_status":"nao_solicitada","filhos":1}`
}

// TestCasaMulher: o caso só existe na Casa da Mulher, um ativo por mulher,
// reavaliação e encerramento só com o caso ativo, recepção não registra.
func TestCasaMulher(t *testing.T) {
	c := novoCenario(t)
	s := c.limpo()
	ctx := context.Background()
	cid := c.cadastrar(t).ID

	if _, err := s.RegistrarCasoCasaMulher(ctx, c.identity, cid, c.unidade, casoMulherInput()); !errors.Is(err, domain.ErrServicoIndisponivel) {
		t.Errorf("o CRAS não abre caso da Casa da Mulher: %v", err)
	}
	if _, err := s.RegistrarCasoCasaMulher(ctx, c.so("sempras:recepcao"), cid, c.unMulher, casoMulherInput()); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("recepção não registra o caso: %v", err)
	}
	if _, err := s.RegistrarCasoCasaMulher(ctx, c.identity, cid, c.unMulher, domain.CasoCasaMulherInput{}); err == nil {
		t.Error("caso sem formas de violência deveria falhar")
	}
	if _, err := s.RegistrarCasoCasaMulher(ctx, c.identity, uuid.New(), c.unMulher, casoMulherInput()); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("munícipe inexistente: %v", err)
	}
	caso, err := s.RegistrarCasoCasaMulher(ctx, c.identity, cid, c.unMulher, casoMulherInput())
	if err != nil || caso.Status != domain.CasaMulherStatusAtivo || caso.Abrigada {
		t.Fatalf("abrir: %+v, %v", caso, err)
	}
	if _, err := s.RegistrarCasoCasaMulher(ctx, c.identity, cid, c.unMulher, casoMulherInput()); !errors.Is(err, domain.ErrCasaMulherJaAtivo) {
		t.Errorf("segundo caso ativo: %v", err)
	}

	in := casoMulherInput()
	in.NivelRisco, in.Abrigada, in.MedidaProtetivaStatus, in.MedidaProtetivaProcesso = "extremo", true, "deferida", "0001234-56.2026"
	if _, err := s.AtualizarCasoCasaMulher(ctx, c.identity, caso.ID, domain.CasoCasaMulherInput{}); err == nil {
		t.Error("reavaliação inválida deveria falhar")
	}
	at, err := s.AtualizarCasoCasaMulher(ctx, c.identity, caso.ID, in)
	if err != nil || !at.Abrigada || at.DataAbrigamento == nil || at.NivelRisco != "extremo" || at.AtualizadoEm == nil {
		t.Fatalf("reavaliar: %+v, %v", at, err)
	}
	if _, err := s.AtualizarCasoCasaMulher(ctx, c.so("sempras:recepcao"), caso.ID, in); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("recepção não reavalia: %v", err)
	}
	if _, err := s.AtualizarCasoCasaMulher(ctx, c.identity, uuid.New(), in); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("caso inexistente: %v", err)
	}
	if _, err := s.EncerrarCasoCasaMulher(ctx, c.identity, caso.ID, "?"); err == nil {
		t.Error("motivo inválido deveria falhar")
	}
	fim, err := s.EncerrarCasoCasaMulher(ctx, c.identity, caso.ID, "mudanca_municipio")
	if err != nil || fim.Status != domain.CasaMulherStatusEncerrado || fim.Abrigada || fim.DataEncerramento == nil {
		t.Fatalf("encerrar: %+v, %v", fim, err)
	}
	if _, err := s.EncerrarCasoCasaMulher(ctx, c.identity, caso.ID, "outro"); !errors.Is(err, domain.ErrCasaMulherEstado) {
		t.Errorf("já encerrado: %v", err)
	}
	if _, err := s.AtualizarCasoCasaMulher(ctx, c.identity, caso.ID, in); !errors.Is(err, domain.ErrCasaMulherEstado) {
		t.Errorf("reavaliar encerrado: %v", err)
	}
	if _, err := s.EncerrarCasoCasaMulher(ctx, c.so("sempras:recepcao"), caso.ID, "outro"); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("recepção não encerra: %v", err)
	}
	if _, err := s.EncerrarCasoCasaMulher(ctx, c.identity, uuid.New(), "outro"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("encerrar inexistente: %v", err)
	}
	// Encerrado, a mulher pode ter um caso novo; o prontuário traz os dois.
	if _, err := s.RegistrarCasoCasaMulher(ctx, c.identity, cid, c.unMulher, casoMulherInput()); err != nil {
		t.Fatalf("caso novo após o encerramento: %v", err)
	}
	p, err := s.Prontuario(ctx, c.identity, cid)
	if err != nil || len(p.CasaMulherCasos) != 2 {
		t.Fatalf("prontuário: %d casos, %v", len(p.CasaMulherCasos), err)
	}
}
