// Package telemetry configura o distributed tracing (§51). Se
// OTEL_EXPORTER_OTLP_ENDPOINT não estiver definido, Setup deixa o tracer
// provider no-op padrão do OpenTelemetry no lugar — a plataforma nunca
// exige que um collector esteja rodando (nenhum está listado entre os
// serviços do docker-compose no §5) — ela só exporta traces quando um
// endpoint é configurado.
package telemetry

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.42.0"
)

// Shutdown esvazia os buffers e para o tracer provider. Seguro de chamar
// mesmo quando o tracing nunca foi habilitado (nesse caso, é um no-op).
type Shutdown func(context.Context) error

// newExporter cria o exporter OTLP/HTTP (variável para os testes simularem
// a falha).
var newExporter = func(ctx context.Context, endpoint string) (sdktrace.SpanExporter, error) {
	return otlptracehttp.New(ctx, otlptracehttp.WithEndpoint(endpoint), otlptracehttp.WithInsecure())
}

// Setup configura o tracer provider global e o propagador de contexto
// (text-map propagator). serviceName/environment são anexados a todo span
// como atributos de resource. endpoint é o endereço do collector
// OTLP/HTTP (host:porta, sem esquema) — tipicamente
// OTEL_EXPORTER_OTLP_ENDPOINT. Tracing é opcional: se o exporter não puder
// ser criado, o erro é registrado e o processo segue com o no-op — a
// observabilidade nunca derruba a API.
func Setup(ctx context.Context, serviceName, environment, endpoint string, logger *slog.Logger) Shutdown {
	// Todo serviço que fala com outro (chamadas de cliente HTTP,
	// publish/consume no RabbitMQ) deve propagar o contexto de trace da
	// mesma forma, independentemente de um exporter estar configurado ou
	// não — assim os headers de trace já saem corretos mesmo com tracing
	// desabilitado, e ligar um collector depois não exige mudar nenhum
	// outro código.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	if endpoint == "" {
		logger.Info("telemetry: OTEL_EXPORTER_OTLP_ENDPOINT not set, tracing is a no-op")
		return noop
	}

	exporter, err := newExporter(ctx, endpoint)
	if err != nil {
		logger.Error("telemetry: falha ao criar o exporter OTLP, tracing desligado", slog.Any("error", err))
		return noop
	}

	// Merge só falha com schema URLs conflitantes; um resource sem schema
	// nunca conflita.
	res, _ := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(
			semconv.ServiceName(serviceName),
			semconv.DeploymentEnvironmentNameKey.String(environment),
		),
	)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	logger.Info("telemetry: tracing enabled", slog.String("endpoint", endpoint))
	return tp.Shutdown
}

func noop(context.Context) error { return nil }
