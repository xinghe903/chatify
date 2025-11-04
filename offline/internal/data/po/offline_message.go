package po

import (
	"offline/internal/biz/bo"
	"strings"

	"github.com/xinghe903/chatify/pkg/model"

	"gorm.io/gorm"
)

// 消息状态枚举
const (
	// MessageStatusPending 待处理
	MessageStatusPending MessageStatus = "pending"
	// MessageStatusArchived 已归档
	MessageStatusArchived MessageStatus = "archived"
	// MessageStatusDelivered 已送达
	MessageStatusDelivered MessageStatus = "delivered"
)

// MessageStatus 消息状态类型
type MessageStatus string

// OfflineMessage 离线消息实体类
// 用于存储用户的离线消息
// 数据库表名: chatify_offline_message
type OfflineMessage struct {
	model.BaseModel
	MsgID       string        `json:"msg_id" gorm:"index:idx_msg_id,unique"`
	MessageType int32         `json:"message_type"`
	FromUserID  string        `json:"from_user_id" gorm:"index:idx_from_user"`
	TargetType  int32         `json:"target_type"`
	ToUserID    string        `json:"to_user_id" gorm:"index:idx_to_user,priority:1"`
	Content     []byte        `json:"content" gorm:"type:text"`
	Timestamp   int64         `json:"timestamp" gorm:"index:idx_timestamp"`
	ExpireTime  int64         `json:"expire_time" gorm:"index:idx_expire_time"`
	ContentID   string        `json:"content_id" gorm:"index:idx_content_id"`
	TaskID      string        `json:"task_id" gorm:"index:idx_task_id"`
	Status      MessageStatus `json:"status" gorm:"type:varchar(20);index:idx_status"`
	Description string        `json:"description" gorm:"type:varchar(255)"`
}

func NewOfflineMessageFromBo(boMsg *bo.OfflineMessage) *OfflineMessage {
	return &OfflineMessage{
		BaseModel: model.BaseModel{
			ID: boMsg.ID,
		},
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
		Status:      MessageStatus(boMsg.Status),
		Description: boMsg.Description,
	}
}

// TableName 设置表名
func (OfflineMessage) TableName() string {
	return "chatify_offline_message"
}

// BeforeCreate GORM钩子，创建前的处理
func (om *OfflineMessage) BeforeCreate(tx *gorm.DB) error {
	if !strings.HasPrefix(om.ID, "omid") {
		// offline message id prefix
		om.ID = "omid" + om.ID
	}
	return nil
}

func (om *OfflineMessage) ToBo() *bo.OfflineMessage {
	return &bo.OfflineMessage{
		ID:          om.ID,
		MsgID:       om.MsgID,
		MessageType: om.MessageType,
		FromUserID:  om.FromUserID,
		TargetType:  om.TargetType,
		ToUserID:    om.ToUserID,
		Content:     om.Content,
		Timestamp:   om.Timestamp,
		ExpireTime:  om.ExpireTime,
		ContentID:   om.ContentID,
		TaskID:      om.TaskID,
		Status:      bo.MessageStatus(om.Status),
		Description: om.Description,
	}
}
