package redisx

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
)

func TestConnectPingAndKey(t *testing.T) {
	ctx := context.Background()
	mr := miniredis.RunT(t)
	c, err := Connect(ctx, "redis://"+mr.Addr()+"/0")
	if err != nil {
		t.Fatal(err)
	}
	if err := Ping(c)(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := Connect(ctx, "http://nao-e-redis"); err == nil {
		t.Fatal("URL inválida")
	}
	addr := mr.Addr()
	mr.Close()
	if _, err := Connect(ctx, "redis://"+addr+"/0"); err == nil {
		t.Fatal("PING inicial falha (fail fast)")
	}
	if Key("ws", "ticket", "x") != "nexus:ws:ticket:x" || Key() != "nexus" {
		t.Fatal("chave namespaced")
	}
}
