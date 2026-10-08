package storage

import (
	"context"
	"errors"
	"io"
	"time"
)

// WORMWriter é implementado por providers que suportam retenção imutável
// por objeto (S3 Object Lock). Usado pela cópia WORM da trilha de
// auditoria (F2.6) — se o provider não implementar, o exportador cai
// para Put comum e registra o aviso.
type WORMWriter interface {
	// EnsureImmutableBucket garante que bucket existe COM object-lock
	// habilitado e uma retenção padrão em modo Compliance de
	// retentionDays dias. Retorna erro se o bucket já existe sem
	// object-lock (não dá para retrofitar).
	EnsureImmutableBucket(ctx context.Context, bucket string, retentionDays int) error
	// PutImmutable grava object com retenção Compliance até now + retentionDays.
	PutImmutable(ctx context.Context, bucket, object string, reader io.Reader, size int64, contentType string, retentionDays int) error
}

// Provider define a interface de armazenamento de blobs/objetos (ex: PDFs).
type Provider interface {
	// Ping verifica conectividade + credenciais válidas contra o storage —
	// usado pelo /ready (§ Monitoramento).
	Ping(ctx context.Context) error

	// Put salva um objeto (arquivo) no storage.
	Put(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, contentType string) error

	// Get retorna o stream de leitura de um objeto salvo.
	Get(ctx context.Context, bucketName, objectName string) (io.ReadCloser, error)

	// Delete remove um objeto do storage.
	Delete(ctx context.Context, bucketName, objectName string) error

	// PresignedPutURL gera uma URL temporária para o frontend fazer upload diretamente.
	PresignedPutURL(ctx context.Context, bucketName, objectName string, expiry time.Duration) (string, error)

	// PresignedGetURL gera uma URL temporária para o frontend visualizar/fazer download diretamente.
	PresignedGetURL(ctx context.Context, bucketName, objectName string, expiry time.Duration) (string, error)

	// Stat devolve metadados do objeto (ErrObjectNotFound se não existir) —
	// usado para confirmar um upload direto: tamanho e tipo são conferidos
	// DEPOIS do PUT, já que a URL pré-assinada não limita o corpo.
	Stat(ctx context.Context, bucketName, objectName string) (ObjectInfo, error)
}

// ObjectInfo são os metadados de um objeto armazenado.
type ObjectInfo struct {
	Size        int64
	ContentType string
	ETag        string
}

// ErrObjectNotFound indica objeto inexistente.
var ErrObjectNotFound = errors.New("storage: objeto não encontrado")
