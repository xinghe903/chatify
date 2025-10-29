package biz

import (
	im_v1 "api/im/v1"
	v1 "api/offline/v1"
	"context"
	"offline/internal/biz/bo"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

// MessageRepo 定义消息存储仓库接口
type MessageRepo interface {
	// ArchiveMessages 归档消息
	ArchiveMessages(ctx context.Context, messages []*bo.OfflineMessage) error
	GetOfflineMessagesByUserID(ctx context.Context, userID string, lastMessageId string) ([]*bo.OfflineMessage, error)
	MarkMessagesAsDelivered(ctx context.Context, messageIDs []string) error
}

// OfflineUsecase 离线消息业务逻辑
type OfflineUsecase struct {
	messageRepo MessageRepo
	log         *log.Helper
}

// NewOfflineUsecase 创建离线消息业务逻辑实例
func NewOfflineUsecase(messageRepo MessageRepo, logger log.Logger) *OfflineUsecase {
	return &OfflineUsecase{
		messageRepo: messageRepo,
		log:         log.NewHelper(logger),
	}
}

// ArchiveMessages 实现消息归档业务逻辑
func (uc *OfflineUsecase) ArchiveMessages(ctx context.Context, taskId string, messages []*im_v1.BaseMessage) error {
	// 参数验证
	if len(messages) == 0 {
		uc.log.WithContext(ctx).Infof("message count=0, skip archive")
		return nil
	}
	if len(messages) > bo.BatchArchiveSize {
		uc.log.WithContext(ctx).Warnf("message count=%d, but max is %d", len(messages), bo.BatchArchiveSize)
		return v1.ErrorTooManyMessages("message count=%d, but max is %d", len(messages), bo.BatchArchiveSize)
	}
	uc.log.WithContext(ctx).Debugf("Preparing to archive %d messages", len(messages))
	// 将v1.BaseMessage转换为bo.OfflineMessage
	nowTime := time.Now().Unix()
	boMessages := make([]*bo.OfflineMessage, 0, len(messages))
	for _, msg := range messages {
		boMessages = append(boMessages, &bo.OfflineMessage{
			MsgID:       msg.MsgId,
			MessageType: int32(msg.MessageType),
			FromUserID:  msg.FromUserId,
			TargetType:  int32(msg.TargetType),
			ToUserID:    msg.ToUserId,
			Content:     msg.Content,
			Timestamp:   msg.Timestamp,
			ExpireTime:  msg.ExpireTime,
			ContentID:   msg.ContentId,
			TaskID:      taskId,
			Status:      bo.MessageStatusPending,
			Description: "archived offline message",
			CreatedAt:   nowTime,
			UpdatedAt:   nowTime,
		})
	}

	// 调用数据层归档消息
	err := uc.messageRepo.ArchiveMessages(ctx, boMessages)
	if err != nil {
		uc.log.WithContext(ctx).Errorf("failed to archive message. taskID=%s, error=%s", taskId, err.Error())
		return v1.ErrorArchiveMessageFailed("data archive failed")
	}
	uc.log.WithContext(ctx).Debugf("Successfully archived %d message", len(messages))
	return nil
}

// GetOfflineMessages 获取用户离线消息
func (uc *OfflineUsecase) GetOfflineMessages(ctx context.Context, userID string, lastMessageId string) ([]*bo.OfflineMessage, error) {
	// 参数验证
	if userID == "" {
		return nil, v1.ErrorInvalidUser("user ID cannot be empty")
	}
	if lastMessageId == "" {
		lastMessageId = "0"
	}
	uc.log.WithContext(ctx).Debugf("Getting offline message for user %s, last message ID: %s", userID, lastMessageId)
	// 调用数据层获取消息
	messages, err := uc.messageRepo.GetOfflineMessagesByUserID(ctx, userID, lastMessageId)
	if err != nil {
		uc.log.WithContext(ctx).Errorf("Failed to get offline message: %v", err)
		return nil, v1.ErrorGetOfflineMessageFailed("data get offline message failed")
	}
	return messages, nil
}

// MarkMessagesAsDelivered 标记消息为已送达
func (uc *OfflineUsecase) MarkMessagesAsDelivered(ctx context.Context, messageIDs []string) error {
	if len(messageIDs) == 0 {
		return nil // 空消息ID列表，直接返回
	}
	uc.log.WithContext(ctx).Debugf("Marking %d messages as delivered", len(messageIDs))
	// 调用数据层标记消息状态
	if err := uc.messageRepo.MarkMessagesAsDelivered(ctx, messageIDs); err != nil {
		uc.log.WithContext(ctx).Errorf("Failed to mark message as delivered: %v", err)
		return v1.ErrorMarkMessageAsDeliveredFailed("data mark message as delivered failed")
	}
	uc.log.WithContext(ctx).Debugf("Successfully marked %d messages as delivered", len(messageIDs))
	return nil
}
