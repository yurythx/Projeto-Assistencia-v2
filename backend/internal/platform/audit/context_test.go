package audit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/platform/logging"
)

func TestFromRequest_PopulatesIPAndCorrelation(t *testing.T) {
	corr := uuid.NewString()
	req := httptest.NewRequest("PATCH", "/api/v1/admin/feature-flags/x", nil)
	req.RemoteAddr = "203.0.113.7:54812"
	req = req.WithContext(logging.WithCorrelationID(req.Context(), corr))

	e := FromRequest(req)

	if e.IPAddress != "203.0.113.7:54812" {
		t.Errorf("IPAddress = %q, want the raw RemoteAddr (sanitizeIP strips the port at Record time)", e.IPAddress)
	}
	if e.CorrelationID == nil || e.CorrelationID.String() != corr {
		t.Errorf("CorrelationID = %v, want %s", e.CorrelationID, corr)
	}
}

func TestFromRequest_NonUUIDCorrelationIsDropped(t *testing.T) {
	req := httptest.NewRequest("PATCH", "/x", nil)
	req = req.WithContext(logging.WithCorrelationID(req.Context(), "client-supplied-not-a-uuid"))

	e := FromRequest(req)

	if e.CorrelationID != nil {
		t.Errorf("CorrelationID = %v, want nil for a non-UUID X-Request-ID", e.CorrelationID)
	}
}

func TestFromRequest_NoCorrelationInContext(t *testing.T) {
	req := httptest.NewRequest("PATCH", "/x", nil)
	req.RemoteAddr = "198.51.100.4:1010"

	e := FromRequest(req)

	if e.CorrelationID != nil {
		t.Errorf("CorrelationID = %v, want nil", e.CorrelationID)
	}
	if e.IPAddress != "198.51.100.4:1010" {
		t.Errorf("IPAddress = %q", e.IPAddress)
	}
}

func TestCaptureOriginFeedsFromContext(t *testing.T) {
	var got Entry
	h := CaptureOrigin(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = Meta(r.Context(), "x.y", "res", "1", nil, nil)
	}))
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.RemoteAddr = "198.51.100.23:4567"
	req.Header.Set("User-Agent", "navegador-teste/1.0")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if got.IPAddress != "198.51.100.23" || got.UserAgent != "navegador-teste/1.0" {
		t.Fatalf("origem não propagada para a auditoria: ip=%q ua=%q", got.IPAddress, got.UserAgent)
	}
	if e := FromContext(context.Background()); e.IPAddress != "" || e.UserAgent != "" {
		t.Fatal("sem CaptureOrigin a origem fica vazia (nunca inventada)")
	}
}
