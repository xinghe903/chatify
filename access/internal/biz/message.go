package biz

import (
	"context"

	im_v1 "github.com/xinghe903/chatify/api/im/v1"

	"github.com/go-kratos/kratos/v2/log"
)

type Message struct {
	log        *log.Helper
	mqProducer MqProducer
}

func NewMessage(logger log.Logger, producer MqProducer) *Message {
	return &Message{
		log:        log.NewHelper(logger),
		mqProducer: producer,
	}
}

func (m *Message) DispatchMessage(ctx context.Context, message *im_v1.BaseMessage) error {
	return m.mqProducer.SendMessageWithUserMessage(ctx, message)
}
