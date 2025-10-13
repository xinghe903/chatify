package monitoring

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

// OTelConf 链路追踪配置
// type OTelConf struct {
// 	Endpoint string  // 数据上报地址
// 	Service  string  // 服务名称
// 	Exporter string  // exporter
// 	Ratio    float64 // 采样率
// }

// func NewDefaultOTelConf() *OTelConf {
// 	return &OTelConf{
// 		Endpoint: "http://127.0.0.1:14268/api/traces",
// 		Service:  "client-service",
// 		Exporter: "jaeger",
// 		Ratio:    1.0,
// 	}
// }

func newExporter(ctx context.Context, endpoint string) (trace.SpanExporter, error) {
	return otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(),
	)
}

// InitTraceProvider 初始化链路追踪
// endpoint: 链路追踪服务地址
// service: 服务名称
// exporter: exporter, 如jaeger, zipkin
// ratio: 采样率
func InitTraceProvider(endpoint, service, exporter string, ratio float64) {
	exp, err := newExporter(context.Background(), endpoint)
	if err != nil {
		return
	}
	tp := tracesdk.NewTracerProvider(
		// 将基于父span的采样率设置为100%
		tracesdk.WithSampler(tracesdk.ParentBased(tracesdk.TraceIDRatioBased(ratio))),
		// 始终确保在生产中批量处理
		tracesdk.WithBatcher(exp),
		// 在资源中记录有关此应用程序的信息
		tracesdk.WithResource(resource.NewSchemaless(
			semconv.ServiceNameKey.String(service),
			attribute.String("exporter", exporter),
			// attribute.Float64("float", 312.23),
		)),
	)
	fmt.Printf("init otel success\n")
	otel.SetTextMapPropagator(&propagation.TraceContext{})
	otel.SetTracerProvider(tp)
}
