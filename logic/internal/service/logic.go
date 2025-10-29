package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	v1 "api/logic/v1"
	"logic/internal/biz"
)

// LogicService is a greeter service.
type LogicService struct {
	v1.UnimplementedLogicServiceServer
	log      *log.Helper
	uc       *biz.Logic
	consumer *biz.UserMessageHandler
}

// NewLogicService new a greeter service.
func NewLogicService(uc *biz.Logic, logger log.Logger, consumer *biz.UserMessageHandler) *LogicService {
	return &LogicService{uc: uc,
		log:      log.NewHelper(logger),
		consumer: consumer, // 仅用作进入handler并初始化数据消费协程
	}
}

// SendSystemPush 实现系统推送接口
func (s *LogicService) SendSystemPush(ctx context.Context, in *v1.SystemPushRequest) (*v1.SystemPushResponse, error) {
	s.log.WithContext(ctx).Infof("Receive system push request. ContentID: %s, UserCount: %d", in.ContentId, len(in.ToUserIds))

	// 调用业务层处理系统推送
	resp, err := s.uc.SendSystemPush(ctx, in)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
