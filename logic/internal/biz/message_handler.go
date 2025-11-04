package biz

import (
	"context"
	"encoding/json"
	"errors"
	"logic/internal/biz/bo"

	im_v1 "github.com/xinghe903/chatify/api/im/v1"

	"github.com/go-kratos/kratos/v2/log"
)

var (
	ErrInvalidMessageType = errors.New("invalid message type")
	ErrInvalidTargetType  = errors.New("invalid target type")
	ErrMessageDuplicate   = errors.New("message already processed")
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

type MqProducer interface {
	SendMessageWithDataReport(ctx context.Context, message *bo.DataReport) error
}

type UserMessageHandler struct {
	log        *log.Helper
	consumer   Consumer
	dedupRepo  MessageDedupRepo
	mqProducer MqProducer
}

func NewUserMessageHandler(
	logger log.Logger,
	consumer Consumer,
	dedupRepo MessageDedupRepo,
	mqProducer MqProducer,
) (*UserMessageHandler, func()) {
	handle := &UserMessageHandler{
		log:        log.NewHelper(logger),
		consumer:   consumer,
		dedupRepo:  dedupRepo,
		mqProducer: mqProducer,
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

		// 检查消息是否已消费（使用 baseMsg.MsgId 作为唯一标识）
		if baseMsg.MsgId != "" {
			isNew, err := h.dedupRepo.CheckAndSetDedup(ctx, baseMsg.MsgId)
			if err != nil {
				h.log.WithContext(ctx).Errorf("检查消息去重失败: msgId=%s, error=%v", baseMsg.MsgId, err)
				// Redis错误时继续处理消息，避免因为Redis故障导致消息无法消费
			} else if !isNew {
				// 消息已被消费过，跳过
				h.log.WithContext(ctx).Debugf("消息已消费，跳过: msgId=%s", baseMsg.MsgId)
				return ErrMessageDuplicate
			}
		} else {
			h.log.WithContext(ctx).Warnf("消息缺少 msgId，跳过去重检查: %v", baseMsg)
		}

		// 处理消息
		switch baseMsg.MessageType {
		case im_v1.MessageType_CHAT:
			return h.chat(ctx, &baseMsg)
		case im_v1.MessageType_CONTROL:
			return h.control(ctx, &baseMsg)
		case im_v1.MessageType_DATAREPORT:
			return h.dataReport(ctx, &baseMsg)
		default:
			return ErrInvalidMessageType
		}
	}
}

// chat 处理聊天消息
func (h *UserMessageHandler) chat(ctx context.Context, baseMsg *im_v1.BaseMessage) error {
	if baseMsg.TargetType != im_v1.TargetType_USER && baseMsg.TargetType != im_v1.TargetType_GROUP {
		return ErrInvalidTargetType
	}
	h.log.WithContext(ctx).Infof("Receive chat message. baseMsg: %v", baseMsg)
	return nil
}

// dataReport 处理数据上报消息
func (h *UserMessageHandler) dataReport(ctx context.Context, baseMsg *im_v1.BaseMessage) error {
	if baseMsg.TargetType != im_v1.TargetType_SYSTEM {
		return ErrInvalidTargetType
	}
	var dataReport bo.DataReport
	if err := json.Unmarshal(baseMsg.Content, &dataReport); err != nil {
		h.log.WithContext(ctx).Errorf("data report json unmarshal error: %v", err)
		return err
	}
	if err := h.mqProducer.SendMessageWithDataReport(ctx, &dataReport); err != nil {
		h.log.WithContext(ctx).Errorf("send data report message error: %v", err)
		return err
	}
	h.log.WithContext(ctx).Infof("Receive data report message. baseMsg: %v", baseMsg)
	return nil
}

// control 处理控制类消息
func (h *UserMessageHandler) control(ctx context.Context, baseMsg *im_v1.BaseMessage) error {
	if baseMsg.TargetType != im_v1.TargetType_SYSTEM {
		return ErrInvalidTargetType
	}
	h.log.WithContext(ctx).Infof("Receive control message. baseMsg: %v", baseMsg)
	return nil
}
