package application_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/application"
	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/domain"
	"github.com/yurythx/projeto-assistencia-v2/internal/modules/sempras/infrastructure"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/database"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/database/dbtest"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/notificacoes"
	"github.com/yurythx/projeto-assistencia-v2/internal/platform/outbox"
)

// casasFixas limita a varredura às Casas Abrigo dadas (o banco de teste é
// compartilhado) ou faz a lista delas falhar.
type casasFixas struct {
	domain.Repository
	unidades []uuid.UUID
	err      error
}

func (r casasFixas) UnidadesDoTipo(context.Context, database.DBTX, string) ([]uuid.UUID, error) {
	return r.unidades, r.err
}

type hubAvisos struct {
	mu      sync.Mutex
	topicos []string
}

func (h *hubAvisos) Publish(_ context.Context, topic, _ string, _ any) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.topicos = append(h.topicos, topic)
	return nil
}

// lotarNaCasa põe um usuário na Casa Abrigo do cenário com o perfil dado.
func (c cenario) lotarNaCasa(t *testing.T, perms ...string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	u := dbtest.User(t, c.pool)
	var perfil uuid.UUID
	if err := c.pool.QueryRow(ctx, `INSERT INTO perfis (slug, nome, permissoes) VALUES ($1, $1, $2) RETURNING id`,
		"perfil-"+uuid.NewString()[:8], perms).Scan(&perfil); err != nil {
		t.Fatal(err)
	}
	if _, err := c.pool.Exec(ctx, `INSERT INTO user_scopes (user_id, perfil_id, unidade_id) VALUES ($1, $2, $3)`, u, perfil, c.unAbrigo); err != nil {
		t.Fatal(err)
	}
	return u
}

func avisosDe(t *testing.T, c cenario, usuario uuid.UUID) []string {
	t.Helper()
	rows, err := c.pool.Query(context.Background(), `SELECT chave FROM notificacoes WHERE user_id = $1 ORDER BY chave`, usuario)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			t.Fatal(err)
		}
		out = append(out, k)
	}
	return out
}

// Os avisos de prazo (ADR 032): a equipe da casa recebe um aviso por prazo
// e nível, uma vez só; a recepção não recebe; sem equipe, nada é gravado.
func TestAvisosDePrazoCasaAbrigo(t *testing.T) {
	c := novoCenario(t)
	ctx := context.Background()
	tecnico, recepcao := c.lotarNaCasa(t, "sempras:atendimento"), c.lotarNaCasa(t, "sempras:recepcao")
	rel := c.relatorio(t, nil) // prazo em 5 dias: dentro da janela de 7
	prazo := time.Now().AddDate(0, 0, -2)
	tf, err := c.limpo().CriarTarefa(ctx, c.identity, c.unAbrigo, domain.TarefaDados{Titulo: "Ofício ao fórum", Prazo: &prazo})
	if err != nil {
		t.Fatal(err)
	}
	hub := &hubAvisos{}
	repo := casasFixas{Repository: infrastructure.NewRepository(), unidades: []uuid.UUID{c.unAbrigo}}
	s := application.NewService(c.pool, repo, outbox.NewWriter("nexus.sempras"), publisher{}, c.store, bucket, 1<<20, time.Minute, testLogger()).
		WithNotificacoes(notificacoes.NewService(c.pool, hub, testLogger()))

	n, err := s.AvisarPrazosUmaVez(ctx)
	if err != nil || n != 2 {
		t.Fatalf("primeira varredura: %d %v", n, err)
	}
	chaves := avisosDe(t, c, tecnico)
	if len(chaves) != 2 || !strings.HasPrefix(chaves[0], "sempras:casa-abrigo:relatorio:"+rel.ID.String()+":") ||
		!strings.HasSuffix(chaves[0], ":aviso") || !strings.HasPrefix(chaves[1], "sempras:casa-abrigo:tarefa:"+tf.ID.String()+":") ||
		!strings.HasSuffix(chaves[1], ":atraso") {
		t.Errorf("avisos do técnico: %v", chaves)
	}
	if len(avisosDe(t, c, recepcao)) != 0 {
		t.Error("a recepção não recebe os avisos de prazo")
	}
	if len(hub.topicos) != 2 {
		t.Errorf("entrega em tempo real: %v", hub.topicos)
	}
	if n, err := s.AvisarPrazosUmaVez(ctx); err != nil || n != 0 {
		t.Errorf("segunda varredura repete avisos: %d %v", n, err)
	}

	// sem a entrega em tempo real e numa casa sem equipe: nada a gravar
	semEquipe := casasFixas{Repository: infrastructure.NewRepository(), unidades: []uuid.UUID{c.unMulher}}
	if n, err := c.svc(semEquipe, publisher{}).AvisarPrazosUmaVez(ctx); err != nil || n != 0 {
		t.Errorf("casa sem equipe: %d %v", n, err)
	}
}

