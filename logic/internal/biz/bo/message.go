package bo

import (
	"encoding/json"
	v1 "logic/api/logic/v1"
)

type TargetUser struct {
	UserId     string
	GroupId    string
	FromUserId string
}

type Message struct {
	MessageId   string
	Content     []byte
	ContentType string
	ExpireTime  int64
	Target      []*TargetUser
}

func NewMessage(req *v1.SendMessageRequest) *Message {
	if req == nil {
		return nil
	}
	var targets []*TargetUser
	for _, userId := range req.GetTargetUserIds() {
		targets = append(targets, &TargetUser{
			UserId:     userId,
			GroupId:    "",
			FromUserId: "xinghe",
		})
	}
	if req.GroupId != "" {
		targets = append(targets, &TargetUser{
			UserId:     "",
			GroupId:    req.GroupId,
			FromUserId: "xinghe",
		})
	}
	return &Message{
		Target:      targets,
		Content:     req.Content,
		ContentType: req.ContentType,
		ExpireTime:  req.ExpireTime,
	}
}

func (m Message) MarshalJSON() ([]byte, error) {
	type WrapMessage struct {
		MessageId   string
		Content     string
		ContentType string
		ExpireTime  int64
		Target      []*TargetUser
	}
	wm := &WrapMessage{
		MessageId:   m.MessageId,
		Content:     string(m.Content),
		ContentType: m.ContentType,
		ExpireTime:  m.ExpireTime,
		Target:      m.Target,
	}
	return json.Marshal(wm)
}

func (m *Message) ToJsonStr() string {
	json, _ := json.Marshal(m)
	return string(json)
}
