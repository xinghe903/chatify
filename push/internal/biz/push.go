package biz

import (
	access_v1 "api/access/v1"
	im_v1 "api/im/v1"
	"context"
	"errors"
	"push/internal/biz/bo"
	"push/internal/data/po"
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
	SaveMessages(ctx context.Context, messages []*po.Message) error
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
}

// NewPush
func NewPush(logger log.Logger, session SessionRepo, message MessageRepo, manager AccessNodeManager) *Push {
	return &Push{
		log:         log.NewHelper(logger),
		session:     session,
		messageRepo: message,
		manager:     manager,
	}
}

// PushToUser 推送消息到用户
func (p *Push) PushToUser(ctx context.Context, taskID string, messages []*im_v1.BaseMessage) error {
	p.log.WithContext(ctx).Debug("PushToUser", "taskID", taskID, "messageCount", len(messages))
	var dbMessages []*po.Message
	// 创建一个map，用于按access服务实例ID进行分组
	accessMessageGroups := make(map[string]*access_v1.PushMessageRequest)
	// 遍历所有消息，进行分组处理
	for _, msg := range messages {
		// 忽略空的用户ID
		if msg.ToUserId == "" {
			p.log.WithContext(ctx).Warn("ignore message with empty to_user_id", "msg_id", msg.MsgId)
			continue
		}
		// 设置分布式锁，防止重复发送
		// lockKey := lockPrefix + msg.MsgId
		// lockValue := "processing"

		// 查询用户的完整会话信息
		session, err := p.session.GetSession(ctx, msg.ToUserId)
		if err != nil {
			p.log.WithContext(ctx).Error("failed to get user session", "to_user_id", msg.ToUserId, "error", err.Error())
			// 继续处理其他消息
			continue
		}

		// 如果用户没有连接，记录日志后继续处理其他消息
		if session == nil || session.ConnectionId == "" {
			p.log.WithContext(ctx).Debug("user has no active connections", "to_user_id", msg.ToUserId)
			// 保存消息到数据库，但不发送
			dbMessage := &po.Message{
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
				Status:      po.MessageStatusPending,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}
			dbMessages = append(dbMessages, dbMessage)
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
	var successMsgIDs []string
	for connectId, messages := range accessMessageGroups {
		// 获取该用户当前连接的 access 节点
		successIds, err := p.manager.SendToUser(ctx, connectId, messages.Message)
		if err != nil {
			p.log.WithContext(ctx).Error("failed to send message to access node", "connectId", connectId, "error", err.Error())
			// 继续处理其他消息
			continue
		}
		successMsgIDs = append(successMsgIDs, successIds...)
	}

	p.log.WithContext(ctx).Info("PushToUser completed", "taskID", taskID, "messageCount", len(messages), "savedCount", len(dbMessages))
	return nil
}
