package data

import (
	v1 "api/service/v1"
	"client/internal/biz"
	"client/internal/conf"
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/grpc"
)

// serviceClient 是调用 service 服务的客户端实现
type serviceClient struct {
	client v1.ServiceClient
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
	)
	if err != nil {
		return nil, nil, err
	}

	// 清理函数
	cleanup := func() {
		if err := conn.Close(); err != nil {
			log.NewHelper(logger).Errorf("failed to close service client connection: %v", err)
		}
	}

	// 创建服务客户端
	client := v1.NewServiceClient(conn)

	return &serviceClient{client: client}, cleanup, nil
}

// Chat 调用 service 服务的 Chat 方法
func (s *serviceClient) Chat(ctx context.Context, name string) (string, error) {
	// 创建双向流式 RPC 连接
	stream, err := s.client.Chat(ctx)
	if err != nil {
		return "", err
	}

	// 发送请求
	if err := stream.Send(&v1.ChatReq{Name: name}); err != nil {
		return "", err
	}

	// 接收响应
	resp, err := stream.Recv()
	if err != nil {
		return "", err
	}

	// 关闭发送方向，完成流式 RPC
	if err := stream.CloseSend(); err != nil {
		return "", err
	}

	return resp.Message, nil
}

// ChatStream 处理双向流式 RPC，返回一个可以发送和接收消息的流
func (s *serviceClient) ChatStream(ctx context.Context) (v1.Service_ChatClient, error) {
	return s.client.Chat(ctx)
}
