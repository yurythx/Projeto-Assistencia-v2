package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yurythx/projeto-nexus/internal/modules/mercurio/application"
	"github.com/yurythx/projeto-nexus/internal/modules/mercurio/domain"
	"github.com/yurythx/projeto-nexus/internal/modules/mercurio/infrastructure"
	"github.com/yurythx/projeto-nexus/internal/modules/mercurio/transport"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
	"github.com/yurythx/projeto-nexus/internal/platform/outbox"
	"github.com/yurythx/projeto-nexus/internal/platform/ws"
)

// fakeHub registra publicações e tópicos derrubados.
type fakeHub struct {
	mu        sync.Mutex
	published []string
	dropped   []string
	err       error
}

func (h *fakeHub) Publish(_ context.Context, topic, frameType string, _ any) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.published = append(h.published, frameType+"@"+topic)
	return h.err
}

func (h *fakeHub) DropTopic(topic string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.dropped = append(h.dropped, topic)
}

type env struct {
	t          *testing.T
	pool       *pgxpool.Pool
	hub        *fakeHub
	ana, beto  auth.Identity
	moderator  auth.Identity
	globalRoom domain.Room
}

func newEnv(t *testing.T) *env {
	pool := dbtest.Pool(t)
	e := &env{t: t, pool: pool, hub: &fakeHub{},
		ana:  auth.Identity{UserID: dbtest.User(t, pool), Username: "ana"},
		beto: auth.Identity{UserID: dbtest.User(t, pool), Username: "beto"},
		moderator: auth.Identity{UserID: dbtest.User(t, pool), Username: "mod", Permissions: []string{string(auth.PermMercurioManage)},
			Scopes: []auth.Scope{{Perfil: "moderador", Permissions: []string{string(auth.PermMercurioManage)}}}},
	}
	r, err := e.real().SaveRoom(context.Background(), e.moderator, uuid.Nil, application.RoomInput{Kind: "global", Name: "Geral " + uuid.NewString()[:6]})
	if err != nil {
		t.Fatal(err)
	}
	e.globalRoom = r
	return e
}

