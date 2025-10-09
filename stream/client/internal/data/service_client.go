package data

import (
	v1 "api/service/v1"
	"client/internal/biz"
	"client/internal/conf"
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	ggrpc "google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/stats"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// serviceClient 是调用 service 服务的客户端实现
type serviceClient struct {
	client v1.ServiceClient
	log    *log.Helper
}

// 确保 serviceClient 实现了 biz.ServiceClient 接口
var _ biz.ServiceClient = (*serviceClient)(nil)

// MultiStatsHandler 组合多个 stats.Handler，转发所有事件
type MultiStatsHandler struct {
	handlers []stats.Handler
}

// NewMultiStatsHandler 创建一个组合的 stats.Handler
func NewMultiStatsHandler(handlers ...stats.Handler) *MultiStatsHandler {
	return &MultiStatsHandler{handlers: handlers}
}

// TagRPC 转发到所有 handler
func (m *MultiStatsHandler) TagRPC(ctx context.Context, info *stats.RPCTagInfo) context.Context {
	for _, h := range m.handlers {
		ctx = h.TagRPC(ctx, info)
	}
	return ctx
}

// HandleRPC 转发到所有 handler
func (m *MultiStatsHandler) HandleRPC(ctx context.Context, s stats.RPCStats) {
	for _, h := range m.handlers {
		h.HandleRPC(ctx, s)
	}
}

// TagConn 转发到所有 handler
func (m *MultiStatsHandler) TagConn(ctx context.Context, info *stats.ConnTagInfo) context.Context {
	for _, h := range m.handlers {
		ctx = h.TagConn(ctx, info)
	}
	return ctx
}

// HandleConn 转发到所有 handler
func (m *MultiStatsHandler) HandleConn(ctx context.Context, s stats.ConnStats) {
	for _, h := range m.handlers {
		h.HandleConn(ctx, s)
	}
}

type finalMetadataLogger struct {
	log *log.Helper
}

// TagRPC 实现
func (f *finalMetadataLogger) TagRPC(ctx context.Context, info *stats.RPCTagInfo) context.Context {

	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		f.log.WithContext(ctx).Debugf("Server received stream metadata: %v", md)
	} else {
		f.log.WithContext(ctx).Debugf("Server received stream with no metadata")
	}
	return ctx
}

// HandleRPC 实现
func (f *finalMetadataLogger) HandleRPC(ctx context.Context, s stats.RPCStats) {
	if outHeader, ok := s.(*stats.OutHeader); ok {
		if outHeader.Client {
			f.log.WithContext(ctx).Debugf("✅ FINAL Outgoing Headers (to server): %v", outHeader.Header)
		}
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		f.log.WithContext(ctx).Debugf("Server received stream metadata: %v", md)
	} else {
		f.log.WithContext(ctx).Debugf("Server received stream with no metadata")
	}
}

// TagConn 实现（空实现）
func (f *finalMetadataLogger) TagConn(ctx context.Context, info *stats.ConnTagInfo) context.Context {
	return ctx
}

// HandleConn 实现（空实现）
func (f *finalMetadataLogger) HandleConn(ctx context.Context, s stats.ConnStats) {
	// 不做任何事
	// 如果你想打印连接事件，可以在这里添加日志
	// 例如：连接创建、关闭、流量统计等
}

