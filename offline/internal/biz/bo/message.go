package bo

const (
	BatchArchiveSize int = 1000 // 批量归档消息大小最大限制
)

// 消息状态枚举
type MessageStatus string

const (
	// MessageStatusPending 待处理
	MessageStatusPending MessageStatus = "pending"
	// MessageStatusArchived 已归档
	MessageStatusArchived MessageStatus = "archived"
	// MessageStatusDelivered 已送达
	MessageStatusDelivered MessageStatus = "delivered"
)

// OfflineMessage 离线消息业务对象
// 用于在业务层(biz)和数据层(data)之间传递消息数据
type OfflineMessage struct {
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
