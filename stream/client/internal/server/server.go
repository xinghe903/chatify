package server

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/wire"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

// ProviderSet is server providers.
var ProviderSet = wire.NewSet(NewGRPCServer, NewHTTPServer)

var once sync.Once

func newExporter(ctx context.Context, endpoint string) (trace.SpanExporter, error) {
	return otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(),
	)
}

func InitTraceProvider(url string) {
	once.Do(func() {
		exp, err := newExporter(context.Background(), "http://127.0.0.1:14268/api/traces")
		if err != nil {
			return
		}
		tp := tracesdk.NewTracerProvider(
			// 将基于父span的采样率设置为100%
			tracesdk.WithSampler(tracesdk.ParentBased(tracesdk.TraceIDRatioBased(1.0))),
			// 始终确保在生产中批量处理
			tracesdk.WithBatcher(exp),
			// 在资源中记录有关此应用程序的信息
			tracesdk.WithResource(resource.NewSchemaless(
				semconv.ServiceNameKey.String("client-service"),
				attribute.String("exporter", "jaeger"),
				// attribute.Float64("float", 312.23),
			)),
		)
		fmt.Printf("init trace success\n")
		otel.SetTracerProvider(tp)
	})
}
