package biz

import (
	im_v1 "api/im/v1"
	"context"
	"encoding/json"
	"errors"

	"github.com/go-kratos/kratos/v2/log"
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
		switch baseMsg.TargetType {
		default:
			return errors.New("unknown target type")
		}
	}
}
