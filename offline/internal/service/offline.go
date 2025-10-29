package service

import (
	"context"

	im_v1 "api/im/v1"
	v1 "api/offline/v1"
	"offline/internal/biz"
	"offline/internal/biz/bo"

	"github.com/go-kratos/kratos/v2/log"
)

// OfflineService 离线消息服务
type OfflineService struct {
	v1.UnimplementedOfflineServiceServer

	uc  *biz.OfflineUsecase
	log *log.Helper
}

// NewOfflineService 创建离线消息服务实例
func NewOfflineService(uc *biz.OfflineUsecase, logger log.Logger) *OfflineService {
	return &OfflineService{
		uc:  uc,
		log: log.NewHelper(logger),
	}
}

// ArchiveMessages 实现归档消息的RPC方法
func (s *OfflineService) ArchiveMessages(ctx context.Context, in *v1.ArchiveRequest) (*v1.ArchiveResponse, error) {
	s.log.WithContext(ctx).Debugf("ArchiveMessages request received. messageCount=%d", len(in.Message))
	// 检查消息数量是否超过限制
	if len(in.Message) > bo.BatchArchiveSize {
		s.log.WithContext(ctx).Warnf("message count=%d, but max is %d", len(in.Message), bo.BatchArchiveSize)
		return nil, v1.ErrorTooManyMessages("message count=%d, but max is %d", len(in.Message), bo.BatchArchiveSize)
	}

	// 调用业务层的ArchiveMessages方法处理消息归档
	err := s.uc.ArchiveMessages(ctx, in.TaskId, in.Message)
	if err != nil {
		return nil, err
	}
	s.log.WithContext(ctx).Debugf("ArchiveMessages request processed successfully. messageCount=%d", len(in.Message))
	return &v1.ArchiveResponse{}, nil
}

func (s *OfflineService) RetrieveOfflineMessages(ctx context.Context, in *v1.RetrieveRequest) (*v1.RetrieveResponse, error) {
	s.log.WithContext(ctx).Debugf("RetrieveOfflineMessages request received. userId=%s, lastMessageId=%s", in.UserId, in.LastMessageId)
	messages, err := s.uc.GetOfflineMessages(ctx, in.UserId, in.LastMessageId)
	if err != nil {
		return nil, err
	}
	data := make([]*im_v1.BaseMessage, len(messages))
	for i, msg := range messages {
		data[i] = &im_v1.BaseMessage{
			MsgId:       msg.MsgID,
			MessageType: im_v1.MessageType(msg.MessageType),
			FromUserId:  msg.FromUserID,
			TargetType:  im_v1.TargetType(msg.TargetType),
			ToUserId:    msg.ToUserID,
			Content:     msg.Content,
		}
	}
	return &v1.RetrieveResponse{}, nil
}

func (s *OfflineService) AcknowledgeMessages(ctx context.Context, in *v1.AckRequest) (*v1.AckResponse, error) {
	if err := s.uc.MarkMessagesAsDelivered(ctx, in.MessageIds); err != nil {
		return nil, err
	}
	return &v1.AckResponse{}, nil
}
