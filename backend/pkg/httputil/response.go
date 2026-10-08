// Package httputil fornece o envelope de resposta JSON padrão, a
// decodificação de requisição e os helpers de validação compartilhados
// por todo handler de transporte HTTP da plataforma. Handlers devem usar
// estes helpers em vez de escrever direto em http.ResponseWriter, para que
// toda resposta — sucesso ou erro — tenha o mesmo formato {data, error,
// meta} e nenhum handler acabe vazando detalhe de erro interno para o
// cliente.
package httputil

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/platform/logging"
)

// Envelope é o formato de resposta padrão de todo endpoint do Projeto Nexus.
type Envelope struct {
	Data  any        `json:"data"`
	Error *ErrorBody `json:"error"`
	Meta  any        `json:"meta,omitempty"`
}

// ErrorBody é o formato de erro padrão. Nunca contém stack trace ou
// detalhe de erro interno — só um código estável e uma mensagem segura de
// mostrar ao cliente.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WriteJSON escreve um envelope de sucesso com o status HTTP informado.
func WriteJSON(w http.ResponseWriter, status int, data any, meta any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Envelope{Data: data, Error: nil, Meta: meta})
}

// WriteOK escreve um envelope 200 OK.
func WriteOK(w http.ResponseWriter, data any) {
	WriteJSON(w, http.StatusOK, data, nil)
}

// WriteOKWithMeta escreve um envelope 200 OK incluindo metadados de
// paginação/outros.
func WriteOKWithMeta(w http.ResponseWriter, data any, meta any) {
	WriteJSON(w, http.StatusOK, data, meta)
}

// WriteCreated escreve um envelope 201 Created.
func WriteCreated(w http.ResponseWriter, data any) {
	WriteJSON(w, http.StatusCreated, data, nil)
}

// WriteAccepted escreve um envelope 202 Accepted — usado pelos endpoints
// de criação de job assíncrono, como o gatilho de teste de conectividade
// de uma integração.
func WriteAccepted(w http.ResponseWriter, data any) {
	WriteJSON(w, http.StatusAccepted, data, nil)
}

// WriteError escreve o envelope de erro padrão para err. Se err for (ou
// envolver) um *apperrors.Error, seu Status/Code/Message são usados tal
// qual; caso contrário, é tratado como um erro interno inesperado, logado
// com todo o detalhe no lado do servidor, e reportado ao cliente como um
// 500 genérico — o cliente nunca vê o erro bruto original nesse caso.
//
// Gap G-13 / RFC 7807 (ADR 006, opção B — negociação de conteúdo, sem
// quebra): um cliente que envia "Accept: application/problem+json" recebe
// o corpo no formato Problem Details (type/title/status/detail/instance +
// a extensão "code" legível por máquina). Qualquer outro Accept mantém o
// envelope {data,error} histórico. Sucesso não muda (o RFC 7807 só trata
// de erro).
func WriteError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, err error) {
	appErr, ok := apperrors.As(err)
	if !ok {
		appErr = apperrors.Internal(err)
	}

	log := logging.FromContext(r.Context(), logger)
	if appErr.Status >= 500 {
		log.Error("request failed", slog.String("code", string(appErr.Code)), slog.Any("error", appErr.Err))
	} else {
		log.Warn("request rejected", slog.String("code", string(appErr.Code)), slog.String("message", appErr.Message))
	}

	if wantsProblemJSON(r) {
		writeProblemDetails(w, r, appErr)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(appErr.Status)
	_ = json.NewEncoder(w).Encode(Envelope{
		Data: nil,
		Error: &ErrorBody{
			Code:    string(appErr.Code),
			Message: appErr.Message,
		},
	})
}

// ProblemDetails é o corpo RFC 7807. "type" é uma URN estável por código
// de erro (independente de domínio); "code" e "request_id" são membros de
// extensão do RFC — "code" preserva o identificador legível por máquina
// que o envelope histórico já expunha.
type ProblemDetails struct {
	Type      string `json:"type"`
	Title     string `json:"title"`
	Status    int    `json:"status"`
	Detail    string `json:"detail"`
	Instance  string `json:"instance,omitempty"`
	Code      string `json:"code"`
	RequestID string `json:"request_id,omitempty"`
}

func wantsProblemJSON(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "application/problem+json")
}

func writeProblemDetails(w http.ResponseWriter, r *http.Request, appErr *apperrors.Error) {
	title := http.StatusText(appErr.Status)
	if title == "" {
		title = "Error"
	}
	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	w.WriteHeader(appErr.Status)
	_ = json.NewEncoder(w).Encode(ProblemDetails{
		Type:      "urn:nexus:error:" + strings.ToLower(string(appErr.Code)),
		Title:     title,
		Status:    appErr.Status,
		Detail:    appErr.Message,
		Instance:  r.URL.Path,
		Code:      string(appErr.Code),
		RequestID: logging.RequestID(r.Context()),
	})
}