func (e *env) svc(repo domain.Repository) *application.Service {
	return application.NewService(e.pool, repo, e.hub, outbox.NewWriter("test"), slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func (e *env) real() *application.Service { return e.svc(infrastructure.NewRepository()) }

func (e *env) message() domain.Message {
	e.t.Helper()
	m, err := e.real().Send(context.Background(), e.ana, e.globalRoom.ID, "olá")
	if err != nil {
		e.t.Fatal(err)
	}
	return m
}

func TestEveryRepositoryFailureIsPropagated(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	type op = func() func(s *application.Service) error
	ops := map[string]op{
		"Rooms": func() func(*application.Service) error {
			if _, err := e.real().Direct(ctx, e.ana, e.beto.UserID); err != nil {
				t.Fatal(err)
			}
			return func(s *application.Service) error { _, err := s.Rooms(ctx, e.ana); return err }
		},
		"SaveRoom": func() func(*application.Service) error {
			return func(s *application.Service) error {
				_, err := s.SaveRoom(ctx, e.moderator, e.globalRoom.ID, application.RoomInput{Kind: "global", Name: e.globalRoom.Name})
				return err
			}
		},
		"Direct": func() func(*application.Service) error {
			other := auth.Identity{UserID: dbtest.User(t, e.pool)}
			return func(s *application.Service) error { _, err := s.Direct(ctx, other, e.beto.UserID); return err }
		},
		"Messages": func() func(*application.Service) error {
			return func(s *application.Service) error {
				_, err := s.Messages(ctx, e.ana, e.globalRoom.ID, nil, 0)
				return err
			}
		},
		"Send": func() func(*application.Service) error {
			return func(s *application.Service) error { _, err := s.Send(ctx, e.ana, e.globalRoom.ID, "oi"); return err }
		},
		"Edit": func() func(*application.Service) error {
			m := e.message()
			return func(s *application.Service) error { _, err := s.Edit(ctx, e.ana, m.ID, "editada"); return err }
		},
		"Delete": func() func(*application.Service) error {
			m := e.message()
			return func(s *application.Service) error { return s.Delete(ctx, e.ana, m.ID) }
		},
		"Moderate": func() func(*application.Service) error {
			m := e.message()
			return func(s *application.Service) error { return s.Delete(ctx, e.moderator, m.ID) }
		},
		"MarkRead": func() func(*application.Service) error {
			return func(s *application.Service) error { return s.MarkRead(ctx, e.ana, e.globalRoom.ID) }
		},
	}
	// O nome do outro participante na lista de conversas é cosmético: se
	// não puder ser lido, a sala aparece com o nome genérico.
	tolerant := map[string]bool{"Rooms/UserName": true}
	for name, o := range ops {
		probe := &faultRepo{inner: infrastructure.NewRepository()}
		if err := o()(e.svc(probe)); err != nil {
			t.Fatalf("%s sem falha: %v", name, err)
		}
		for i := range probe.trace {
			if tolerant[name+"/"+probe.trace[i]] {
				continue
			}
			for _, poison := range []bool{false, true} {
				r := &faultRepo{inner: infrastructure.NewRepository()}
				if poison {
					r.poisonAt = i + 1
				} else {
					r.failAt = i + 1
				}
				if err := o()(e.svc(r)); err == nil && !r.noTx {
					t.Errorf("%s: falha (veneno=%v) na chamada %d (%s) foi engolida", name, poison, i+1, probe.trace[i])
				}
			}
		}
	}
}

// Tempo real: publicação após o commit, aviso pessoal nas diretas,
// inscrições derrubadas ao mudar quem participa e falha do Hub só logada.
func TestRealtime(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	dm, err := e.real().Direct(ctx, e.ana, e.beto.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if rooms, err := e.real().Rooms(ctx, e.ana); err != nil || !hasRoomNamed(rooms, dm.ID, "Usuário") {
		t.Fatalf("conversa direta exibe o nome do outro participante: %+v %v", rooms, err)
	}
	e.hub.published = nil
	if _, err := e.real().Send(ctx, e.ana, dm.ID, "oi, Beto"); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(e.hub.published, ",")
	if !strings.Contains(joined, "mercurio.message@"+application.RoomTopic(dm.ID)) || !strings.Contains(joined, "mercurio.unread@"+ws.UserTopic(e.beto.UserID.String())) {
		t.Fatalf("mensagem na sala e aviso de não lida no tópico pessoal: %s", joined)
	}
	e.hub.err = errors.New("hub fora")
	if _, err := e.real().Send(ctx, e.ana, dm.ID, "ainda persiste"); err != nil {
		t.Fatalf("falha no tempo real não perde a mensagem (já gravada): %v", err)
	}
	e.hub.err = nil

	dep := uuid.New()
	sala, err := e.real().SaveRoom(ctx, e.moderator, uuid.Nil, application.RoomInput{Kind: "department", Name: "Sala", ADGroup: "GRP_A"})
	if err != nil {
		t.Fatal(err)
	}
	e.hub.dropped = nil
	if _, err := e.real().SaveRoom(ctx, e.moderator, sala.ID, application.RoomInput{Kind: "department", Name: "Sala renomeada", ADGroup: "GRP_A"}); err != nil || len(e.hub.dropped) != 0 {
		t.Fatalf("renomear não derruba inscrições: %v %v", err, e.hub.dropped)
	}
	if _, err := e.real().SaveRoom(ctx, e.moderator, sala.ID, application.RoomInput{Kind: "department", Name: "Sala", ADGroup: "GRP_B"}); err != nil || len(e.hub.dropped) != 1 {
		t.Fatalf("mudar o grupo derruba as inscrições (reautorização): %v %v", err, e.hub.dropped)
	}
	_ = dep
}

func hasRoomNamed(rooms []domain.Room, id uuid.UUID, prefix string) bool {
	for _, r := range rooms {
		if r.ID == id {
			return strings.HasPrefix(r.Name, prefix)
		}
	}
	return false
}

// Autorização de tópicos WebSocket e indicador de digitação.
func TestWebSocketAuthorizationAndTyping(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	s := e.real()
	dm, err := s.Direct(ctx, e.ana, e.beto.UserID)
	if err != nil {
		t.Fatal(err)
	}
	ana := ws.ClientInfo{UserID: e.ana.UserID.String(), Username: "ana"}
	intrusa := ws.ClientInfo{UserID: uuid.NewString(), Username: "intrusa"}

	if err := s.Authorize(ctx, ana, application.RoomTopic(dm.ID)); err != nil {
		t.Fatalf("participante assina a sala: %v", err)
	}
	for name, c := range map[string]struct {
		client ws.ClientInfo
		topic  string
	}{
		"não participante": {intrusa, application.RoomTopic(dm.ID)},
		"outro módulo":     {ana, "files:folder:x"},
		"id malformado":    {ana, "mercurio:room:xyz"},
		"sala inexistente": {ana, application.RoomTopic(uuid.New())},
	} {
		if err := s.Authorize(ctx, c.client, c.topic); !errors.Is(err, ws.ErrTopicForbidden) {
			t.Errorf("%s: %v", name, err)
		}
	}

	typing := func(c ws.ClientInfo, typ string, data any) error {
		raw, _ := json.Marshal(data)
		return s.Inbound(ctx, c, ws.Frame{Type: typ, Data: raw})
	}
	if err := typing(ana, "mercurio.typing", map[string]string{"room_id": dm.ID.String()}); err != nil {
		t.Fatalf("digitação publicada: %v", err)
	}
	if err := typing(intrusa, "mercurio.typing", map[string]string{"room_id": dm.ID.String()}); err == nil {
		t.Error("não participante não publica digitação")
	}
	if err := typing(ana, "mercurio.qualquer", nil); err == nil {
		t.Error("frame desconhecido é recusado")
	}
	if err := s.Inbound(ctx, ana, ws.Frame{Type: "mercurio.typing", Data: json.RawMessage(`{`)}); err == nil {
		t.Error("frame malformado é recusado")
	}
	if err := application.MapError(errBoom); !errors.Is(err, errBoom) {
		t.Errorf("erro desconhecido passa adiante: %v", err)
	}
}

func TestHandlersReportServiceFailures(t *testing.T) {
	e := newEnv(t)
	down := &faultRepo{inner: infrastructure.NewRepository(), failAt: 1}
	h := transport.NewHandlers(e.svc(down), slog.New(slog.NewTextHandler(io.Discard, nil)))
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.WithIdentity(req.Context(), e.ana)))
		})
	})
	h.RegisterRoutes(r)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/mercurio/rooms", nil))
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), errBoom.Error()) {
		t.Errorf("lista de salas com o banco fora: %d %s", rec.Code, rec.Body.String())
	}
}

