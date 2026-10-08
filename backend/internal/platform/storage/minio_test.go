package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
)

// realMinio devolve um provider contra o MinIO de teste
// (TEST_MINIO_ENDPOINT, TEST_MINIO_ACCESS_KEY, TEST_MINIO_SECRET_KEY) ou
// pula o teste.
func realMinio(t *testing.T) (*MinioProvider, string) {
	t.Helper()
	endpoint := os.Getenv("TEST_MINIO_ENDPOINT")
	if endpoint == "" {
		t.Skip("TEST_MINIO_ENDPOINT não definido")
	}
	p, err := NewMinioProvider(endpoint, os.Getenv("TEST_MINIO_ACCESS_KEY"), os.Getenv("TEST_MINIO_SECRET_KEY"), false)
	if err != nil {
		t.Fatal(err)
	}
	return p, endpoint
}

func bucketName(prefix string) string {
	return prefix + "-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:20]
}

func TestMinioObjectLifecycle(t *testing.T) {
	p, _ := realMinio(t)
	ctx := context.Background()
	bucket := bucketName("nx-obj")

	if err := p.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	if err := p.EnsureBucket(ctx, bucket); err != nil {
		t.Fatal(err)
	}
	if err := p.EnsureBucket(ctx, bucket); err != nil { // idempotente
		t.Fatal(err)
	}

	body := []byte("conteúdo do anexo")
	if err := p.Put(ctx, bucket, "a/b.txt", bytes.NewReader(body), int64(len(body)), "text/plain"); err != nil {
		t.Fatal(err)
	}
	info, err := p.Stat(ctx, bucket, "a/b.txt")
	if err != nil || info.Size != int64(len(body)) || info.ContentType != "text/plain" || info.ETag == "" {
		t.Fatalf("stat = %+v, %v", info, err)
	}
	rc, err := p.Get(ctx, bucket, "a/b.txt")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(rc)
	_ = rc.Close()
	if !bytes.Equal(got, body) {
		t.Fatalf("get = %q", got)
	}

	// presigned PUT funciona de verdade, sem passar pelo backend
	putURL, err := p.PresignedPutURL(ctx, bucket, "direto.bin", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodPut, putURL, strings.NewReader("xyz"))
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("presigned put: %v %v", resp, err)
	}
	_ = resp.Body.Close()
	getURL, err := p.PresignedGetURL(ctx, bucket, "direto.bin", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	resp, err = http.Get(getURL) // #nosec G107 -- URL do MinIO de teste
	if err != nil {
		t.Fatal(err)
	}
	got, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if string(got) != "xyz" {
		t.Fatalf("presigned get = %q", got)
	}

	if err := p.Delete(ctx, bucket, "a/b.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Stat(ctx, bucket, "a/b.txt"); !errors.Is(err, ErrObjectNotFound) {
		t.Fatalf("stat depois do delete: %v", err)
	}
	// Get de objeto inexistente falha na hora, não no meio da leitura
	if _, err := p.Get(ctx, bucket, "a/b.txt"); !errors.Is(err, ErrObjectNotFound) {
		t.Fatalf("get depois do delete: %v", err)
	}
}

func TestMinioPresignUsesPublicEndpoint(t *testing.T) {
	_, endpoint := realMinio(t)
	p, err := NewMinioProviderWithPresign(endpoint, "arquivos.exemplo.gov.br", "k", "s", false, true)
	if err != nil {
		t.Fatal(err)
	}
	// assina offline (região fixa), sem alcançar o host público
	u, err := p.PresignedGetURL(context.Background(), "docs", "o", time.Minute)
	if err != nil || !strings.HasPrefix(u, "https://arquivos.exemplo.gov.br/docs/o?") {
		t.Fatalf("url = %q, %v", u, err)
	}
	u, err = p.PresignedPutURL(context.Background(), "docs", "o", time.Minute)
	if err != nil || !strings.HasPrefix(u, "https://arquivos.exemplo.gov.br/docs/o?") {
		t.Fatalf("url = %q, %v", u, err)
	}
}

func TestMinioImmutableBucket(t *testing.T) {
	p, _ := realMinio(t)
	ctx := context.Background()
	bucket := bucketName("nx-worm")

	if err := p.EnsureImmutableBucket(ctx, bucket, 1); err != nil {
		t.Fatal(err)
	}
	if err := p.EnsureImmutableBucket(ctx, bucket, 2); err != nil { // já existe com lock
		t.Fatal(err)
	}
	body := []byte(`{"dia":"2026-01-01"}`)
	if err := p.PutImmutable(ctx, bucket, "d.json", bytes.NewReader(body), int64(len(body)), "application/json", 1); err != nil {
		t.Fatal(err)
	}
	// Compliance: a versão gravada não pode ser apagada
	info, err := p.client.StatObject(ctx, bucket, "d.json", minioStatOpts())
	if err != nil {
		t.Fatal(err)
	}
	if err := p.client.RemoveObject(ctx, bucket, "d.json", minioRemoveVersion(info.VersionID)); err == nil {
		t.Fatal("objeto em Compliance foi apagado")
	}

	// bucket comum não vira WORM
	plain := bucketName("nx-plain")
	if err := p.EnsureBucket(ctx, plain); err != nil {
		t.Fatal(err)
	}
	if err := p.EnsureImmutableBucket(ctx, plain, 1); err == nil || !strings.Contains(err.Error(), "sem object-lock") {
		t.Fatalf("err = %v", err)
	}
}

