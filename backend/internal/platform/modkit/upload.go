package modkit

import (
	"context"
	"errors"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/platform/storage"
)

// Tipos de conteúdo aceitos por categoria de upload.
var (
	ImageTypes    = []string{"image/png", "image/jpeg", "image/webp", "image/gif"}
	DocumentTypes = []string{
		"application/pdf", "text/plain", "text/csv", "text/markdown",
		"application/msword", "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.ms-excel", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"application/vnd.ms-powerpoint", "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"application/vnd.oasis.opendocument.text", "application/vnd.oasis.opendocument.spreadsheet",
		"application/zip", "image/png", "image/jpeg", "image/webp", "image/gif",
	}
)

var extPattern = regexp.MustCompile(`^[a-z0-9]{1,8}$`)

// UploadTicket autoriza o navegador a enviar um arquivo DIRETO ao MinIO
// (PUT na URL pré-assinada) — o corpo nunca atravessa a API, o que mantém
// os timeouts curtos do servidor HTTP (anti-Slowloris) compatíveis com
// arquivos grandes.
type UploadTicket struct {
	ObjectKey string            `json:"object_key"`
	UploadURL string            `json:"upload_url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt time.Time         `json:"expires_at"`
}

// NewUpload gera a chave do objeto (prefixo/AAAA/MM/uuid.ext — o nome
// original nunca vira caminho, sem path traversal) e a URL pré-assinada.
func NewUpload(ctx context.Context, store storage.Provider, bucket, prefix, filename, contentType string, allowed []string, expiry time.Duration) (UploadTicket, error) {
	contentType = strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	if !slices.Contains(allowed, contentType) {
		return UploadTicket{}, apperrors.Validation(fmt.Sprintf("tipo de arquivo não permitido: %q", contentType))
	}
	ext := strings.TrimPrefix(strings.ToLower(path.Ext(filename)), ".")
	if !extPattern.MatchString(ext) {
		ext = "bin"
	}
	now := time.Now().UTC()
	key := fmt.Sprintf("%s/%04d/%02d/%s.%s", strings.Trim(prefix, "/"), now.Year(), now.Month(), uuid.NewString(), ext)
	url, err := store.PresignedPutURL(ctx, bucket, key, expiry)
	if err != nil {
		return UploadTicket{}, apperrors.DependencyUnavailable("armazenamento indisponível").WithCause(err)
	}
	return UploadTicket{
		ObjectKey: key, UploadURL: url, Method: "PUT",
		Headers: map[string]string{"Content-Type": contentType}, ExpiresAt: now.Add(expiry),
	}, nil
}

// ConfirmUpload confere que o objeto existe, respeita o tamanho máximo e
// o tipo permitido. Um objeto fora da política é removido.
func ConfirmUpload(ctx context.Context, store storage.Provider, bucket, key, expectedPrefix string, maxBytes int64, allowed []string) (storage.ObjectInfo, error) {
	if !strings.HasPrefix(key, strings.Trim(expectedPrefix, "/")+"/") || strings.Contains(key, "..") {
		return storage.ObjectInfo{}, apperrors.Validation("chave de objeto inválida para este módulo")
	}
	info, err := store.Stat(ctx, bucket, key)
	if errors.Is(err, storage.ErrObjectNotFound) {
		return storage.ObjectInfo{}, apperrors.Validation("o arquivo ainda não foi enviado")
	}
	if err != nil {
		return storage.ObjectInfo{}, apperrors.DependencyUnavailable("armazenamento indisponível").WithCause(err)
	}
	ct := strings.ToLower(strings.SplitN(info.ContentType, ";", 2)[0])
	if info.Size > maxBytes || info.Size == 0 || !slices.Contains(allowed, ct) {
		_ = store.Delete(ctx, bucket, key)
		return storage.ObjectInfo{}, apperrors.Validation(fmt.Sprintf("arquivo recusado (tamanho máximo %d MB, tipos permitidos)", maxBytes/1024/1024))
	}
	return info, nil
}
