package application_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/domain/events"
	"github.com/yurythx/projeto-nexus/internal/modules/tramite/application"
	"github.com/yurythx/projeto-nexus/internal/modules/tramite/domain"
	"github.com/yurythx/projeto-nexus/internal/modules/tramite/infrastructure"
	"github.com/yurythx/projeto-nexus/internal/platform/notificacoes"
)

type hubNulo struct{ n int }

func (h *hubNulo) Publish(context.Context, string, string, any) error { h.n++; return nil }

func eventoAtlas(t *testing.T, payload map[string]any) events.Event {
	t.Helper()
	ev, err := events.New("atlas.workflow.deactivated", "atlas", uuid.Nil, payload)
	if err != nil {
		t.Fatal(err)
	}
	return ev
}

func (e *env) aviso(processo, wf uuid.UUID) (string, bool) {
	e.t.Helper()
	var msg string
	err := e.pool.QueryRow(context.Background(), `SELECT mensagem FROM notificacoes WHERE user_id = $1 AND chave = $2`,
		e.author.UserID, "tramite:atlas:"+processo.String()+":"+wf.String()).Scan(&msg)
	return msg, err == nil
}

// Versão do procedimento substituída ou desativada no Atlas: quem abriu um
// processo em andamento que segue aquela versão é avisado, uma vez.
func TestAvisoDoAtlas(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	hub := &hubNulo{}
	s := e.real().WithNotificacoes(notificacoes.NewService(e.pool, hub, slog.New(slog.NewTextHandler(io.Discard, nil))))
	wf := uuid.New()
	abrir := func() domain.Processo {
		p, err := s.Abrir(ctx, e.author, application.AbrirInput{TipoID: e.tipo, Assunto: "Pregão", Sigilo: "publico",
			UnidadeOrigemID: e.unidade, AtlasProcedimentoID: &wf, CodigoTTDD: "2.0.02.00.07"})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	andamento, encerrado := abrir(), abrir()
	if _, err := s.Concluir(ctx, e.author, encerrado.ID, "Fim"); err != nil {
		t.Fatal(err)
	}
	nova := uuid.New()
	ev := eventoAtlas(t, map[string]any{"id": wf, "codigo_processual": "ADM.LIC.001", "versao": 1, "substituido_por": nova, "versao_nova": 2})
	for range 2 { // a reentrega do evento não duplica o aviso
		if err := s.HandleAtlasEvent(ctx, ev); err != nil {
			t.Fatal(err)
		}
	}
	msg, ok := e.aviso(andamento.ID, wf)
	if !ok || !strings.Contains(msg, "a versão 2 entrou em vigor") || hub.n != 1 {
		t.Fatalf("aviso da versão nova: %q %v (entregas %d)", msg, ok, hub.n)
	}
	if _, ok := e.aviso(encerrado.ID, wf); ok {
		t.Fatal("processo encerrado não é avisado")
	}

	// Desativado sem substituta.
	wf2 := uuid.New()
	p, err := s.Abrir(ctx, e.author, application.AbrirInput{TipoID: e.tipo, Assunto: "x", Sigilo: "publico",
		UnidadeOrigemID: e.unidade, AtlasProcedimentoID: &wf2})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.HandleAtlasEvent(ctx, eventoAtlas(t, map[string]any{"id": wf2, "codigo_processual": "X", "versao": 3})); err != nil {
		t.Fatal(err)
	}
	if msg, ok := e.aviso(p.ID, wf2); !ok || !strings.Contains(msg, "foi desativada no Atlas") {
		t.Fatalf("aviso de desativação: %q %v", msg, ok)
	}

	// Payload inválido ou avisos desligados: nada a reprocessar.
	if err := s.HandleAtlasEvent(ctx, events.Event{Payload: json.RawMessage(`[]`)}); err != nil {
		t.Fatal(err)
	}
	if err := e.real().HandleAtlasEvent(ctx, ev); err != nil {
		t.Fatal(err)
	}

	// Falha no banco volta para a fila (retry): na consulta ou no aviso.
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, falha := range []struct{ fail, poison int }{{1, 0}, {0, 1}} {
		outro := uuid.New()
		if _, err := s.Abrir(ctx, e.author, application.AbrirInput{TipoID: e.tipo, Assunto: "x", Sigilo: "publico",
			UnidadeOrigemID: e.unidade, AtlasProcedimentoID: &outro}); err != nil {
			t.Fatal(err)
		}
		r := &faultRepo{Repository: infrastructure.NewRepository(), failAt: falha.fail, poisonAt: falha.poison}
		svc := e.svc(r, fakeSign{available: true}).WithNotificacoes(notificacoes.NewService(e.pool, hub, log))
		if err := svc.HandleAtlasEvent(ctx, eventoAtlas(t, map[string]any{"id": outro, "codigo_processual": "X", "versao": 1})); err == nil {
			t.Errorf("falha %+v engolida", falha)
		}
	}
}
