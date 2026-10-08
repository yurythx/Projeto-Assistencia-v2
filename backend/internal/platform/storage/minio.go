package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinioProvider implementa a interface Provider para interagir com o MinIO (ou AWS S3).
type MinioProvider struct {
	client        *minio.Client // ops server-side (rede interna)
	presignClient *minio.Client // assina URLs para o host alcançável pelo navegador
}

// NewMinioProvider constrói o cliente do servidor MinIO. As URLs
// pré-assinadas são assinadas para o MESMO host — use
// NewMinioProviderWithPresign quando o navegador acessa o MinIO por outro
// endereço (o caso do Docker: backend fala com "minio:9000", navegador com
// "localhost:9000").
func NewMinioProvider(endpoint, accessKey, secretKey string, useSSL bool) (*MinioProvider, error) {
	return NewMinioProviderWithPresign(endpoint, endpoint, accessKey, secretKey, useSSL, useSSL)
}

// minioRegion fixa a região usada na assinatura V4. O MinIO responde como
// "us-east-1" por padrão. Passar Region explicitamente é essencial no
// presignClient: sem ela, o minio-go dispara um GetBucketLocation ao vivo
// contra o publicEndpoint na hora de assinar a URL — e esse host
// (ex.: localhost:9000) não é alcançável de DENTRO do container, então a
// assinatura falhava com "connection refused". Com a região conhecida o
// minio-go assina offline, sem nenhuma chamada de rede.
const minioRegion = "us-east-1"

// NewMinioProviderWithPresign separa o endpoint interno (Get/Put/Delete) do
// endpoint público usado só para PresignedPutURL/PresignedGetURL.
func NewMinioProviderWithPresign(endpoint, publicEndpoint, accessKey, secretKey string, useSSL, publicUseSSL bool) (*MinioProvider, error) {
	creds := credentials.NewStaticV4(accessKey, secretKey, "")
	client, err := minio.New(endpoint, &minio.Options{Creds: creds, Secure: useSSL, Region: minioRegion})
	if err != nil {
		return nil, fmt.Errorf("storage: failed to initialize minio client: %w", err)
	}
	presignClient := client
	if publicEndpoint != "" && publicEndpoint != endpoint {
		presignClient, err = minio.New(publicEndpoint, &minio.Options{Creds: creds, Secure: publicUseSSL, Region: minioRegion})
		if err != nil {
			return nil, fmt.Errorf("storage: failed to initialize minio presign client: %w", err)
		}
	}
	return &MinioProvider{client: client, presignClient: presignClient}, nil
}

// Ping verifica se o MinIO está alcançável e as credenciais configuradas
// são válidas — usado pelo /ready (ver internal/app/router.go) e, por
// tabela, pelo painel de Monitoramento do frontend (GET /api/health), que
// antes não tinha NENHUMA forma de checar este serviço (achado de
// auditoria: o card do MinIO sempre mostrava "Desconhecido", já que nada
// nunca perguntava ao MinIO se ele estava de pé). ListBuckets é a
// operação mais barata que exige tanto conectividade quanto uma
// autenticação bem-sucedida — não depende de conhecer o nome de nenhum
// bucket específico, ao contrário de BucketExists.
func (p *MinioProvider) Ping(ctx context.Context) error {
	if _, err := p.client.ListBuckets(ctx); err != nil {
		return fmt.Errorf("storage: ping minio: %w", err)
	}
	return nil
}

// EnsureBucket verifica se um bucket existe e cria caso não exista.
func (p *MinioProvider) EnsureBucket(ctx context.Context, bucketName string) error {
	exists, err := p.client.BucketExists(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("storage: check bucket exists: %w", err)
	}
	if !exists {
		err = p.client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
		if err != nil {
			return fmt.Errorf("storage: create bucket: %w", err)
		}
	}
	return nil
}

var _ WORMWriter = (*MinioProvider)(nil)

// errInvalidRetention: retenção zero ou negativa não protege nada (e um
// int negativo viraria um uint enorme na configuração do bucket).
var errInvalidRetention = errors.New("storage: retenção WORM precisa ser de pelo menos 1 dia")

