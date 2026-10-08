package outbox

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// testPool/truncateOutbox vêm de outbox_test.go, mesmo pacote.

func TestStats_Get_CountsByStatus(t *testing.T) {
	pool := testPool(t)
	truncateOutbox(t, pool)
	stats := NewStats(pool)
	ctx := context.Background()

	insertRow := func(status string) {
		_, err := pool.Exec(ctx, `
			INSERT INTO outbox_events (event_type, aggregate_type, aggregate_id, payload, status)
			VALUES ($1, 'test_aggregate', $2, '{}'::jsonb, $3)
		`, "test.stats."+status, uuid.NewString(), status)
		if err != nil {
			t.Fatalf("insert outbox row (status=%s): %v", status, err)
		}
	}

	insertRow("pending")
	insertRow("pending")
	insertRow("published")
	insertRow("failed")

	got, err := stats.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	want := Counts{Pending: 2, Published: 1, Failed: 1}
	if got != want {
		t.Errorf("Get() = %+v, want %+v", got, want)
	}
}

func TestStats_Get_EmptyTableIsAllZero(t *testing.T) {
	pool := testPool(t)
	truncateOutbox(t, pool)
	stats := NewStats(pool)

	got, err := stats.Get(context.Background())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != (Counts{}) {
		t.Errorf("Get() numa tabela vazia deveria ser tudo zero, got %+v", got)
	}
}
