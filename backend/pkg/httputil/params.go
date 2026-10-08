package httputil

import (
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/domain/pagination"
)

// UUIDParam lê um parâmetro de rota chi como UUID, devolvendo um erro 400
// pronto para WriteError.
func UUIDParam(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		return uuid.Nil, apperrors.BadRequest(name + " inválido")
	}
	return id, nil
}

// OptionalUUIDQuery lê um parâmetro de query opcional como UUID.
func OptionalUUIDQuery(r *http.Request, name string) (*uuid.UUID, error) {
	v := strings.TrimSpace(r.URL.Query().Get(name))
	if v == "" {
		return nil, nil
	}
	id, err := uuid.Parse(v)
	if err != nil {
		return nil, apperrors.BadRequest(name + " inválido")
	}
	return &id, nil
}

// Page lê page/page_size da query aplicando o teto maxPageSize.
func Page(r *http.Request, maxPageSize int) pagination.Params {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	size, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	return pagination.New(page, size, maxPageSize)
}

// Query devolve um parâmetro de query sem espaços nas pontas, limitado a
// max caracteres (defesa contra termos de busca gigantes). O corte é por
// caractere, não por byte: cortar no meio de um "ç" geraria UTF-8
// inválido, que o Postgres rejeita (500 em vez de uma busca).
func Query(r *http.Request, name string, max int) string {
	v := strings.TrimSpace(r.URL.Query().Get(name))
	if utf8.RuneCountInString(v) > max {
		v = string([]rune(v)[:max])
	}
	return strings.ToValidUTF8(v, "")
}

// WritePage escreve uma listagem paginada padrão.
func WritePage(w http.ResponseWriter, items any, p pagination.Params, total int64) {
	WriteOKWithMeta(w, items, pagination.NewMeta(p, total))
}

// WriteNoContent escreve 204.
func WriteNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// Bind decodifica e valida o corpo JSON em dst.
func Bind(w http.ResponseWriter, r *http.Request, dst any) error {
	if err := DecodeJSON(w, r, dst); err != nil {
		return err
	}
	return Validate(dst)
}
