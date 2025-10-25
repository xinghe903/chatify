package data

import (
	im_v1 "api/im/v1"
	pb "api/push/v1"
	"context"
	"fmt"
	"logic/internal/biz"
	"logic/internal/biz/bo"
	"logic/internal/conf"
	"pkg/auth"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/circuitbreaker"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/transport/grpc"
)

// PushClient 封装Push服务的客户端操作
type PushClient struct {
	client    pb.PushServiceClient
	log       *log.Helper
	sonyFlake *auth.Sonyflake
}

// NewPushServiceClient 创建Push服务gRPC客户端
func NewPushServiceClient(c *conf.Bootstrap, logger log.Logger, r registry.Discovery) (biz.PushRepo, func()) {
	cfg := c.Client.PushClient
	conn, err := grpc.DialInsecure(
		context.Background(),
		grpc.WithEndpoint(fmt.Sprintf("discovery:///%s", cfg.Addr)),
		grpc.WithDiscovery(r),
		grpc.WithMiddleware(
			tracing.Client(),
			circuitbreaker.Client(),
		),
	)
	if err != nil {
		panic("Failed to create push service gRPC connection. " + err.Error())
	}
	log.NewHelper(logger).Info("Push service gRPC client initialized successfully")
	// 创建客户端
	client := pb.NewPushServiceClient(conn)
	cleanup := func() {
		if err := conn.Close(); err != nil {
			log.NewHelper(logger).Errorf("Failed to close push service connection: %v", err)
		} else {
			log.NewHelper(logger).Info("Push service connection closed successfully")
		}
	}
	return &PushClient{
		client: client,
		// conn:   conn,
		sonyFlake: auth.NewSonyflake(),
		log:       log.NewHelper(logger),
	}, cleanup
}

func (p *PushClient) SendMessage(ctx context.Context, taskId string, messages []*bo.Message) error {
	p.log.WithContext(ctx).Debugf("Sending message to push service msg len=%d", len(messages))
	// 转换为proto定义的BaseMessage
	baseMessages := make([]*im_v1.BaseMessage, 0, len(messages))
	for _, message := range messages {
		baseMessages = append(baseMessages, message.ToBaseMessage())
	}
	// 调用PushToUser接口发送消息
	resp, err := p.client.PushToUser(ctx, &pb.PushRequest{
		TaskId:  taskId,
		Message: baseMessages,
	})
	if err != nil {
		p.log.WithContext(ctx).Errorf("Failed to push message to user: %v", err)
		return err
	}

	p.log.WithContext(ctx).Debugf("Push message to user response: %v", resp)
	return nil
}