// Validações do serviço (a API também as faz, mas outros chamadores não
// passam pelo handler).
func TestServiceValidations(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	s := e.real()
	if _, err := s.SaveRoom(ctx, e.moderator, uuid.Nil, application.RoomInput{Kind: "direct", Name: "x"}); err == nil {
		t.Error("tipo de sala inválido")
	}
	if _, err := s.Send(ctx, e.ana, e.globalRoom.ID, "   "); err == nil {
		t.Error("mensagem vazia")
	}
	dep1, dep2 := dbtest.Unidade(t, e.pool), uuid.New()
	_ = dep1
	var depID uuid.UUID
	if err := e.pool.QueryRow(ctx, `INSERT INTO departamentos (unidade_id, nome, slug) VALUES ($1, 'D', $2) RETURNING id`, dep1, uuid.NewString()).Scan(&depID); err != nil {
		t.Fatal(err)
	}
	sala, err := s.SaveRoom(ctx, e.moderator, uuid.Nil, application.RoomInput{Kind: "department", Name: "D", DepartamentoID: &depID})
	if err != nil {
		t.Fatal(err)
	}
	e.hub.dropped = nil
	same := depID
	if _, err := s.SaveRoom(ctx, e.moderator, sala.ID, application.RoomInput{Kind: "department", Name: "D2", DepartamentoID: &same}); err != nil || len(e.hub.dropped) != 0 {
		t.Fatalf("mesmo departamento: nada muda no acesso: %v %v", err, e.hub.dropped)
	}
	if _, err := s.SaveRoom(ctx, e.moderator, sala.ID, application.RoomInput{Kind: "department", Name: "D2", DepartamentoID: &dep2}); err == nil {
		t.Fatal("departamento inexistente é recusado")
	}
}
