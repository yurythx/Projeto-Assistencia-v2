package metrics

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

func gauge(t *testing.T, state string) float64 {
	t.Helper()
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, mf := range mfs {
		if mf.GetName() != "nexus_postgres_connections" {
			continue
		}
		for _, m := range mf.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetName() == "state" && l.GetValue() == state {
					return m.GetGauge().GetValue()
				}
			}
		}
	}
	t.Fatalf("gauge state=%s ausente", state)
	return 0
}

func TestPostgresPoolMetricsFollowTheCurrentPool(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL não definido")
	}
	newPool := func(max string) *pgxpool.Pool {
		p, err := pgxpool.New(context.Background(), dsn+"&pool_max_conns="+max)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(p.Close)
		return p
	}
	first := newPool("3")
	RegisterPostgresPoolMetrics(first)
	conn, err := first.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if gauge(t, "max") != 3 || gauge(t, "acquired") != 1 {
		t.Fatalf("max=%v acquired=%v", gauge(t, "max"), gauge(t, "acquired"))
	}
	conn.Release()
	if gauge(t, "idle") != 1 || gauge(t, "acquired") != 0 {
		t.Fatalf("idle=%v acquired=%v", gauge(t, "idle"), gauge(t, "acquired"))
	}

	// segundo registro não entra em pânico e passa a observar o novo pool
	RegisterPostgresPoolMetrics(newPool("7"))
	if gauge(t, "max") != 7 {
		t.Fatalf("max = %v", gauge(t, "max"))
	}
}

func TestMetricVectorsAreRegistered(t *testing.T) {
	for _, c := range []prometheus.Collector{HTTPRequestsTotal, HTTPRequestDuration, RabbitMQPublishedTotal, RabbitMQConsumedTotal,
		RabbitMQFailedTotal, RabbitMQRetryTotal, RabbitMQDLQTotal, WebSocketConnections, WebSocketErrorsTotal, IdempotencyOutcomesTotal} {
		if err := prometheus.Register(c); err == nil {
			t.Fatalf("%v não estava registrado", c)
		}
	}
}
