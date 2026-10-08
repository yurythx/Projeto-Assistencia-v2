// Package redisx configura o cliente Redis compartilhado do Nexus. O Redis
// sustenta três responsabilidades de plataforma que precisam de estado
// comum entre réplicas: o backplane do WebSocket (pub/sub), o rate
// limiting adaptativo / lockout progressivo (A07) e a invalidação do cache
// de permissões do IAM.
package redisx

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Connect abre o cliente a partir de uma URL redis:// ou rediss:// e faz
// um PING inicial (fail fast no boot).
func Connect(ctx context.Context, url string) (*redis.Client, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("redisx: parse REDIS_URL: %w", err)
	}
	opts.DialTimeout = 5 * time.Second
	opts.ReadTimeout = 3 * time.Second
	opts.WriteTimeout = 3 * time.Second

	client := redis.NewClient(opts)
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redisx: ping: %w", err)
	}
	return client, nil
}

// Ping é um Check de readiness compatível com httpserver.Check.
func Ping(client *redis.Client) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		return client.Ping(ctx).Err()
	}
}

// Key monta uma chave namespaced do Nexus ("nexus:<partes>").
func Key(parts ...string) string {
	k := "nexus"
	for _, p := range parts {
		k += ":" + p
	}
	return k
}
