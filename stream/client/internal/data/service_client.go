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
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	ggrpc "google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// serviceClient 是调用 service 服务的客户端实现
type serviceClient struct {
	client v1.ServiceClient
	log    *log.Helper
}

// 确保 serviceClient 实现了 biz.ServiceClient 接口
var _ biz.ServiceClient = (*serviceClient)(nil)

// NewServiceClient 创建一个新的 service 客户端
func NewServiceClient(c *conf.Data, logger log.Logger) (biz.ServiceClient, func(), error) {
	// 创建 gRPC 客户端连接
	conn, err := grpc.DialInsecure(
		context.Background(),
		grpc.WithEndpoint(c.Service.Addr),
		grpc.WithTimeout(c.Service.Timeout.AsDuration()),
		grpc.WithMiddleware(
			recovery.Recovery(),
			logging.Client(logger),
		),
		grpc.WithOptions(ggrpc.WithStatsHandler(otelgrpc.NewClientHandler())),
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
	// 创建双向流式 RPC 连接
	stream, err := s.client.Chat(ctx)
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

	// 接收响应
	for i := 0; i < 5; i++ {
		resp, err := stream.Recv()
		if err != nil {
			return "", err
		}
		s.log.WithContext(ctx).Debugf("index=%d, Received: %s", i, resp.Message)
	}

	// 关闭发送方向，完成流式 RPC
	if err := stream.CloseSend(); err != nil {
		return "", err
	}

	return "", nil
}

// ChatStream 处理双向流式 RPC，返回一个可以发送和接收消息的流
func (s *serviceClient) ChatStream(ctx context.Context) (v1.Service_ChatClient, error) {
	s.log.WithContext(ctx).Debugf("Creating ChatStream")
	md := metadata.New(nil)
	propagator := otel.GetTextMapPropagator()
	propagator.Inject(ctx, propagation.HeaderCarrier(md))

	// 创建带有metadata的上下文
	ctx = metadata.NewOutgoingContext(ctx, md)
	return s.client.Chat(ctx)
}
