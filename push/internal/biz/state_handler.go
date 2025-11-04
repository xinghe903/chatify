package biz

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/xinghe903/chatify/push/internal/biz/bo"

	im_v1 "github.com/xinghe903/chatify/api/im/v1"
	offline_v1 "github.com/xinghe903/chatify/api/offline/v1"

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
	msgSize := 0
	var messagesToSend []*im_v1.BaseMessage
	fn := func(latestId string) error {
		messages, err := h.offlineRepo.RetrieveOfflineMessages(ctx, userState.UserID, latestId)
		if err != nil {
			h.log.WithContext(ctx).Errorf("failed to retrieve offline messages. userID=%s, error=%s", userState.UserID, err.Error())
			return offline_v1.ErrorGetOfflineMessageFailed("data get offline message failed")
		}
		msgSize = len(messages)
		for _, message := range messages {
			messagesToSend = append(messagesToSend, message.ToBaseMessage())
		}
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
	latestId := ""
	for {
		if err := fn(latestId); err != nil {
			return err
		}
		h.log.WithContext(ctx).Debugf("Send offline message to user: %v. message size=%d, latestId=%s", userState.UserID, msgSize, latestId)
		if msgSize != bo.MaxMessageCount {
			break
		}
		latestId = messagesToSend[len(messagesToSend)-1].MsgId
		messagesToSend = []*im_v1.BaseMessage{}
	}
	return nil
}

func (h *UserStateHandler) UserOffline(ctx context.Context, userState *bo.UserStateMessage) error {
	return nil
}
