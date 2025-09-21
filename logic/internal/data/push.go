package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"logic/internal/biz"
	"logic/internal/biz/bo"
	"logic/internal/conf"
	pb "push/api/push/v1"
)

// PushClient 封装Push服务的客户端操作
type PushClient struct {
	client pb.PushServiceClient
	conn   *grpc.ClientConn
	logger *log.Helper
}

// NewPushServiceClient 创建Push服务gRPC客户端
func NewPushServiceClient(c *conf.Bootstrap, logger log.Logger) (biz.PushRepo, func()) {
	cfg := c.PushClient
	addr := "localhost:30023"
	timeout := 5 * time.Second

	// 使用配置文件中的值（如果有）
	if cfg != nil {
		if cfg.Addr != "" {
			addr = cfg.Addr
		}
		if cfg.Timeout != nil {
			timeout = cfg.Timeout.AsDuration()
		}
	}
	return nil, nil

	// 创建gRPC连接
	conn, err := grpc.DialContext(
		context.Background(),
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(timeout),
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
		}
	}

	return &PushClient{
		client: client,
		conn:   conn,
		logger: log.NewHelper(log.With(logger, "module", "push-service-client")),
	}, cleanup
}

func (p *PushClient) SendMessage(ctx context.Context, message *bo.Message) error {
	_, err := p.client.PushMessage(ctx, &pb.PushMessageRequest{
		MessageId:   message.MessageId,
		Content:     message.Content,
		ContentType: message.ContentType,
		ExpireTime:  message.ExpireTime,
		UserIds:     message.UserIds,
	})
	return err
}
