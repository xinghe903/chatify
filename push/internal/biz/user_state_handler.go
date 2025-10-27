package biz

import (
	im_v1 "api/im/v1"
	"context"
	"encoding/json"
	"errors"
	"push/internal/biz/bo"

	"github.com/go-kratos/kratos/v2/log"
)

var ErrUserStateInvalid = errors.New("user state invalid")

type MessageHandler func(ctx context.Context, key string, value []byte) error

type Consumer interface {
	Start(ctx context.Context, handler MessageHandler)
}

type UserStateHandler struct {
	log         *log.Helper
	offlineRepo OfflineRepo
	manager     AccessNodeManager
	consumer    Consumer
}

func NewUserStateHandler(
	logger log.Logger,
	offlineRepo OfflineRepo,
	manager AccessNodeManager,
	consumer Consumer,
) (*UserStateHandler, func()) {
	handle := &UserStateHandler{
		log:         log.NewHelper(logger),
		offlineRepo: offlineRepo,
		manager:     manager,
		consumer:    consumer,
	}
	ctx, cancel := context.WithCancelCause(context.TODO())
	handle.consumer.Start(ctx, handle.Handle())
	return handle, func() { cancel(errors.New("consumer context canceled")) }
}

func (h *UserStateHandler) Handle() MessageHandler {
	return func(ctx context.Context, key string, value []byte) error {
		var userState bo.UserStateMessage
		if err := json.Unmarshal(value, &userState); err != nil {
			h.log.WithContext(ctx).Errorf("consumer kafka message json unmarshal error: %v", err)
			return err
		}
		switch userState.State {
		case bo.UserStateOnline:
			return h.UserOnline(ctx, &userState)
		case bo.UserStateOffline:
			return h.UserOffline(ctx, &userState)
		default:
			return ErrUserStateInvalid
		}
	}
}

func (h *UserStateHandler) UserOnline(ctx context.Context, userState *bo.UserStateMessage) error {
	// TODO. The maximum length of messages should be taken into consideration.
	messages, err := h.offlineRepo.RetrieveOfflineMessages(ctx, userState.UserID)
	if err != nil {
		return err
	}
	var messagesToSend []*im_v1.BaseMessage
	for _, message := range messages {
		messagesToSend = append(messagesToSend, message.ToBaseMessage())
	}
	h.log.WithContext(ctx).Infof("Send offline message to user: %v. message size=%d", userState.UserID, len(messagesToSend))
	successIds, err := h.manager.SendToUser(ctx, userState.ConnectionId, messagesToSend)
	if err != nil {
		return err
	}
	if err = h.offlineRepo.AcknowledgeMessages(ctx, userState.UserID, successIds); err != nil {
		return err
	}
	return nil
}

func (h *UserStateHandler) UserOffline(ctx context.Context, userState *bo.UserStateMessage) error {
	return nil
}
