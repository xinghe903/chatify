package data

import (
	"context"
	"pkg/auth"
	"push/internal/biz"
	"push/internal/data/po"

	"github.com/go-kratos/kratos/v2/log"
)

// messageRepo 消息仓库实现

type messageRepo struct {
	data      *Data
	log       *log.Helper
	sonyFlake *auth.Sonyflake
}

// NewMessageRepo 创建消息仓库实例
func NewMessageRepo(data *Data, logger log.Logger) biz.MessageRepo {
	return &messageRepo{
		data:      data,
		log:       log.NewHelper(log.With(logger, "module", "data/message")),
		sonyFlake: auth.NewSonyflake(),
	}
}

// SaveMessages 批量存储消息到数据库
func (r *messageRepo) SaveMessages(ctx context.Context, messages []*po.Message) error {
	if len(messages) == 0 {
		return nil
	}
	var err error
	for _, message := range messages {
		if message.ID, err = r.sonyFlake.GenerateBase62(); err != nil {
			r.log.WithContext(ctx).Errorf("failed to generate message ID. err=%s", err.Error())
		}
	}

	// 使用事务批量保存消息
	tx := r.data.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		r.log.WithContext(ctx).Errorf("failed to begin transaction. err=%s", tx.Error.Error())
		return tx.Error
	}

	// 分批保存消息，避免单次插入过多数据
	const batchSize = 1000
	for i := 0; i < len(messages); i += batchSize {
		end := i + batchSize
		if end > len(messages) {
			end = len(messages)
		}

		batch := messages[i:end]
		if err := tx.Create(batch).Error; err != nil {
			tx.Rollback()
			r.log.WithContext(ctx).Errorf("failed to save messages. err=%s", err.Error())
			return err
		}
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		r.log.WithContext(ctx).Errorf("failed to commit transaction. err=%s", err.Error())
		tx.Rollback()
		return err
	}

	r.log.WithContext(ctx).Debugf("save messages success. count=%d", len(messages))
	return nil
}
