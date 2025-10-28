package biz

import (
	im_v1 "api/im/v1"
	"context"
	"encoding/json"
	"errors"

	"github.com/go-kratos/kratos/v2/log"
)

var (
	ErrInvalidMessageType = errors.New("invalid message type")
	ErrInvalidTargetType  = errors.New("invalid target type")
)

type MessageHandler func(ctx context.Context, key string, value []byte) error

type Consumer interface {
	Start(ctx context.Context, handler MessageHandler)
}

type UserMessageHandler struct {
	log      *log.Helper
	consumer Consumer
}

func NewUserMessageHandler(
	logger log.Logger,
	consumer Consumer,
) (*UserMessageHandler, func()) {
	handle := &UserMessageHandler{
		log:      log.NewHelper(logger),
		consumer: consumer,
	}
	ctx, cancel := context.WithCancelCause(context.TODO())
	handle.consumer.Start(ctx, handle.Handle())
	return handle, func() { cancel(errors.New("consumer context canceled")) }
}

func (h *UserMessageHandler) Handle() MessageHandler {
	return func(ctx context.Context, key string, value []byte) error {
		var baseMsg im_v1.BaseMessage
		if err := json.Unmarshal(value, &baseMsg); err != nil {
			h.log.WithContext(ctx).Errorf("consumer kafka message json unmarshal error: %v", err)
			return err
		}
		switch baseMsg.MessageType {
		case im_v1.MessageType_CHAT:
			return h.chat(&baseMsg)
		case im_v1.MessageType_CONTROL:
			return h.control(&baseMsg)
		case im_v1.MessageType_DATAREPORT:
			return h.dataReport(&baseMsg)
		default:
			return ErrInvalidMessageType
		}
	}
}

// chat 处理聊天消息
func (h *UserMessageHandler) chat(baseMsg *im_v1.BaseMessage) error {
	if baseMsg.TargetType != im_v1.TargetType_USER && baseMsg.TargetType != im_v1.TargetType_GROUP {
		return ErrInvalidTargetType
	}
	return nil
}

// dataReport 处理数据上报消息
func (h *UserMessageHandler) dataReport(baseMsg *im_v1.BaseMessage) error {
	if baseMsg.TargetType != im_v1.TargetType_SYSTEM {
		return ErrInvalidTargetType
	}
	return nil
}

// control 处理控制类消息
func (h *UserMessageHandler) control(baseMsg *im_v1.BaseMessage) error {
	if baseMsg.TargetType != im_v1.TargetType_SYSTEM {
		return ErrInvalidTargetType
	}
	return nil
}
