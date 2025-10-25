package data

import (
	im_v1 "api/im/v1"
	pb "api/offline/v1"
	"context"
	"fmt"
	"pkg/auth"
	"push/internal/biz"
	"push/internal/biz/bo"
	"push/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/circuitbreaker"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/transport/grpc"
)

var _ biz.OfflineRepo = (*OfflineClient)(nil)

// OfflineClient 封装offline服务的客户端操作
type OfflineClient struct {
	client    pb.OfflineServiceClient
	log       *log.Helper
	sonyFlake *auth.Sonyflake
}

// NewOfflineClient 创建offline服务gRPC客户端
func NewOfflineClient(c *conf.Bootstrap, logger log.Logger, r registry.Discovery) (biz.OfflineRepo, func()) {
	cfg := c.Client.OfflineClient
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
		panic("Failed to create offline service gRPC connection. " + err.Error())
	}
	log.NewHelper(logger).Info("offline service gRPC client initialized successfully")
	// 创建客户端
	client := pb.NewOfflineServiceClient(conn)
	cleanup := func() {
		if err := conn.Close(); err != nil {
			log.NewHelper(logger).Errorf("Failed to close offline service connection: %v", err)
		} else {
			log.NewHelper(logger).Info("offline service connection closed successfully")
		}
	}
	return &OfflineClient{
		client: client,
		// conn:   conn,
		sonyFlake: auth.NewSonyflake(),
		log:       log.NewHelper(logger),
	}, cleanup
}

func (p *OfflineClient) ArchiveMessages(ctx context.Context, taskId string, messages []*bo.Message) error {
	if len(messages) == 0 {
		return nil
	}
	p.log.WithContext(ctx).Debugf("Sending message to offline service msg len=%d", len(messages))
	// 转换为proto定义的BaseMessage
	baseMessages := make([]*im_v1.BaseMessage, 0, len(messages))
	for _, message := range messages {
		baseMessages = append(baseMessages, message.ToBaseMessage())
	}
	// 调用ArchiveMessages接口发送消息
	resp, err := p.client.ArchiveMessages(ctx, &pb.ArchiveRequest{
		TaskId:  taskId,
		Message: baseMessages,
	})
	if err != nil {
		p.log.WithContext(ctx).Errorf("Failed to offline message to user: %v", err)
		return err
	}

	p.log.WithContext(ctx).Debugf("offline message to user response: %v", resp)
	return nil
}

func (p *OfflineClient) RetrieveOfflineMessages(ctx context.Context, userID string) ([]*bo.Message, error) {
	if userID == "" {
		return nil, fmt.Errorf("user id is empty")
	}
	resp, err := p.client.RetrieveOfflineMessages(ctx, &pb.RetrieveRequest{
		UserId: userID,
	})
	if err != nil {
		p.log.WithContext(ctx).Errorf("Failed to retrieve offline message: %v", err)
		return nil, err
	}
	if resp.Code != pb.RetrieveResponse_OK {
		return nil, fmt.Errorf("Failed to retrieve offline message: %s", resp.Code)
	}
	var messages []*bo.Message
	for _, message := range resp.Message {
		messages = append(messages, bo.NewMessage(message))
	}
	return messages, nil
}
