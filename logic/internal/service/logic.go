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
	log *log.Helper
	uc  *biz.Logic
}

// NewLogicService new a greeter service.
func NewLogicService(uc *biz.Logic, logger log.Logger) *LogicService {
	return &LogicService{uc: uc, log: log.NewHelper(logger)}
}

// SendSystemPush 实现系统推送接口
func (s *LogicService) SendSystemPush(ctx context.Context, in *v1.SystemPushRequest) (*v1.SystemPushResponse, error) {
	s.log.WithContext(ctx).Infof("Receive system push request. ContentID: %s, UserCount: %d", in.ContentId, len(in.ToUserIds))

	// 调用业务层处理系统推送
	resp, err := s.uc.SendSystemPush(ctx, in)
	if err != nil {
		s.log.WithContext(ctx).Errorf("Failed to process system push: %v", err)
		return &v1.SystemPushResponse{
			Code:    v1.SystemPushResponse_OK,
			Message: "System error",
		}, nil
	}

	return resp, nil
}
