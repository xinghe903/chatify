package server

import (
	"access/internal/conf"
	"access/internal/service"
	basehttp "net/http"

	"github.com/xinghe903/chatify/pkg/monitoring"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/metrics"
	"github.com/go-kratos/kratos/v2/middleware/ratelimit"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// NewHTTPServer new an HTTP server.
func NewHTTPServer(cb *conf.Bootstrap, svc *service.AccessService, logger log.Logger) *http.Server {
	c := cb.Server
	var opts = []http.ServerOption{
		http.Middleware(
			recovery.Recovery(),
			tracing.Server(),
			metrics.Server(
				metrics.WithSeconds(monitoring.MetricSeconds),
				metrics.WithRequests(monitoring.MetricRequests),
			),
			ratelimit.Server(),
		),
	}
	if c.Http.Network != "" {
		opts = append(opts, http.Network(c.Http.Network))
	}
	if c.Http.Addr != "" {
		opts = append(opts, http.Address(c.Http.Addr))
	}
	if c.Http.Timeout != nil {
		opts = append(opts, http.Timeout(c.Http.Timeout.AsDuration()))
	}
	srv := http.NewServer(opts...)
	srv.Handle("/metrics", promhttp.Handler())
	srv.HandleFunc("/chatify/access/v1/ws", func(w basehttp.ResponseWriter, r *basehttp.Request) {
		// 正确获取操作名称和上下文
		operation := r.RequestURI
		ctx := r.Context()
		// 使用 Kratos 的 trace 工具直接从请求头中提取或创建 trace 信息
		spanCtx := trace.SpanContextFromContext(ctx)
		if !spanCtx.IsValid() {
			// 如果上下文中没有有效的 span，创建一个新的 span
			tracer := otel.Tracer("websocket")
			var span trace.Span
			ctx, span = tracer.Start(ctx, operation)
			defer span.End()
			// 将上下文传递给服务处理函数，确保追踪上下文传播
			svc.ServeHTTP(w, r.WithContext(ctx))
		} else {
			// 如果已经有有效的 span，直接使用
			svc.ServeHTTP(w, r)
		}
	})
	return srv
}