// otelUnaryClientInterceptor 是一个一元拦截器，用于将 trace 信息注入 gRPC metadata
func otelUnaryClientInterceptor() ggrpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *ggrpc.ClientConn, invoker ggrpc.UnaryInvoker, opts ...ggrpc.CallOption) error {

		span := trace.SpanFromContext(ctx)
		sc := span.SpanContext()

		fmt.Printf(" Span valid: %v\n", sc.IsValid())
		fmt.Printf(" TraceID: %s\n", sc.TraceID().String())
		fmt.Printf(" SpanID: %s\n", sc.SpanID().String())

		// 获取全局的 propagator（通常是 W3C Trace Context）
		propagator := otel.GetTextMapPropagator()
		// 创建一个 carrier（载体），用于存放 trace header
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		}
		fmt.Printf("🔧 Before inject, MD: %v\n", md)

		// 使用 metadata 作为 carrier
		carrier := propagation.HeaderCarrier(md)
		propagator.Inject(ctx, carrier)

		// 把更新后的 metadata 放回 context
		ctx = metadata.NewOutgoingContext(ctx, metadata.MD(carrier))
		fmt.Printf("🔧 After inject, MD: %v\n", carrier)

		// 继续调用
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// otelStreamClientInterceptor 是一个流式拦截器，用于注入 trace 信息
func otelStreamClientInterceptor() ggrpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *ggrpc.StreamDesc, cc *ggrpc.ClientConn, method string, streamer ggrpc.Streamer, opts ...ggrpc.CallOption) (ggrpc.ClientStream, error) {
		propagator := otel.GetTextMapPropagator()

		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		}

		carrier := propagation.HeaderCarrier(md)
		propagator.Inject(ctx, carrier)

		ctx = metadata.NewOutgoingContext(ctx, metadata.MD(carrier))

		return streamer(ctx, desc, cc, method, opts...)
	}
}

// NewServiceClient 创建一个新的 service 客户端
func NewServiceClient(c *conf.Data, logger log.Logger) (biz.ServiceClient, func(), error) {
	// 创建组合的 stats handler
	multiStatsHandler := NewMultiStatsHandler(
		otelgrpc.NewClientHandler(),                      // 保留 OpenTelemetry 支持
		&finalMetadataLogger{log: log.NewHelper(logger)}, // 打印最终 metadata
	)
	// 创建 gRPC 客户端连接
	conn, err := grpc.DialInsecure(
		context.Background(),
		grpc.WithEndpoint(c.Service.Addr),
		grpc.WithTimeout(c.Service.Timeout.AsDuration()),
		grpc.WithMiddleware(
			recovery.Recovery(),
			logging.Client(logger),
		),
		grpc.WithOptions(
			ggrpc.WithStatsHandler(multiStatsHandler),
			// ggrpc.WithUnaryInterceptor(otelUnaryClientInterceptor()),   // ✅ 注入 trace
			// ggrpc.WithStreamInterceptor(otelStreamClientInterceptor()), // ✅ 流式注入
		),
	)
	if err != nil {
		panic("failed to create service client " + err.Error())
	}

	// 清理函数
	cleanup := func() {
		if err := conn.Close(); err != nil {
			log.NewHelper(logger).Errorf("failed to close service client connection: %v", err)
		}
	}

	// 创建服务客户端
	client := v1.NewServiceClient(conn)

	return &serviceClient{client: client, log: log.NewHelper(logger)}, cleanup, nil
}

// Chat 调用 service 服务的 Chat 方法
func (s *serviceClient) Chat(ctx context.Context, name string) (string, error) {
	s.log.WithContext(ctx).Debugf("Chat: %s", name)
	// 调用 service 服务的 Chat 方法
	resp, err := s.client.Chat(ctx, &v1.ChatReq{Name: name})
	if err != nil {
		return "", err
	}

	return resp.Message, nil
}

// ChatStream 处理双向流式 RPC，返回一个可以发送和接收消息的流
func (s *serviceClient) ChatStream(ctx context.Context, name string) (string, error) {
	s.log.WithContext(ctx).Debugf("Chat: %s", name)
	// 创建双向流式 RPC 连接
	stream, err := s.client.ChatStream(ctx)
	if err != nil {
		return "", err
	}

	// 发送请求
	for i := 0; i < 5; i++ {
		s.log.WithContext(ctx).Debugf("Sending: %s", fmt.Sprintf("%s:%d", name, i))
		if err := stream.Send(&v1.ChatReq{Name: fmt.Sprintf("%s:%d", name, i)}); err != nil {
			return "", err
		}
	}

	go func() {
		// 接收响应
		for i := 0; i < 5; i++ {
			resp, err := stream.Recv()
			if err != nil {
				s.log.WithContext(ctx).Errorf("Receive error: %v", err)
				return
			}
			s.log.WithContext(ctx).Debugf("index=%d, Received: %s", i, resp.Message)
		}
	}()

	// 关闭发送方向，完成流式 RPC
	if err := stream.CloseSend(); err != nil {
		return "", err
	}

	return "", nil
}
