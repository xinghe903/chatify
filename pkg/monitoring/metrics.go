package monitoring

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/middleware/metrics"
	"go.opentelemetry.io/otel/exporters/prometheus"
	otlpmetric "go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

var (
	MetricRequests otlpmetric.Int64Counter
	MetricSeconds  otlpmetric.Float64Histogram
	Meter          otlpmetric.Meter
)

func InitPrometheus(serviceName string) error {
	exporter, err := prometheus.New()
	if err != nil {
		return err
	}
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
	)
	Meter = provider.Meter(serviceName)
	MetricRequests, err = metrics.DefaultRequestsCounter(Meter, metrics.DefaultServerRequestsCounterName)
	if err != nil {
		return err
	}
	MetricSeconds, err = metrics.DefaultSecondsHistogram(Meter, metrics.DefaultServerSecondsHistogramName)
	if err != nil {
		return err
	}
	fmt.Printf("init metric success\n")
	return nil
}
