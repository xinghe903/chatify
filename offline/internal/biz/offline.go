package biz

import (
	v1 "api/im/v1"
	"context"
	"errors"
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
		log:         log.NewHelper(log.With(logger, "module", "offline-usecase")),
	}
}

// ArchiveMessages 实现消息归档业务逻辑
func (uc *OfflineUsecase) ArchiveMessages(ctx context.Context, taskId string, messages []*v1.BaseMessage) error {
	// 参数验证
	if len(messages) == 0 {
		return errors.New("messages cannot be empty")
	}
	if len(messages) > bo.BatchArchiveSize {
		return errors.New("too many messages in one request")
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
		uc.log.WithContext(ctx).Errorf("Failed to archive messages: %v", err)
		return err
	}

	uc.log.WithContext(ctx).Debugf("Successfully archived %d messages", len(messages))
	return nil
}

// GetOfflineMessages 获取用户离线消息
func (uc *OfflineUsecase) GetOfflineMessages(ctx context.Context, userID string, lastMessageId string) ([]*bo.OfflineMessage, error) {
	// 参数验证
	if userID == "" {
		return nil, errors.New("user ID cannot be empty")
	}
	if lastMessageId == "" {
		lastMessageId = "0"
	}
	uc.log.WithContext(ctx).Debugf("Getting offline messages for user %s, last message ID: %s", userID, lastMessageId)
	// 调用数据层获取消息
	messages, err := uc.messageRepo.GetOfflineMessagesByUserID(ctx, userID, lastMessageId)
	if err != nil {
		uc.log.WithContext(ctx).Errorf("Failed to get offline messages: %v", err)
		return nil, err
	}
	return messages, nil
}

// MarkMessagesAsDelivered 标记消息为已送达
func (uc *OfflineUsecase) MarkMessagesAsDelivered(ctx context.Context, messageIDs []string) error {
	if len(messageIDs) == 0 {
		return errors.New("message IDs cannot be empty")
	}

	uc.log.WithContext(ctx).Debugf("Marking %d messages as delivered", len(messageIDs))

	// 调用数据层标记消息状态
	if err := uc.messageRepo.MarkMessagesAsDelivered(ctx, messageIDs); err != nil {
		uc.log.WithContext(ctx).Errorf("Failed to mark messages as delivered: %v", err)
		return err
	}

	uc.log.WithContext(ctx).Debugf("Successfully marked %d messages as delivered", len(messageIDs))
	return nil
}
