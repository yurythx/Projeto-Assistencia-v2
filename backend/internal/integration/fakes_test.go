package integration

import (
	"bytes"
	"context"
	"io"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/yurythx/projeto-nexus/internal/platform/storage"
)

// memStorage é um storage.Provider em memória para os testes.
type memStorage struct {
	mu      sync.Mutex
	objects map[string]memObject
}

type memObject struct {
	data        []byte
	contentType string
}

func newMemStorage() *memStorage { return &memStorage{objects: map[string]memObject{}} }

func (m *memStorage) Ping(context.Context) error { return nil }

func (m *memStorage) Put(_ context.Context, bucket, object string, r io.Reader, _ int64, ct string) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.objects[bucket+"/"+object] = memObject{data: b, contentType: ct}
	m.mu.Unlock()
	return nil
}

func (m *memStorage) Get(_ context.Context, bucket, object string) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.objects[bucket+"/"+object]
	if !ok {
		return nil, storage.ErrObjectNotFound
	}
	return io.NopCloser(bytes.NewReader(o.data)), nil
}

func (m *memStorage) Delete(_ context.Context, bucket, object string) error {
	m.mu.Lock()
	delete(m.objects, bucket+"/"+object)
	m.mu.Unlock()
	return nil
}

func (m *memStorage) PresignedPutURL(_ context.Context, bucket, object string, _ time.Duration) (string, error) {
	return "https://minio.test/" + bucket + "/" + object + "?put", nil
}

func (m *memStorage) PresignedGetURL(_ context.Context, bucket, object string, _ time.Duration) (string, error) {
	return "https://minio.test/" + bucket + "/" + object + "?get", nil
}

func (m *memStorage) Stat(_ context.Context, bucket, object string) (storage.ObjectInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.objects[bucket+"/"+object]
	if !ok {
		return storage.ObjectInfo{}, storage.ErrObjectNotFound
	}
	return storage.ObjectInfo{Size: int64(len(o.data)), ContentType: o.contentType}, nil
}

// okReauth aceita a senha "correta" e recusa qualquer outra.
type okReauth struct{}

func (okReauth) Reauthenticate(_ context.Context, _ uuid.UUID, _ string, _ bool, password string) (string, error) {
	if password != "correta" {
		return "", errReauth
	}
	return "password_reauth:test", nil
}
