package application_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yurythx/projeto-nexus/internal/modules/calendar/application"
	"github.com/yurythx/projeto-nexus/internal/modules/calendar/domain"
	"github.com/yurythx/projeto-nexus/internal/modules/calendar/infrastructure"
	"github.com/yurythx/projeto-nexus/internal/modules/calendar/transport"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/database/dbtest"
	"github.com/yurythx/projeto-nexus/internal/platform/outbox"
)

type env struct {
	t    *testing.T
	pool *pgxpool.Pool
	ana  auth.Identity
	base time.Time
}

func newEnv(t *testing.T) *env {
	pool := dbtest.Pool(t)
	return &env{t: t, pool: pool, ana: auth.Identity{UserID: dbtest.User(t, pool)}, base: time.Now().UTC().Add(240 * time.Hour).Truncate(time.Hour)}
}

func (e *env) svc(repo domain.Repository) *application.Service {
	return application.NewService(e.pool, repo, outbox.NewWriter("test"))
}

func (e *env) real() *application.Service { return e.svc(infrastructure.NewRepository()) }

func (e *env) room() domain.Room {
	e.t.Helper()
	r, err := e.real().SaveRoom(context.Background(), gestor, uuid.Nil, domain.Room{Name: "Sala " + uuid.NewString()[:8], Active: true})
	if err != nil {
		e.t.Fatal(err)
	}
	return r
}

func (e *env) input(room *uuid.UUID, offset time.Duration) application.EventInput {
	return application.EventInput{Title: "Evento", Visibility: "internal", RoomID: room, StartsAt: e.base.Add(offset), EndsAt: e.base.Add(offset + time.Hour)}
}

func (e *env) event(room *uuid.UUID, offset time.Duration) domain.Event {
	e.t.Helper()
	ev, err := e.real().CreateEvent(context.Background(), e.ana, e.input(room, offset))
	if err != nil {
		e.t.Fatal(err)
	}
	return ev
}

func TestEveryRepositoryFailureIsPropagated(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	offset := time.Duration(0)
	next := func() time.Duration { offset += 2 * time.Hour; return offset }
	type op = func() func(s *application.Service) error
	ops := map[string]op{
		"Events": func() func(*application.Service) error {
			return func(s *application.Service) error {
				_, err := s.Events(ctx, e.ana, e.base, e.base.Add(time.Hour), nil)
				return err
			}
		},
		"PublicEvents": func() func(*application.Service) error {
			return func(s *application.Service) error {
				_, err := s.PublicEvents(ctx, e.base, e.base.Add(time.Hour))
				return err
			}
		},
		"Event": func() func(*application.Service) error {
			ev := e.event(nil, 0)
			return func(s *application.Service) error { _, err := s.Event(ctx, e.ana, ev.ID); return err }
		},
		"CreateEvent": func() func(*application.Service) error {
			r := e.room()
			return func(s *application.Service) error {
				_, err := s.CreateEvent(ctx, e.ana, e.input(&r.ID, next()))
				return err
			}
		},
		"UpdateEvent": func() func(*application.Service) error {
			ev := e.event(nil, 0)
			r := e.room()
			return func(s *application.Service) error {
				_, err := s.UpdateEvent(ctx, e.ana, ev.ID, e.input(&r.ID, next()))
				return err
			}
		},
		"CancelEvent": func() func(*application.Service) error {
			ev := e.event(nil, 0)
			return func(s *application.Service) error { _, err := s.CancelEvent(ctx, e.ana, ev.ID); return err }
		},
		"DeleteEvent": func() func(*application.Service) error {
			ev := e.event(nil, 0)
			return func(s *application.Service) error { return s.DeleteEvent(ctx, e.ana, ev.ID) }
		},
		"Rooms": func() func(*application.Service) error {
			return func(s *application.Service) error { _, err := s.Rooms(ctx, e.ana); return err }
		},
		"RoomBusy": func() func(*application.Service) error {
			r := e.room()
			return func(s *application.Service) error {
				_, err := s.RoomBusy(ctx, r.ID, e.base, e.base.Add(time.Hour))
				return err
			}
		},
		"SaveRoom": func() func(*application.Service) error {
			r := e.room()
			return func(s *application.Service) error { _, err := s.SaveRoom(ctx, gestor, r.ID, r); return err }
		},
		"DeleteRoom": func() func(*application.Service) error {
			r := e.room()
			return func(s *application.Service) error { return s.DeleteRoom(ctx, gestor, r.ID) }
		},
	}
	for name, o := range ops {
		probe := &faultRepo{inner: infrastructure.NewRepository()}
		if err := o()(e.svc(probe)); err != nil {
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
				if err := o()(e.svc(r)); err == nil && !r.noTx {
					t.Errorf("%s: falha (veneno=%v) na chamada %d (%s) foi engolida", name, poison, i+1, probe.trace[i])
				}
			}
		}
	}
	if err := application.MapError(errBoom); !errors.Is(err, errBoom) {
		t.Error("erro desconhecido passa adiante")
	}
	if _, err := e.real().SaveRoom(ctx, gestor, uuid.Nil, domain.Room{Name: "Negativa " + uuid.NewString()[:6], Capacity: -5}); err == nil ||
		!strings.Contains(err.Error(), "valor fora do permitido") {
		t.Errorf("capacidade negativa é validação, não \"intervalo\": %v", err)
	}
}

// Ocupação: evento privado aparece como "Reservado" (sem expor o título).
func TestRoomBusyHidesPrivateTitles(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	r := e.room()
	in := e.input(&r.ID, 0)
	in.Title, in.Visibility = "Consulta médica", "private"
	if _, err := e.real().CreateEvent(ctx, e.ana, in); err != nil {
		t.Fatal(err)
	}
	busy, err := e.real().RoomBusy(ctx, r.ID, e.base, e.base.Add(2*time.Hour))
	if err != nil || len(busy) != 1 || busy[0].Title != "Reservado" {
		t.Fatalf("ocupação sem o título do evento privado: %+v %v", busy, err)
	}
	if _, err := e.real().RoomBusy(ctx, r.ID, e.base, e.base); err == nil {
		t.Fatal("período vazio é inválido")
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
	h.RegisterPublicRoutes(r)
	h.RegisterAuthedRoutes(r)
	for _, path := range []string{"/calendar/events", "/calendar/public/events", "/calendar/rooms"} {
		down.calls = 0
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), errBoom.Error()) {
			t.Errorf("GET %s com o banco fora: %d %s", path, rec.Code, rec.Body.String())
		}
	}
}