func TestMinioRejectsInvalidRetention(t *testing.T) {
	p, err := NewMinioProvider("127.0.0.1:1", "k", "s", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, days := range []int{0, -1} {
		if err := p.EnsureImmutableBucket(context.Background(), "b", days); !errors.Is(err, errInvalidRetention) {
			t.Fatalf("ensure(%d) = %v", days, err)
		}
		if err := p.PutImmutable(context.Background(), "b", "o", strings.NewReader(""), 0, "", days); !errors.Is(err, errInvalidRetention) {
			t.Fatalf("put(%d) = %v", days, err)
		}
	}
}

func TestNewMinioProviderInvalidEndpoints(t *testing.T) {
	if _, err := NewMinioProvider("minio:9000/caminho", "k", "s", false); err == nil {
		t.Fatal("endpoint com caminho aceito")
	}
	if _, err := NewMinioProviderWithPresign("minio:9000", "publico/caminho", "k", "s", false, false); err == nil {
		t.Fatal("endpoint público com caminho aceito")
	}
}

// fakeS3 responde cada requisição via handle (method, query) → status, body.
// 403 não é repetido pelo minio-go, então as falhas saem na hora.
func fakeS3(t *testing.T, handle func(r *http.Request) (int, string)) *MinioProvider {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status, body := handle(r)
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	p, err := NewMinioProvider(strings.TrimPrefix(srv.URL, "http://"), "k", "s", false)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

const accessDenied = `<?xml version="1.0" encoding="UTF-8"?><Error><Code>AccessDenied</Code><Message>negado</Message></Error>`

func deny(*http.Request) (int, string) { return http.StatusForbidden, accessDenied }

func TestMinioFailures(t *testing.T) {
	ctx := context.Background()
	p := fakeS3(t, deny)

	check := func(name string, err error, want string) {
		t.Helper()
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v (quer %q)", name, err, want)
		}
	}
	check("ping", p.Ping(ctx), "ping minio")
	check("ensure", p.EnsureBucket(ctx, "bucket"), "check bucket exists")
	check("ensure worm", p.EnsureImmutableBucket(ctx, "bucket", 1), "check worm bucket")
	check("put", p.Put(ctx, "bucket", "o", strings.NewReader("x"), 1, "text/plain"), "put object")
	check("put worm", p.PutImmutable(ctx, "bucket", "o", strings.NewReader("x"), 1, "text/plain", 1), "put immutable object")
	check("delete", p.Delete(ctx, "bucket", "o"), "delete object")
	_, err := p.Get(ctx, "bucket", "o")
	check("get", err, "storage: get bucket/o")
	_, err = p.Get(ctx, "Nome Inválido", "o")
	check("get nome inválido", err, "storage: get object")
	_, err = p.Stat(ctx, "bucket", "o")
	check("stat", err, "storage: stat bucket/o")
	_, err = p.PresignedPutURL(ctx, "Nome Inválido", "o", time.Minute)
	check("presign put", err, "presigned put")
	_, err = p.PresignedGetURL(ctx, "Nome Inválido", "o", time.Minute)
	check("presign get", err, "presigned get")
}

func TestMinioBucketCreationFailures(t *testing.T) {
	ctx := context.Background()
	// bucket não existe (HEAD 404) e a criação é negada
	p := fakeS3(t, func(r *http.Request) (int, string) {
		if r.Method == http.MethodHead {
			return http.StatusNotFound, ""
		}
		return deny(r)
	})
	if err := p.EnsureBucket(ctx, "bucket"); err == nil || !strings.Contains(err.Error(), "create bucket") {
		t.Fatalf("ensure = %v", err)
	}
	if err := p.EnsureImmutableBucket(ctx, "bucket", 1); err == nil || !strings.Contains(err.Error(), "create worm bucket") {
		t.Fatalf("ensure worm = %v", err)
	}

	// bucket existe com lock, mas gravar a retenção é negado
	lock := `<?xml version="1.0" encoding="UTF-8"?><ObjectLockConfiguration><ObjectLockEnabled>Enabled</ObjectLockEnabled></ObjectLockConfiguration>`
	p = fakeS3(t, func(r *http.Request) (int, string) {
		switch {
		case r.Method == http.MethodHead:
			return http.StatusOK, ""
		case r.Method == http.MethodGet && r.URL.Query().Has("object-lock"):
			return http.StatusOK, lock
		}
		return deny(r)
	})
	if err := p.EnsureImmutableBucket(ctx, "bucket", 1); err == nil || !strings.Contains(err.Error(), "set worm retention") {
		t.Fatalf("retention = %v", err)
	}
}

func TestMapObjectErr(t *testing.T) {
	err := mapObjectErr("stat", "b", "o", fmt.Errorf("rede"))
	if err.Error() != "storage: stat b/o: rede" {
		t.Fatal(err)
	}
}

func minioStatOpts() minio.StatObjectOptions { return minio.StatObjectOptions{} }

func minioRemoveVersion(v string) minio.RemoveObjectOptions {
	return minio.RemoveObjectOptions{VersionID: v}
}
