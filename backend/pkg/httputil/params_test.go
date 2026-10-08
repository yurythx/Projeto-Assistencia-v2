package httputil

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
)

func TestQueryTruncatesByCharacter(t *testing.T) {
	cases := map[string]struct {
		raw  string
		max  int
		want string
	}{
		"curto":            {"q=%20oi%20", 10, "oi"},
		"acentos":          {"q=a%C3%A7%C3%A3o", 3, "açã"}, // "ação": 4 caracteres, 6 bytes
		"utf8 inválido":    {"q=ab%FFcd", 10, "abcd"},
		"exatamente o max": {"q=abc", 3, "abc"},
	}
	for name, tc := range cases {
		r := httptest.NewRequest(http.MethodGet, "/?"+tc.raw, nil)
		got := Query(r, "q", tc.max)
		if got != tc.want || !utf8.ValidString(got) {
			t.Errorf("%s: Query = %q, quer %q", name, got, tc.want)
		}
	}
}

func TestParams(t *testing.T) {
	id := uuid.New()
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id.String())
	rctx.URLParams.Add("ruim", "x")
	r := httptest.NewRequest(http.MethodGet, "/?page=2&page_size=500&u="+id.String()+"&v=nope", nil)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	if got, err := UUIDParam(r, "id"); err != nil || got != id {
		t.Fatalf("UUIDParam = %v, %v", got, err)
	}
	if _, err := UUIDParam(r, "ruim"); err == nil {
		t.Fatal("UUID inválido aceito")
	}
	if got, err := OptionalUUIDQuery(r, "u"); err != nil || *got != id {
		t.Fatalf("OptionalUUIDQuery = %v, %v", got, err)
	}
	if got, err := OptionalUUIDQuery(r, "ausente"); err != nil || got != nil {
		t.Fatalf("ausente = %v, %v", got, err)
	}
	if _, err := OptionalUUIDQuery(r, "v"); err == nil {
		t.Fatal("UUID de query inválido aceito")
	}
	if p := Page(r, 100); p.Page != 2 || p.PageSize != 100 {
		t.Fatalf("Page = %+v", p)
	}

	rec := httptest.NewRecorder()
	WritePage(rec, []int{1}, Page(r, 100), 1)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"meta"`) {
		t.Fatalf("WritePage = %d %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	WriteCreated(rec, map[string]string{"id": "1"})
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"id":"1"`) {
		t.Fatalf("WriteCreated = %d %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	WriteNoContent(rec)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("WriteNoContent = %d", rec.Code)
	}
}

func TestBind(t *testing.T) {
	type in struct {
		Nome string `json:"nome" validate:"required"`
	}
	var dst in
	rec := httptest.NewRecorder()
	if err := Bind(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"nome":"x"}`)), &dst); err != nil || dst.Nome != "x" {
		t.Fatalf("Bind = %v %+v", err, dst)
	}
	var empty in
	if err := Bind(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`)), &empty); err == nil {
		t.Fatal("campo obrigatório ausente aceito")
	}
	if err := Bind(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{`)), &dst); err == nil {
		t.Fatal("JSON quebrado aceito")
	}
}

func TestValidateRejectsNonStruct(t *testing.T) {
	err := Validate("não é struct")
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) || appErr.Status != http.StatusBadRequest {
		t.Fatalf("err = %v", err)
	}
}

func TestProblemDetailsUnknownStatusHasGenericTitle(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.Header.Set("Accept", "application/problem+json")
	rec := httptest.NewRecorder()
	WriteError(rec, r, slog.New(slog.NewTextHandler(io.Discard, nil)), &apperrors.Error{Code: "CUSTOM", Status: 599, Message: "m"})
	var pd ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &pd); err != nil || pd.Title != "Error" || pd.Status != 599 {
		t.Fatalf("problem = %+v, %v", pd, err)
	}
}
