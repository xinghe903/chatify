package bo

import im_v1 "api/im/v1"

const (
	MaxMessageCount int = 1000
)

// 消息状态枚举
// 定义了消息在推送过程中的状态
type MessageStatus string

const (
	// MessageStatusPending 待发送 - 消息创建但未开始发送流程或发送失败
	MessageStatusPending MessageStatus = "pending"
	// MessageStatusSent 发送成功 - 消息已通过任一方式成功送达
	MessageStatusSent MessageStatus = "sent"
)

// Message 消息业务对象
// 用于在业务层(biz)和数据层(data)之间传递消息数据
// 这是一个抽象层，避免在业务层直接操作数据层的实体

type Message struct {
	ID          string        `json:"id"`
	MsgID       string        `json:"msg_id"`
	MessageType int32         `json:"message_type"`
	FromUserID  string        `json:"from_user_id"`
	TargetType  int32         `json:"target_type"`
	ToUserID    string        `json:"to_user_id"`
	Content     []byte        `json:"content"`
	Timestamp   int64         `json:"timestamp"`
	ExpireTime  int64         `json:"expire_time"`
	ContentID   string        `json:"content_id"`
	TaskID      string        `json:"task_id"`
	Status      MessageStatus `json:"status"`
	Description string        `json:"description"`
	CreatedAt   int64         `json:"created_at"`
	UpdatedAt   int64         `json:"updated_at"`
}

func NewMessage(msg *im_v1.BaseMessage) *Message {
	return &Message{
		MsgID:       msg.MsgId,
		MessageType: int32(msg.MessageType),
		FromUserID:  msg.FromUserId,
		TargetType:  int32(msg.TargetType),
		ToUserID:    msg.ToUserId,
		Content:     msg.Content,
		Timestamp:   msg.Timestamp,
		ExpireTime:  msg.ExpireTime,
		ContentID:   msg.ContentId,
	}
}

func (m *Message) ToBaseMessage() *im_v1.BaseMessage {
	return &im_v1.BaseMessage{
		MsgId:       m.MsgID,
		MessageType: im_v1.MessageType(m.MessageType),
		FromUserId:  m.FromUserID,
		TargetType:  im_v1.TargetType(m.TargetType),
		ToUserId:    m.ToUserID,
		Content:     m.Content,
		Timestamp:   m.Timestamp,
		ExpireTime:  m.ExpireTime,
		ContentId:   m.ContentID,
	}
}
