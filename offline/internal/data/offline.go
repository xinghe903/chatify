package data

import (
	"context"
	"pkg/auth"
	"time"

	"offline/internal/biz"
	"offline/internal/biz/bo"
	"offline/internal/data/po"

	"github.com/go-kratos/kratos/v2/log"
)

var _ biz.MessageRepo = (*OfflineMessageRepo)(nil)

// OfflineMessageRepo 离线消息仓库实现
type OfflineMessageRepo struct {
	data      *Data
	log       *log.Helper
	sonyFlake *auth.Sonyflake
}

// NewOfflineMessageRepo 创建离线消息仓库实例
func NewOfflineMessageRepo(data *Data, logger log.Logger) biz.MessageRepo {
	return &OfflineMessageRepo{
		data:      data,
		log:       log.NewHelper(logger),
		sonyFlake: auth.NewSonyflake(),
	}
}

// ArchiveMessages 归档消息到MySQL数据库
func (r *OfflineMessageRepo) ArchiveMessages(ctx context.Context, messages []*bo.OfflineMessage) error {
	if len(messages) == 0 {
		return nil
	}

	r.log.WithContext(ctx).Infof("Archiving %d messages", len(messages))

	// 将BO转换为PO实体
	offlineMessages := make([]*po.OfflineMessage, 0, len(messages))
	now := time.Now()
	var err error
	for _, msg := range messages {
		m := po.NewOfflineMessageFromBo(msg)
		if m.ID, err = r.sonyFlake.GenerateBase62(); err != nil {
			r.log.WithContext(ctx).Errorf("failed to generate offline message ID. err=%s", err.Error())
		}
		m.CreatedAt = now
		m.UpdatedAt = now
		offlineMessages = append(offlineMessages, m)
	}

	// 批量插入数据库
	result := r.data.db.WithContext(ctx).CreateInBatches(offlineMessages, bo.BatchArchiveSize)
	if result.Error != nil {
		r.log.WithContext(ctx).Errorf("Failed to archive messages: %v", result.Error)
		return result.Error
	}

	r.log.WithContext(ctx).Infof("Successfully archived %d messages", result.RowsAffected)
	return nil
}

// GetOfflineMessagesByUserID 根据用户ID获取离线消息
func (r *OfflineMessageRepo) GetOfflineMessagesByUserID(ctx context.Context, userID string, lastMessageId string) ([]*bo.OfflineMessage, error) {
	var messages []po.OfflineMessage

	query := r.data.db.WithContext(ctx).
		Where("to_user_id = ?", userID).
		Where("status = ?", po.MessageStatusPending).
		Where("msg_id > ?", lastMessageId).
		Order("msg_id ASC").
		Limit(bo.BatchArchiveSize)

	if err := query.Find(&messages).Error; err != nil {
		r.log.WithContext(ctx).Errorf("Failed to get offline messages for user %s: %v", userID, err)
		return nil, err
	}

	// 将PO转换为BO
	result := make([]*bo.OfflineMessage, len(messages))
	for i, msg := range messages {
		result[i] = msg.ToBo()
	}

	return result, nil
}

// MarkMessagesAsDelivered 标记消息为已送达
func (r *OfflineMessageRepo) MarkMessagesAsDelivered(ctx context.Context, messageIDs []string) error {
	if len(messageIDs) == 0 {
		return nil
	}

	result := r.data.db.WithContext(ctx).
		Model(&po.OfflineMessage{}).
		Where("msg_id IN ?", messageIDs).
		Updates(map[string]interface{}{
			"status":     po.MessageStatusDelivered,
			"updated_at": time.Now(),
		})

	if result.Error != nil {
		r.log.WithContext(ctx).Errorf("Failed to mark messages as delivered: %v", result.Error)
		return result.Error
	}

	r.log.WithContext(ctx).Infof("Marked %d messages as delivered", result.RowsAffected)
	return nil
}
