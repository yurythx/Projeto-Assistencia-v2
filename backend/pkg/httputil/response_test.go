package httputil

import (
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"os"
	"testing"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestWriteOK_ProducesStandardEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteOK(rec, map[string]string{"hello": "world"})

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var env Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Error != nil {
		t.Errorf("expected nil error, got %+v", env.Error)
	}
	if env.Data == nil {
		t.Error("expected non-nil data")
	}
}

func TestWriteAccepted_Returns202(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteAccepted(rec, map[string]string{"job_id": "1", "status": "queued"})
	if rec.Code != 202 {
		t.Fatalf("status = %d, want 202", rec.Code)
	}
}

func TestWriteError_AppError_UsesItsStatusAndCode(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	WriteError(rec, req, testLogger(), apperrors.NotFound("job not found"))

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	var env Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Data != nil {
		t.Error("expected nil data on error response")
	}
	if env.Error == nil || env.Error.Code != "NOT_FOUND" {
		t.Errorf("Error = %+v, want code NOT_FOUND", env.Error)
	}
	if env.Error.Message != "job not found" {
		t.Errorf("Message = %q", env.Error.Message)
	}
}

func TestWriteError_UnknownError_MapsTo500WithoutLeakingDetail(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	sensitive := "pq: password authentication failed for user \"nix\""
	WriteError(rec, req, testLogger(), errString(sensitive))

	if rec.Code != 500 {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	var env Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Error.Code != "INTERNAL_ERROR" {
		t.Errorf("Code = %q, want INTERNAL_ERROR", env.Error.Code)
	}
	if env.Error.Message == sensitive {
		t.Error("internal error detail leaked into the client-facing message")
	}
}

type errString string

func (e errString) Error() string { return string(e) }

func TestWriteError_ProblemJSON_WhenAccepted(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v2/jobs/42", nil)
	req.Header.Set("Accept", "application/problem+json")

	WriteError(rec, req, testLogger(), apperrors.Validation("campo 'nome' é obrigatório"))

	if rec.Code != 422 {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
	var p ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.Type != "urn:nexus:error:validation_error" {
		t.Errorf("type = %q", p.Type)
	}
	if p.Status != 422 || p.Code != "VALIDATION_ERROR" {
		t.Errorf("status=%d code=%q", p.Status, p.Code)
	}
	if p.Detail != "campo 'nome' é obrigatório" {
		t.Errorf("detail = %q", p.Detail)
	}
	if p.Instance != "/api/v2/jobs/42" {
		t.Errorf("instance = %q", p.Instance)
	}
}

func TestWriteError_KeepsEnvelope_WhenProblemJSONNotAccepted(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/jobs/42", nil)
	req.Header.Set("Accept", "application/json")

	WriteError(rec, req, testLogger(), apperrors.NotFound("não encontrado"))

	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want o envelope histórico", ct)
	}
	var env Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Error == nil || env.Error.Code != "NOT_FOUND" {
		t.Errorf("Error = %+v", env.Error)
	}
}
