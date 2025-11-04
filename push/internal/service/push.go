package service

import (
	"context"

	"github.com/xinghe903/chatify/push/internal/biz"
	"github.com/xinghe903/chatify/push/internal/biz/bo"

	v1 "github.com/xinghe903/chatify/api/push/v1"

	"github.com/go-kratos/kratos/v2/log"
)

// pushService is a push service.
type PushService struct {
	v1.UnimplementedPushServiceServer
	consumerHandler *biz.UserStateHandler
	uc              *biz.Push
	log             *log.Helper
}

// NewPushService new a push service.
func NewPushService(uc *biz.Push, logger log.Logger, handler *biz.UserStateHandler) *PushService {
	return &PushService{
		uc:              uc,
		log:             log.NewHelper(logger),
		consumerHandler: handler, // 仅用作进入handler并初始化数据消费协程
	}
}

// PushToUser 实现向指定用户推送消息的RPC方法
func (s *PushService) PushToUser(ctx context.Context, in *v1.PushRequest) (*v1.PushResponse, error) {
	s.log.WithContext(ctx).Debugf("PushToUser request received. taskID=%s, messageCount=%d", in.TaskId, len(in.Message))
	// 检查消息数量是否超过限制
	if len(in.Message) > bo.MaxMessageCount {
		s.log.WithContext(ctx).Warnf("message count=%d, but max is %d",
			len(in.Message), bo.MaxMessageCount)
		return nil, v1.ErrorTooManyMessages("message count=%d, but max is %d",
			len(in.Message), bo.MaxMessageCount)
	}

	// 调用业务层的PushToUser方法处理消息推送
	err := s.uc.PushToUser(ctx, in.TaskId, in.Message)
	if err != nil {
		return nil, err
	}
	s.log.WithContext(ctx).Debugf("PushToUser request processed successfully. taskID=%s, messageCount=%d",
		in.TaskId, len(in.Message))
	return &v1.PushResponse{}, nil
}
