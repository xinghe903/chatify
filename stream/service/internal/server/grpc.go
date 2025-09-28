package server

import (
	v1 "service/api/service/v1"
	"service/internal/conf"
	"service/internal/service"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	ggrpc "google.golang.org/grpc"
)

// NewGRPCServer new a gRPC server.
func NewGRPCServer(c *conf.Server, svc *service.ChatService, logger log.Logger) *grpc.Server {
	InitTraceProvider("http://127.0.0.1:14268/api/traces")
	var opts = []grpc.ServerOption{
		grpc.Middleware(
			recovery.Recovery(),
			tracing.Server(),
		),
		grpc.StreamInterceptor(NewStreamTracingInterceptor("service-name").StreamServerInterceptor()),
		grpc.Options(
			ggrpc.StatsHandler(otelgrpc.NewServerHandler()),
		),
	}
	if c.Grpc.Network != "" {
		opts = append(opts, grpc.Network(c.Grpc.Network))
	}
	if c.Grpc.Addr != "" {
		opts = append(opts, grpc.Address(c.Grpc.Addr))
	}
	if c.Grpc.Timeout != nil {
		opts = append(opts, grpc.Timeout(c.Grpc.Timeout.AsDuration()))
	}
	srv := grpc.NewServer(opts...)
	v1.RegisterServiceServer(srv, svc)
	return srv
}
