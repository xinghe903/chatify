package biz

import (
	access_v1 "api/access/v1"
	im_v1 "api/im/v1"
	v1 "api/push/v1"
	"context"
	"errors"
	"push/internal/biz/bo"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

const (
	// 分布式锁的键前缀
	lockPrefix = "chatify:push:message:lock:"
	// 分布式锁过期时间（10分钟）
	lockExpiration = 10 * time.Minute
)

var (
	ErrPartialSuccess = errors.New("partial message success")
	ErrAllFailed      = errors.New("all message failed")

	// ErrPending 消息未发送原因
	ErrPendingSendFailed    = errors.New("send failed")
	ErrPendingSessionStatus = errors.New("session status error")
	ErrPendingUserOffline   = errors.New("user offline")
	ErrPendingUserInvalid   = errors.New("user invalid")
)

type SessionRepo interface {
	SetSession(ctx context.Context, session *bo.Session) error
	BatchClearSession(ctx context.Context, uids []string) error
	GetSession(ctx context.Context, uid string) (*bo.Session, error)
	ClearSession(ctx context.Context, uid string) error
}

// MessageRepo 消息仓库接口
type MessageRepo interface {
	// 批量存储消息
	SaveMessages(ctx context.Context, messages []*bo.Message) error
	UpdateMessageStatus(ctx context.Context, messages []*bo.Message) error
}

type OfflineRepo interface {
	ArchiveMessages(ctx context.Context, taskId string, messages []*bo.Message) error
	RetrieveOfflineMessages(ctx context.Context, userID string) ([]*bo.Message, error)
	AcknowledgeMessages(ctx context.Context, userId string, messageIds []string) error
}

type AccessNodeManager interface {
	SendToUser(ctx context.Context, connectId string, messages []*im_v1.BaseMessage) ([]string, error)
	Close() error
}

type Push struct {
	log         *log.Helper
	session     SessionRepo
	messageRepo MessageRepo
	manager     AccessNodeManager
	offlineRepo OfflineRepo
}

// NewPush
func NewPush(logger log.Logger,
	session SessionRepo,
	message MessageRepo,
	manager AccessNodeManager,
	offline OfflineRepo,
) *Push {
	return &Push{
		log:         log.NewHelper(logger),
		session:     session,
		messageRepo: message,
		manager:     manager,
		offlineRepo: offline,
	}
}

// PushToUser 推送消息到用户
func (p *Push) PushToUser(ctx context.Context, taskID string, messages []*im_v1.BaseMessage) error {
	if len(messages) > bo.MaxMessageCount {
		return v1.ErrorTooManyMessages("message count=%d, but max is %d", len(messages), bo.MaxMessageCount)
	}
	// 创建数据库消息
	if err := p.saveMessages(ctx, taskID, messages); err != nil {
		p.log.WithContext(ctx).Errorf("failed to save messages. taskID=%s, error=%s", taskID, err.Error())
		return v1.ErrorSaveMessageFailed("data save message failed")
	}
	// 把所有消息都设置为发送失败
	msgSendMask := make(map[string]error, len(messages))
	for _, msg := range messages {
		msgSendMask[msg.MsgId] = ErrPendingSendFailed
	}
	// 将消息按连接ID进行分组
	accessMessageGroups := p.groupMessageByConnectId(ctx, messages, msgSendMask)
	// 发送消息, 并记录发送成功消息结果
	successMsgIDs := p.sendMessageToAccessNode(ctx, accessMessageGroups, msgSendMask)
	// 归档离线消息
	if err := p.archiveOfflineMessages(ctx, taskID, msgSendMask, messages); err != nil {
		p.log.WithContext(ctx).Errorf("failed to archive offline messages. taskID=%s, error=%s", taskID, err.Error())
		return v1.ErrorArchiveOfflineMessageFailed("data archive message failed")
	}
	// 更新消息状态
	if err := p.updateMessageStatus(ctx, msgSendMask); err != nil {
		p.log.WithContext(ctx).Errorf("failed to update message status. taskID=%s, error=%s", taskID, err.Error())
		return v1.ErrorUpdateMessageStatusFailed("data update message status failed")
	}
	p.log.WithContext(ctx).Debugf("PushToUser completed. taskID=%s, messageCount=%d, successCount=%d", taskID, len(messages), len(successMsgIDs))
	return nil
}

// saveMessages 保存消息到数据库
func (p *Push) saveMessages(ctx context.Context, taskID string, messages []*im_v1.BaseMessage) error {
	boMessages := make([]*bo.Message, 0, len(messages))
	for _, msg := range messages {
		boMessage := &bo.Message{
			MsgID:       msg.MsgId,
			MessageType: int32(msg.MessageType),
			FromUserID:  msg.FromUserId,
			TargetType:  int32(msg.TargetType),
			ToUserID:    msg.ToUserId,
			Content:     msg.Content,
			Timestamp:   msg.Timestamp,
			ExpireTime:  msg.ExpireTime,
			ContentID:   msg.ContentId,
			TaskID:      taskID,
			Status:      bo.MessageStatusPending,
		}
		boMessages = append(boMessages, boMessage)
	}
	if err := p.messageRepo.SaveMessages(ctx, boMessages); err != nil {
		return err
	}
	return nil
}

// groupMessageByConnectId 按连接ID进行消息分组，并对无法分组的消息进行错误处理
func (p *Push) groupMessageByConnectId(ctx context.Context,
	messages []*im_v1.BaseMessage,
	msgSendMask map[string]error,
) map[string]*access_v1.PushMessageRequest {
	// 创建一个map，用于按access服务实例ID进行分组
	accessMessageGroups := make(map[string]*access_v1.PushMessageRequest)
	// 遍历所有消息，进行分组处理
	for _, msg := range messages {
		// 忽略空的用户ID
		if msg.ToUserId == "" {
			msgSendMask[msg.MsgId] = ErrPendingUserInvalid
			p.log.WithContext(ctx).Warnf("ignore message with empty to_user_id msg_id=%s", msg.MsgId)
			continue
		}
		// 设置分布式锁，防止重复发送
		// lockKey := lockPrefix + msg.MsgId
		// lockValue := "processing"

		// 查询用户的完整会话信息
		session, err := p.session.GetSession(ctx, msg.ToUserId)
		if err != nil {
			msgSendMask[msg.MsgId] = ErrPendingSessionStatus
			p.log.WithContext(ctx).Errorf("failed to get user session. to_user_id=%s, error=%s", msg.ToUserId, err.Error())
			// 继续处理其他消息
			continue
		}

		// 如果用户没有连接，记录日志后继续处理其他消息
		if session == nil || session.ConnectionId == "" {
			msgSendMask[msg.MsgId] = ErrPendingUserOffline
			p.log.WithContext(ctx).Debugf("user has no active connections to_user_id=%s", msg.ToUserId)
			continue
		}

		if _, ok := accessMessageGroups[session.ConnectionId]; !ok {
			accessMessageGroups[session.ConnectionId] = &access_v1.PushMessageRequest{
				ConnectionId: session.ConnectionId,
				Message:      []*im_v1.BaseMessage{},
			}
		}
		accessMessageGroups[session.ConnectionId].Message = append(accessMessageGroups[session.ConnectionId].Message, msg)
	}
	return accessMessageGroups
}

// sendMessageToAccessNode 发送消息到 access 节点
// 返回发送成功的消息ID列表
func (p *Push) sendMessageToAccessNode(ctx context.Context,
	accessMessageGroups map[string]*access_v1.PushMessageRequest,
	msgSendMask map[string]error) []string {
	var successMsgIDs []string
	for connectId, messages := range accessMessageGroups {
		// 获取该用户当前连接的 access 节点
		successIds, err := p.manager.SendToUser(ctx, connectId, messages.Message)
		if err != nil {
			// 发送失败，不需要额外记录错误。因为所有消息都是默认发送失败的情况
			p.log.WithContext(ctx).Errorf("failed to send message to access node. connectId=%s, error=%s",
				connectId, err.Error())
			continue
		}
		// 标记发送成功标识
		for _, msgId := range successIds {
			msgSendMask[msgId] = nil
		}
		successMsgIDs = append(successMsgIDs, successIds...)
	}
	return successMsgIDs
}

func (p *Push) archiveOfflineMessages(ctx context.Context,
	taskID string,
	msgSendMask map[string]error,
	messages []*im_v1.BaseMessage,
) error {
	id2Msg := make(map[string]*im_v1.BaseMessage, len(messages))
	for _, msg := range messages {
		id2Msg[msg.MsgId] = msg
	}
	offlineMsgIds := make([]string, 0, len(messages))
	for id, e := range msgSendMask {
		if errors.Is(e, ErrPendingUserOffline) {
			offlineMsgIds = append(offlineMsgIds, id)
		}
	}
	if len(offlineMsgIds) == 0 {
		return nil
	}
	offlineMsg := make([]*bo.Message, 0, len(offlineMsgIds))
	for _, id := range offlineMsgIds {
		msg := id2Msg[id]
		offlineMsg = append(offlineMsg, &bo.Message{
			MsgID:       id,
			Content:     msg.Content,
			ContentID:   msg.ContentId,
			TaskID:      taskID,
			Timestamp:   msg.Timestamp,
			ExpireTime:  msg.ExpireTime,
			ToUserID:    msg.ToUserId,
			FromUserID:  msg.FromUserId,
			Status:      bo.MessageStatusPending,
			Description: ErrPendingUserOffline.Error(),
			MessageType: int32(msg.MessageType),
			TargetType:  int32(msg.TargetType),
		})
	}
	if err := p.offlineRepo.ArchiveMessages(ctx, taskID, offlineMsg); err != nil {
		return err
	}
	return nil
}

// updateMessageStatus 更新消息状态
func (p *Push) updateMessageStatus(ctx context.Context, msgSendMask map[string]error) error {
	boMessages := make([]*bo.Message, 0, len(msgSendMask))
	for msgId, reason := range msgSendMask {
		msg := &bo.Message{
			MsgID:       msgId,
			Status:      bo.MessageStatusSent,
			Description: "",
		}
		if reason != nil {
			msg.Status = bo.MessageStatusPending
			msg.Description = reason.Error()
		}
		boMessages = append(boMessages, msg)
	}
	if err := p.messageRepo.UpdateMessageStatus(ctx, boMessages); err != nil {
		return err
	}
	return nil
}
