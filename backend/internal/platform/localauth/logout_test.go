package localauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/yurythx/projeto-nexus/internal/platform/audit"
	"github.com/yurythx/projeto-nexus/internal/platform/auth"
	"github.com/yurythx/projeto-nexus/internal/platform/logging"
)

// capturingExecer satisfaz a interface (não exportada) que audit.NewWriter
// aceita, guardando os argumentos do último INSERT para inspeção — assim o
// teste do logout confere que a linha de auditoria de fato carrega ação,
// IP de origem e correlation_id (gap G-08 + G-07), sem tocar num Postgres.
type capturingExecer struct {
	calls    int
	lastArgs []any
}

func (c *capturingExecer) Exec(_ context.Context, _ string, args ...any) (pgconn.CommandTag, error) {
	c.calls++
	c.lastArgs = args
	return pgconn.CommandTag{}, nil
}

func TestLogout_RecordsAuditWithIdentityIPAndCorrelation(t *testing.T) {
	exec := &capturingExecer{}
	h := NewHandlers(newFakeStore(), testSigner(t), audit.NewWriter(exec), nil, nil, testLogger())

	uid := uuid.New()
	corr := uuid.NewString()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	r.RemoteAddr = "203.0.113.9:5555"
	ctx := auth.WithIdentity(r.Context(), auth.Identity{Subject: uid.String(), Source: auth.SourceLocal})
	ctx = logging.WithCorrelationID(ctx, corr)
	r = r.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.Logout(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", rec.Header().Get("Cache-Control"))
	}
	if exec.calls != 1 {
		t.Fatalf("audit Exec chamado %d vezes, want exatamente 1", exec.calls)
	}

	// Ordem dos args de audit.Writer.Record: [0]=id, [1]=actor_id,
	// [2]=actor_subject, [3]=actor_roles, [4]=ip, [5]=user_agent,
	// [6]=entity_context, [7]=action, [8]=resource_type, [9]=resource_id,
	// [10]=diff_before, [11]=diff_after, [12]=metadata, [13]=correlation_id.
	if got := exec.lastArgs[7]; got != audit.ActionLogout {
		t.Errorf("action = %v, want %q", got, audit.ActionLogout)
	}
	if got, _ := exec.lastArgs[4].(string); got != "203.0.113.9" {
		t.Errorf("ip_address = %v, want 203.0.113.9 (porta removida por sanitizeIP)", exec.lastArgs[4])
	}
	if cid, ok := exec.lastArgs[13].(*uuid.UUID); !ok || cid == nil || cid.String() != corr {
		t.Errorf("correlation_id = %v, want %s", exec.lastArgs[13], corr)
	}
	if userID, ok := exec.lastArgs[1].(*uuid.UUID); !ok || userID == nil || userID.String() != uid.String() {
		t.Errorf("user_id = %v, want %s (subject de token local é o id interno)", exec.lastArgs[1], uid)
	}
}

func TestLogout_WithoutIdentityIs401(t *testing.T) {
	h := NewHandlers(newFakeStore(), testSigner(t), nil, nil, nil, testLogger())

	r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	rec := httptest.NewRecorder()
	h.Logout(rec, r)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}
