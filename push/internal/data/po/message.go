package po

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// 消息状态枚举
// 定义了消息在推送过程中的状态
const (
	// MessageStatusPending 待发送 - 消息创建但未开始发送流程或发送失败
	MessageStatusPending = "pending"
	// MessageStatusSent 发送成功 - 消息已通过任一方式成功送达
	MessageStatusSent = "sent"
)

// MessageStatus 消息状态类型
// 用于类型安全的消息状态表示
type MessageStatus string

// Message 消息实体类
// 用于存储推送的消息
// 数据库表名: messages

type Message struct {
	ID          string        `json:"id" gorm:"primaryKey"`
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
	Status      MessageStatus `json:"status"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// TableName 设置表名
func (Message) TableName() string {
	return "chatify_push_message"
}

func (s *Message) BeforeCreate(tx *gorm.DB) error {
	if !strings.HasPrefix(s.ID, "mid") {
		s.ID = "mid" + s.ID
	}
	return nil
}
