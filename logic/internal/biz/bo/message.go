package bo

import (
	v1 "logic/api/logic/v1"
)

type Message struct {
	MessageId   string
	UserIds     []string
	GroupId     string
	Content     []byte
	ContentType string
	ExpireTime  int64
}

func NewMessage(req *v1.SendMessageRequest) *Message {
	if req == nil {
		return nil
	}
	return &Message{
		UserIds:     req.TargetUserIds,
		GroupId:     req.GroupId,
		Content:     req.Content,
		ContentType: req.ContentType,
		ExpireTime:  req.ExpireTime,
	}
}