// Cada leitura da varredura que falha (ou a transação envenenada antes de
// gravar o aviso) vira erro da unidade; a lista das casas que falha, erro
// da varredura.
func TestAvisosDePrazoFalhas(t *testing.T) {
	c := novoCenario(t)
	ctx := context.Background()
	c.lotarNaCasa(t, "sempras:gestao")
	c.relatorio(t, nil)
	if _, err := c.svc(casasFixas{Repository: infrastructure.NewRepository(), err: dbtest.ErrInjected}, publisher{}).
		AvisarPrazosUmaVez(ctx); !errors.Is(err, dbtest.ErrInjected) {
		t.Errorf("lista das casas fora: %v", err)
	}
	// EquipeDaUnidade, AcolhidosDaUnidade, MetasPendentesDaUnidade, ListRelatorios, ListTarefas
	for i := 1; i <= 5; i++ {
		for _, poison := range []bool{false, true} {
			f := &faultRepo{inner: infrastructure.NewRepository()}
			if poison {
				f.poisonAt = i
			} else {
				f.failAt = i
			}
			s := c.svc(casasFixas{Repository: f, unidades: []uuid.UUID{c.unAbrigo}}, publisher{})
			if n, err := s.AvisarPrazosUmaVez(ctx); err == nil || n != 0 || !strings.Contains(err.Error(), c.unAbrigo.String()) {
				t.Errorf("falha (veneno=%v) na chamada %d: %d %v", poison, i, n, err)
			}
		}
	}
}

// O laço do worker: varre ao subir e a cada intervalo; a falha vai para o
// log e o laço segue até o contexto acabar.
func TestAvisosDePrazoWorker(t *testing.T) {
	c := novoCenario(t)
	var log bytes.Buffer
	s := application.NewService(c.pool, casasFixas{Repository: infrastructure.NewRepository(), err: dbtest.ErrInjected},
		outbox.NewWriter("nexus.sempras"), publisher{}, c.store, bucket, 1<<20, time.Minute, slog.New(slog.NewTextHandler(&log, nil)))
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	if err := s.AvisarACada(ctx, 20*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if strings.Count(log.String(), "avisos de prazo da Casa Abrigo falharam") < 2 {
		t.Errorf("o laço deveria tentar de novo e registrar no log: %q", log.String())
	}
	parado, para := context.WithCancel(context.Background())
	para()
	if err := c.limpo().AvisarPrazosCasaAbrigo(parado); err != nil {
		t.Errorf("worker com o contexto encerrado: %v", err)
	}
}

// O texto e o link de cada aviso.
func TestAvisoDePrazo(t *testing.T) {
	un, ac, ref := uuid.New(), uuid.New(), uuid.New()
	prazo := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	casos := []struct {
		alerta    domain.Alerta
		link, msg string
	}{
		{domain.Alerta{Tipo: "pia", Nivel: "aviso", Mensagem: "Reavaliação do PIA de B. A.", Prazo: prazo, Dias: 3, AcolhimentoID: &ac, RefID: ac},
			"/sempras/u/" + un.String() + "?aba=pia&acolhido=" + ac.String(), "Casa Abrigo — Vence em 3 dia(s), em 20/10/2026."},
		{domain.Alerta{Tipo: "meta", Nivel: "aviso", Mensagem: "Meta", Prazo: prazo, RefID: ref},
			"/sempras/u/" + un.String() + "?aba=pia", "Casa Abrigo — Vence hoje (20/10/2026)."},
		{domain.Alerta{Tipo: "relatorio", Nivel: "atraso", Mensagem: "Relatório atrasado", Prazo: prazo, Dias: -4, RefID: ref},
			"/sempras/u/" + un.String() + "?aba=relatorios", "Casa Abrigo — Atrasado há 4 dia(s): o prazo era 20/10/2026."},
		{domain.Alerta{Tipo: "tarefa", Nivel: "aviso", Mensagem: strings.Repeat("á", 250), Prazo: prazo, Dias: 1, RefID: ref},
			"/sempras/u/" + un.String() + "?aba=tarefas", "Casa Abrigo — Vence em 1 dia(s), em 20/10/2026."},
	}
	for _, cs := range casos {
		n := application.AvisoDePrazo(un, cs.alerta)
		if n.Link != cs.link || n.Mensagem != cs.msg || n.Validar() != nil || n.Modulo != "sempras" ||
			n.Chave != "sempras:casa-abrigo:"+cs.alerta.Tipo+":"+cs.alerta.RefID.String()+":2026-10-20:"+cs.alerta.Nivel {
			t.Errorf("aviso de %s: %+v", cs.alerta.Tipo, n)
		}
	}
	if n := application.AvisoDePrazo(un, casos[3].alerta); !strings.HasSuffix(n.Titulo, "…") {
		t.Errorf("título longo cortado: %q", n.Titulo)
	}
}