// EnsureImmutableBucket cria bucket COM object-lock (se ainda não existe) e
// define uma retenção padrão em modo Compliance de retentionDays dias.
// Object-lock só pode ser habilitado na criação do bucket — um bucket
// pré-existente sem lock não pode ser convertido, então retornamos erro
// nesse caso para o operador criar o bucket dedicado correto.
func (p *MinioProvider) EnsureImmutableBucket(ctx context.Context, bucket string, retentionDays int) error {
	if retentionDays <= 0 {
		return errInvalidRetention
	}
	exists, err := p.client.BucketExists(ctx, bucket)
	if err != nil {
		return fmt.Errorf("storage: check worm bucket: %w", err)
	}
	if !exists {
		if err := p.client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{ObjectLocking: true}); err != nil {
			return fmt.Errorf("storage: create worm bucket: %w", err)
		}
	} else {
		if _, _, _, _, lerr := p.client.GetObjectLockConfig(ctx, bucket); lerr != nil {
			return fmt.Errorf("storage: bucket %q existe sem object-lock — crie um bucket dedicado com lock para a cópia WORM: %w", bucket, lerr)
		}
	}

	mode := minio.Compliance
	validity := uint(retentionDays) // #nosec G115 -- positivo, conferido acima
	unit := minio.Days
	if err := p.client.SetObjectLockConfig(ctx, bucket, &mode, &validity, &unit); err != nil {
		return fmt.Errorf("storage: set worm retention: %w", err)
	}
	return nil
}

// PutImmutable grava um objeto com retenção Compliance até now+retentionDays.
func (p *MinioProvider) PutImmutable(ctx context.Context, bucket, object string, reader io.Reader, size int64, contentType string, retentionDays int) error {
	if retentionDays <= 0 {
		return errInvalidRetention
	}
	_, err := p.client.PutObject(ctx, bucket, object, reader, size, minio.PutObjectOptions{
		ContentType:     contentType,
		Mode:            minio.Compliance,
		RetainUntilDate: time.Now().UTC().AddDate(0, 0, retentionDays),
	})
	if err != nil {
		return fmt.Errorf("storage: put immutable object: %w", err)
	}
	return nil
}

// Put salva um objeto (arquivo) no bucket.
func (p *MinioProvider) Put(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, contentType string) error {
	_, err := p.client.PutObject(ctx, bucketName, objectName, reader, objectSize, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("storage: put object: %w", err)
	}
	return nil
}

// Get retorna um leitor para um objeto (ErrObjectNotFound se não existir).
// O GetObject do minio-go é preguiçoso: sem o Stat, um objeto inexistente
// só apareceria como erro no meio da leitura. O Stat dispara a própria
// requisição GET (a resposta fica guardada para as leituras), então não
// custa uma ida extra ao servidor.
func (p *MinioProvider) Get(ctx context.Context, bucketName, objectName string) (io.ReadCloser, error) {
	obj, err := p.client.GetObject(ctx, bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("storage: get object: %w", err)
	}
	if _, err := obj.Stat(); err != nil {
		_ = obj.Close()
		return nil, mapObjectErr("get", bucketName, objectName, err)
	}
	return obj, nil
}

// mapObjectErr traduz "objeto não existe" para ErrObjectNotFound.
func mapObjectErr(op, bucket, object string, err error) error {
	if minio.ToErrorResponse(err).Code == "NoSuchKey" {
		return ErrObjectNotFound
	}
	return fmt.Errorf("storage: %s %s/%s: %w", op, bucket, object, err)
}

// Delete remove um objeto do bucket.
func (p *MinioProvider) Delete(ctx context.Context, bucketName, objectName string) error {
	err := p.client.RemoveObject(ctx, bucketName, objectName, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("storage: delete object: %w", err)
	}
	return nil
}

// PresignedPutURL gera uma URL segura temporária para que o cliente consiga fazer o upload via PUT sem passar pelo backend.
func (p *MinioProvider) PresignedPutURL(ctx context.Context, bucketName, objectName string, expiry time.Duration) (string, error) {
	url, err := p.presignClient.PresignedPutObject(ctx, bucketName, objectName, expiry)
	if err != nil {
		return "", fmt.Errorf("storage: generate presigned put: %w", err)
	}
	return url.String(), nil
}

// PresignedGetURL gera uma URL temporária para leitura.
func (p *MinioProvider) PresignedGetURL(ctx context.Context, bucketName, objectName string, expiry time.Duration) (string, error) {
	url, err := p.presignClient.PresignedGetObject(ctx, bucketName, objectName, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("storage: generate presigned get: %w", err)
	}
	return url.String(), nil
}

// Stat implementa Provider.
func (p *MinioProvider) Stat(ctx context.Context, bucketName, objectName string) (ObjectInfo, error) {
	info, err := p.client.StatObject(ctx, bucketName, objectName, minio.StatObjectOptions{})
	if err != nil {
		return ObjectInfo{}, mapObjectErr("stat", bucketName, objectName, err)
	}
	return ObjectInfo{Size: info.Size, ContentType: info.ContentType, ETag: info.ETag}, nil
}
