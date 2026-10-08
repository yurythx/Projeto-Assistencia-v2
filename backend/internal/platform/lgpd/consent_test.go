package lgpd

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yurythx/projeto-nexus/internal/platform/auth"
)

// Consentimento de usuário FEDERADO (Keycloak -> AD): o subject não é o id
// interno. Antes, status respondia "aceito" fixo e o aceite nunca era
// gravado — o modal LGPD simplesmente não aparecia para esses usuários.
func TestConsentForFederatedUser(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL não definido")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	s := NewService(pool, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)

	sub := "kc-" + uuid.NewString()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO users (keycloak_subject, username, email) VALUES ($1, $1, $1 || '@ad.test') RETURNING id`, sub).Scan(&id); err != nil {
		t.Fatal(err)
	}
	identity := auth.Identity{Subject: sub, Username: sub, Source: auth.SourceKeycloak, UserID: id}

	call := func(h http.HandlerFunc, method, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/", strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		req = req.WithContext(auth.WithIdentity(req.Context(), identity))
		rec := httptest.NewRecorder()
		h(rec, req)
		return rec
	}

	if rec := call(s.handleStatus, http.MethodGet, ""); !strings.Contains(rec.Body.String(), `"accepted":false`) {
		t.Fatalf("federado sem aceite deve ver o termo: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(s.handleAccept, http.MethodPost, `{"term_version":"v0-antiga"}`); rec.Code != http.StatusConflict {
		t.Fatalf("versão antiga/inventada não vale como consentimento: %d", rec.Code)
	}
	if rec := call(s.handleAccept, http.MethodPost, `{"term_version":"`+CurrentTermVersion+`","extra":1}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("campos desconhecidos são recusados: %d", rec.Code)
	}
	if rec := call(s.handleAccept, http.MethodPost, `{"term_version":"`+CurrentTermVersion+`"}`); rec.Code != http.StatusOK {
		t.Fatalf("aceite: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(s.handleStatus, http.MethodGet, ""); !strings.Contains(rec.Body.String(), `"accepted":true`) {
		t.Fatalf("aceite do federado deveria ficar gravado: %s", rec.Body.String())
	}
	ok, err := s.HasAcceptedCurrentTerm(context.Background(), id, CurrentTermVersion)
	if err != nil || !ok {
		t.Fatalf("registro no banco: %v %v", ok, err)
	}
}
