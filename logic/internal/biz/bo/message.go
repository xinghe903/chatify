package bo

import (
	im_v1 "github.com/xinghe903/chatify/api/im/v1"
	v1 "github.com/xinghe903/chatify/api/logic/v1"
)

const (
	MaxTargetUsers = 1000
)

// MessageType 消息类型枚举
type MessageType int32

const (
	MessageType_TEXT     MessageType = 1   // 文本消息
	MessageType_IMAGE    MessageType = 2   // 图片消息
	MessageType_VOICE    MessageType = 3   // 语音消息
	MessageType_VIDEO    MessageType = 4   // 视频消息
	MessageType_FILE     MessageType = 5   // 文件消息
	MessageType_LOCATION MessageType = 6   // 位置消息
	MessageType_CUSTOM   MessageType = 10  // 自定义消息
	MessageType_NOTICE   MessageType = 100 // 公告消息
	MessageType_INFORM   MessageType = 101 // 通知消息
)

// TargetType 目标类型枚举
type TargetType int32

const (
	TargetType_USER  TargetType = 1 // 用户
	TargetType_GROUP TargetType = 2 // 群组
)

// Message 消息业务对象
type Message struct {
	MsgId       string      `protobuf:"bytes,1,opt,name=msg_id,json=msgId,proto3" json:"msg_id,omitempty"`
	MessageType MessageType `protobuf:"varint,2,opt,name=message_type,json=messageType,proto3,enum=im.v1.MessageType" json:"message_type,omitempty"`
	FromUserId  string      `protobuf:"bytes,3,opt,name=from_user_id,json=fromUserId,proto3" json:"from_user_id,omitempty"`
	TargetType  TargetType  `protobuf:"varint,4,opt,name=target_type,json=targetType,proto3,enum=im.v1.TargetType" json:"target_type,omitempty"`
	ToUserId    string      `protobuf:"bytes,5,opt,name=to_user_id,json=toUserId,proto3" json:"to_user_id,omitempty"`
	Content     []byte      `protobuf:"bytes,6,opt,name=content,proto3" json:"content,omitempty"`
	Timestamp   int64       `protobuf:"varint,7,opt,name=timestamp,proto3" json:"timestamp,omitempty"`
	ExpireTime  int64       `protobuf:"varint,10,opt,name=expire_time,json=expireTime,proto3" json:"expire_time,omitempty"`
	ContentId   string      `protobuf:"bytes,11,opt,name=content_id,json=contentId,proto3" json:"content_id,omitempty"`
}

// NewMessage 根据SystemPushRequest创建Message对象
func NewMessage(req *v1.SystemPushRequest) *Message {
	var messageType MessageType
	switch req.PushType {
	case v1.PushType_NOTICE:
		messageType = MessageType_NOTICE
	case v1.PushType_INFORM:
		messageType = MessageType_INFORM
	default:
		messageType = MessageType_CUSTOM
	}

	return &Message{
		Content:     req.Content,
		MessageType: messageType,
		FromUserId:  req.FromUserId,
		TargetType:  TargetType_USER,
		Timestamp:   req.Timestamp,
		ExpireTime:  req.Timestamp,
		ContentId:   req.ContentId,
	}
}

// NewMessagesByUserIDs 根据用户ID列表创建多条消息
func NewMessagesByUserIDs(req *v1.SystemPushRequest) []*Message {
	messages := make([]*Message, 0, len(req.ToUserIds))

	for _, userId := range req.ToUserIds {
		msg := NewMessage(req)
		msg.ToUserId = userId
		messages = append(messages, msg)
	}

	return messages
}

// ToBaseMessage 将业务消息转换为proto定义的BaseMessage
func (m *Message) ToBaseMessage() *im_v1.BaseMessage {
	return &im_v1.BaseMessage{
		MsgId:       m.MsgId,
		FromUserId:  m.FromUserId,
		ToUserId:    m.ToUserId,
		Content:     m.Content,
		Timestamp:   m.Timestamp,
		ExpireTime:  m.ExpireTime,
		ContentId:   m.ContentId,
		MessageType: im_v1.MessageType(m.MessageType),
		TargetType:  im_v1.TargetType(m.TargetType),
	}
}

// var MetricBizRequests otlpmetric.Int64Counter

// initBiz := func() {
// 	var err error
// 	MetricBizRequests, err = metrics.DefaultRequestsCounter(monitoring.Meter, "biz_requests")
// 	if err != nil {
// 		panic(err)
// 	}

// }
