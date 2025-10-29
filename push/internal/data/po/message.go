package po

import (
	"pkg/model"
	"strings"

	"gorm.io/gorm"
)

// 消息状态枚举
// 定义了消息在推送过程中的状态
const (
	// MessageStatusPending 待发送 - 消息创建但未开始发送流程或发送失败
	MessageStatusPending MessageStatus = "pending"
	// MessageStatusSent 发送成功 - 消息已通过任一方式成功送达
	MessageStatusSent MessageStatus = "sent"
)

// MessageStatus 消息状态类型
// 用于类型安全的消息状态表示
type MessageStatus string

// Message 消息实体类
// 用于存储推送的消息
// 数据库表名: messages

type Message struct {
	model.BaseModel
	MsgID       string        `json:"msg_id" gorm:"index:idx_msg_id"`
	MessageType int32         `json:"message_type"`
	FromUserID  string        `json:"from_user_id" gorm:"index:idx_from_user"`
	TargetType  int32         `json:"target_type"`
	ToUserID    string        `json:"to_user_id" gorm:"index:idx_to_user"`
	Content     []byte        `json:"content"`
	Timestamp   int64         `json:"timestamp"`
	ExpireTime  int64         `json:"expire_time" gorm:"index:idx_expire_time"`
	ContentID   string        `json:"content_id" gorm:"index:idx_content_id"`
	TaskID      string        `json:"task_id" gorm:"index:idx_task_id"`
	Status      MessageStatus `json:"status" gorm:"type:varchar(20);index:idx_status"`
	Description string        `json:"description"`
}

// TableName 设置表名
func (Message) TableName() string {
	return "chatify_push_message"
}

func (s *Message) BeforeCreate(tx *gorm.DB) error {
	if !strings.HasPrefix(s.ID, "pmid") {
		// push message id prefix
		s.ID = "pmid" + s.ID
	}
	return nil
}
