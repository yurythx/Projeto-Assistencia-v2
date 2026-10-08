package modkit

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"
	"github.com/yurythx/projeto-nexus/internal/platform/storage/storagetest"
)

func TestSlugifyAndSnippet(t *testing.T) {
	if got := Slugify("Relatório Anual 2026!"); got != "relatorio-anual-2026" {
		t.Fatalf("slug: %q", got)
	}
	long := Slugify(strings.Repeat("palavra ", 30))
	if len(long) > 80 || strings.HasSuffix(long, "-") {
		t.Fatalf("slug limitado a 80 sem hífen no fim: %q", long)
	}
	if got := Snippet("  um   texto  ", 50); got != "um texto" {
		t.Fatalf("espaços normalizados: %q", got)
	}
	if got := Snippet("ação completa", 4); got != "ação…" {
		t.Fatalf("corte por caractere (não byte): %q", got)
	}
}

func status(err error) int {
	if ae, ok := apperrors.As(err); ok {
		return ae.Status
	}
	return 0
}

func TestUploadTicketAndConfirmation(t *testing.T) {
	ctx := context.Background()
	st := storagetest.New()
	allowed := []string{"image/png"}
	if _, err := NewUpload(ctx, st, "b", "blog/covers", "x.png", "text/html", allowed, time.Minute); status(err) != 422 {
		t.Fatalf("tipo proibido: %v", err)
	}
	tk, err := NewUpload(ctx, st, "b", "/blog/covers/", "../../etc/passwd", "IMAGE/PNG; charset=x", allowed, time.Minute)
	if err != nil || !strings.HasPrefix(tk.ObjectKey, "blog/covers/") || !strings.HasSuffix(tk.ObjectKey, ".bin") || strings.Contains(tk.ObjectKey, "..") {
		t.Fatalf("nome original nunca vira caminho: %+v %v", tk, err)
	}
	st.FailPresign = true
	if _, err := NewUpload(ctx, st, "b", "p", "a.png", "image/png", allowed, time.Minute); status(err) != 503 {
		t.Fatalf("armazenamento fora: %v", err)
	}
	st.FailPresign = false

	for _, key := range []string{"outro/x.png", "blog/covers/../x.png"} {
		if _, err := ConfirmUpload(ctx, st, "b", key, "blog/covers", 1024, allowed); status(err) != 422 {
			t.Errorf("chave %q: %v", key, err)
		}
	}
	if _, err := ConfirmUpload(ctx, st, "b", "blog/covers/nao-enviado.png", "blog/covers", 1024, allowed); status(err) != 422 {
		t.Fatalf("ainda não enviado: %v", err)
	}
	put := func(key, ct string, n int) {
		if err := st.Put(ctx, "b", key, bytes.NewReader(make([]byte, n)), int64(n), ct); err != nil {
			t.Fatal(err)
		}
	}
	put("blog/covers/ok.png", "image/png", 10)
	if info, err := ConfirmUpload(ctx, st, "b", "blog/covers/ok.png", "blog/covers", 1024, allowed); err != nil || info.Size != 10 {
		t.Fatalf("confirmado: %+v %v", info, err)
	}
	for _, c := range []struct {
		ct string
		n  int
	}{{"text/html", 10}, {"image/png", 2048}, {"image/png", 0}} {
		key := "blog/covers/recusado.png"
		put(key, c.ct, c.n)
		if _, err := ConfirmUpload(ctx, st, "b", key, "blog/covers", 1024, allowed); status(err) != 422 || st.Has("b", key) {
			t.Errorf("%s %d bytes: recusado e apagado (%v)", c.ct, c.n, err)
		}
	}
	st.FailStat = true
	if _, err := ConfirmUpload(ctx, st, "b", "blog/covers/ok.png", "blog/covers", 1024, allowed); status(err) != 503 || errors.Is(err, context.Canceled) {
		t.Fatalf("armazenamento fora: %v", err)
	}
}
