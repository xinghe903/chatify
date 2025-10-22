package service

import (
	"context"

	v1 "api/push/v1"
	"push/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

// pushService is a push service.
type PushService struct {
	v1.UnimplementedPushServiceServer

	uc  *biz.Push
	log *log.Helper
}

// NewPushService new a push service.
func NewPushService(uc *biz.Push, logger log.Logger) *PushService {
	return &PushService{
		uc:  uc,
		log: log.NewHelper(logger),
	}
}

// PushToUser 实现向指定用户推送消息的RPC方法
func (s *PushService) PushToUser(ctx context.Context, in *v1.PushRequest) (*v1.PushResponse, error) {
	s.log.WithContext(ctx).Debugf("PushToUser request received. taskID=%s, messageCount=%d", in.TaskId, len(in.Message))

	// 检查消息数量是否超过限制
	if len(in.Message) > 1000 {
		s.log.WithContext(ctx).Warnf("too many messages in one request. count=%d", len(in.Message))
		return &v1.PushResponse{
			Code:    v1.PushResponse_TOO_MANY_MESSAGE,
			Message: "Too many messages in one request",
		}, nil
	}

	// 调用业务层的PushToUser方法处理消息推送
	err := s.uc.PushToUser(ctx, in.TaskId, in.Message)
	if err != nil {
		s.log.WithContext(ctx).Errorf("PushToUser failed. error=%s", err.Error())
		return nil, err
	}

	// 返回成功响应
	response := &v1.PushResponse{
		Code:    v1.PushResponse_OK,
		Message: "Push to user success",
	}
	s.log.WithContext(ctx).Debugf("PushToUser request processed successfully. taskID=%s, messageCount=%d", in.TaskId, len(in.Message))
	return response, nil
}

// BatchQueryOnline 实现批量查询用户在线状态的RPC方法
func (s *PushService) BatchQueryOnline(ctx context.Context, in *v1.BatchQueryRequest) (*v1.BatchQueryResponse, error) {
	s.log.WithContext(ctx).Debugf("BatchQueryOnline request received. userCount=%d", len(in.UserIds))

	// 实现批量查询用户在线状态的逻辑
	// 这里为了示例，返回模拟数据
	statuses := make([]*v1.BatchQueryResponse_UserStatus, 0, len(in.UserIds))

	for _, userId := range in.UserIds {
		// 实际项目中应该从Redis或其他存储中查询用户是否在线
		status := &v1.BatchQueryResponse_UserStatus{
			UserId: userId,
			Online: true,    // 模拟用户在线
			Node:   "node1", // 模拟节点信息
		}
		statuses = append(statuses, status)
	}

	response := &v1.BatchQueryResponse{
		Statuses: statuses,
	}
	s.log.WithContext(ctx).Debugf("BatchQueryOnline request processed successfully. userCount=%d", len(in.UserIds))
	return response, nil
}
