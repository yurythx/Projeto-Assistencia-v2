package telemetry

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func TestSetupWithoutEndpointIsNoop(t *testing.T) {
	shutdown := Setup(context.Background(), "nexus", "test", "", quiet)
	if err := shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	// o propagador é configurado mesmo sem exporter
	fields := otel.GetTextMapPropagator().Fields()
	if !strings.Contains(strings.Join(fields, ","), "traceparent") {
		t.Fatalf("propagador sem traceparent: %v", fields)
	}
}

func TestSetupExportsSpansToCollector(t *testing.T) {
	var hits atomic.Int32
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/traces" {
			hits.Add(1)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer collector.Close()

	prev := otel.GetTracerProvider()
	defer otel.SetTracerProvider(prev)

	shutdown := Setup(context.Background(), "nexus", "test", strings.TrimPrefix(collector.URL, "http://"), quiet)
	if _, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider); !ok {
		t.Fatalf("provider global não é o do SDK: %T", otel.GetTracerProvider())
	}
	_, span := otel.Tracer("t").Start(context.Background(), "op")
	span.End()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := shutdown(ctx); err != nil { // esvazia o batcher
		t.Fatal(err)
	}
	if hits.Load() == 0 {
		t.Fatal("nenhum span chegou ao collector")
	}
}

func TestSetupExporterFailureKeepsNoop(t *testing.T) {
	orig := newExporter
	defer func() { newExporter = orig }()
	newExporter = func(context.Context, string) (sdktrace.SpanExporter, error) { return nil, errors.New("boom") }

	prev := otel.GetTracerProvider()
	shutdown := Setup(context.Background(), "nexus", "test", "collector:4318", quiet)
	if err := shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if otel.GetTracerProvider() != prev {
		t.Fatal("provider trocado mesmo sem exporter")
	}
}
