package biz

import (
	im_v1 "api/im/v1"
	offline_v1 "api/offline/v1"
	"context"
	"encoding/json"
	"errors"
	"push/internal/biz/bo"

	"github.com/go-kratos/kratos/v2/log"
)

var (
	ErrUserStateInvalid = errors.New("user state invalid")
	ErrMessageDuplicate = errors.New("message already processed")
)

type MessageHandler func(ctx context.Context, key string, value []byte) error

type Consumer interface {
	Start(ctx context.Context, handler MessageHandler)
}

// MessageDedupRepo 消息去重仓库接口
type MessageDedupRepo interface {
	// CheckAndSetDedup 检查消息是否已消费，如果未消费则标记为已消费
	// 返回true表示消息未被消费过，false表示消息已被消费过
	CheckAndSetDedup(ctx context.Context, msgId string) (bool, error)
}

type UserStateHandler struct {
	log         *log.Helper
	offlineRepo OfflineRepo
	manager     AccessNodeManager
	consumer    Consumer
	dedupRepo   MessageDedupRepo
}

func NewUserStateHandler(
	logger log.Logger,
	offlineRepo OfflineRepo,
	manager AccessNodeManager,
	consumer Consumer,
	dedupRepo MessageDedupRepo,
) (*UserStateHandler, func()) {
	handle := &UserStateHandler{
		log:         log.NewHelper(logger),
		offlineRepo: offlineRepo,
		manager:     manager,
		consumer:    consumer,
		dedupRepo:   dedupRepo,
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

		// 检查消息是否已消费（使用 userState.Id 作为唯一标识）
		if userState.Id != "" {
			isNew, err := h.dedupRepo.CheckAndSetDedup(ctx, userState.Id)
			if err != nil {
				h.log.WithContext(ctx).Errorf("检查消息去重失败: id=%s, error=%v", userState.Id, err)
				// Redis错误时继续处理消息，避免因为Redis故障导致消息无法消费
			} else if !isNew {
				// 消息已被消费过，跳过
				h.log.WithContext(ctx).Debugf("消息已消费，跳过: id=%s", userState.Id)
				return ErrMessageDuplicate
			}
		} else {
			h.log.WithContext(ctx).Warnf("消息缺少 id，跳过去重检查: %v", userState)
		}

		// 处理消息
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
		h.log.WithContext(ctx).Errorf("failed to retrieve offline messages. userID=%s, error=%s", userState.UserID, err.Error())
		return offline_v1.ErrorGetOfflineMessageFailed("data get offline message failed")
	}
	var messagesToSend []*im_v1.BaseMessage
	for _, message := range messages {
		messagesToSend = append(messagesToSend, message.ToBaseMessage())
	}
	h.log.WithContext(ctx).Infof("Send offline message to user: %v. message size=%d", userState.UserID, len(messagesToSend))
	successIds, err := h.manager.SendToUser(ctx, userState.ConnectionId, messagesToSend)
	if err != nil {
		h.log.WithContext(ctx).Errorf("failed to send message to access node. userID=%s, error=%s",
			userState.UserID, err.Error())
		return errors.Join(err, errors.New("failed to send message to access node"))
	}
	if err = h.offlineRepo.AcknowledgeMessages(ctx, userState.UserID, successIds); err != nil {
		h.log.WithContext(ctx).Errorf("failed to acknowledge messages. userID=%s, error=%s",
			userState.UserID, err.Error())
		return offline_v1.ErrorMarkMessageAsDeliveredFailed("data acknowledge message failed")
	}
	return nil
}

func (h *UserStateHandler) UserOffline(ctx context.Context, userState *bo.UserStateMessage) error {
	return nil
}
