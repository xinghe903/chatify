package data

import (
	"context"
	"pkg/auth"
	"push/internal/biz"
	"push/internal/biz/bo"
	"push/internal/data/po"

	"github.com/go-kratos/kratos/v2/log"
)

const batchInsertCount = 100 // 每次插入数据长度

var _ biz.MessageRepo = (*messageRepo)(nil)

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
		log:       log.NewHelper(logger),
		sonyFlake: auth.NewSonyflake(),
	}
}

// SaveMessages 批量存储消息到数据库
func (r *messageRepo) SaveMessages(ctx context.Context, boMessages []*bo.Message) error {
	if len(boMessages) == 0 {
		return nil
	}
	// 将业务对象转换为持久化对象
	poMessages := make([]*po.Message, 0, len(boMessages))
	var err error
	for _, boMsg := range boMessages {
		poMsg := &po.Message{
			MsgID:       boMsg.MsgID,
			MessageType: boMsg.MessageType,
			FromUserID:  boMsg.FromUserID,
			TargetType:  boMsg.TargetType,
			ToUserID:    boMsg.ToUserID,
			Content:     boMsg.Content,
			Timestamp:   boMsg.Timestamp,
			ExpireTime:  boMsg.ExpireTime,
			ContentID:   boMsg.ContentID,
			TaskID:      boMsg.TaskID,
			Status:      po.MessageStatus(boMsg.Status),
			Description: boMsg.Description,
		}
		// 生成ID
		if poMsg.ID, err = r.sonyFlake.GenerateBase62(); err != nil {
			r.log.WithContext(ctx).Errorf("failed to generate message ID. err=%s", err.Error())
		}
		poMessages = append(poMessages, poMsg)
	}

	if err := r.data.db.WithContext(ctx).CreateInBatches(poMessages, batchInsertCount).Error; err != nil {
		r.log.WithContext(ctx).Errorf("failed to save messages. err=%s", err.Error())
		return err
	}

	r.log.WithContext(ctx).Debugf("save messages success. count=%d", len(poMessages))
	return nil
}

func (r *messageRepo) UpdateMessageStatus(ctx context.Context, boMessages []*bo.Message) error {
	if len(boMessages) == 0 {
		return nil
	}

	// 将业务对象转换为持久化对象
	poMessages := make([]*po.Message, 0, len(boMessages))
	msgIDs := make([]string, 0, len(boMessages))
	for _, boMsg := range boMessages {
		poMsg := &po.Message{
			MsgID:       boMsg.MsgID,
			Status:      po.MessageStatus(boMsg.Status),
			Description: boMsg.Description,
		}
		poMessages = append(poMessages, poMsg)
		msgIDs = append(msgIDs, boMsg.MsgID)
	}

	// 批量更新消息状态
	tx := r.data.db.WithContext(ctx)
	for _, poMsg := range poMessages {
		if err := tx.Model(po.Message{}).
			Where("msg_id = ?", poMsg.MsgID).
			Updates(map[string]interface{}{
				"status":      poMsg.Status,
				"description": poMsg.Description,
			}).Error; err != nil {
			r.log.WithContext(ctx).Errorf("failed to update message status. msgID=%s, err=%s", poMsg.MsgID, err.Error())
			return err
		}
	}

	return nil
}
