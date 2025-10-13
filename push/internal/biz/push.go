package biz

import (
	"context"
	"push/internal/data"
	"push/internal/data/po"
	v1 "api/push/v1"
	"api/im/v1"
	accessV1 "api/access/v1"
	"time"
	"errors"
	"fmt"
	
	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
)

// UserRepo 用户仓库接口
type UserRepo interface {
	// 查询用户的连接会话信息
	GetUserSession(ctx context.Context, uid string) (*data.Session, error)
	
	// 批量查询用户会话连接信息
	// 接受uid数组，返回用户的session连接信息，假设一个用户只有一个连接
	BatchGetUserSessions(ctx context.Context, uids []string) (map[string]*data.Session, error)
}

// MessageRepo 消息仓库接口
type MessageRepo interface {
	// 批量存储消息
	SaveMessages(ctx context.Context, messages []*po.Message) error
}

type Push struct {
	log                  *log.Helper
	user                 UserRepo
	message              MessageRepo
	redisClient          *redis.Client
	accessClient         accessV1.AccessServiceClient
	accessClientManager  *data.AccessServiceClientManager
}

// NewPush
func NewPush(logger log.Logger, user UserRepo, message MessageRepo, redisClient *redis.Client, 
	accessClient accessV1.AccessServiceClient, accessClientManager *data.AccessServiceClientManager) *Push {
	return &Push{
		log:                 log.NewHelper(logger),
		user:                user,
		message:             message,
		redisClient:         redisClient,
		accessClient:        accessClient,
		accessClientManager: accessClientManager,
	}
}

// 分布式锁的键前缀
const lockPrefix = "push:message:lock:"

// 分布式锁过期时间（10分钟）
const lockExpiration = 10 * time.Minute

// PushToUser 推送消息到用户
func (p *Push) PushToUser(ctx context.Context, taskID string, messages []*im_v1.BaseMessage) error {
	p.log.Debug("PushToUser", "taskID", taskID, "messageCount", len(messages))

	// 1. 根据uid查询用户连接到的access服务实例和connectionId，并按access服务实例分组
	// 建立access服务实例-消息映射，key为accessServiceId，value为该服务实例需要发送的ConnectionMessage列表
	accessMessageGroups := make(map[string][]*accessV1.ConnectionMessage)

	// 用于存储需要保存到数据库的消息实体
	dbMessages := make([]*po.Message, 0, len(messages))

	// 用于存储已经获取的锁，以便最后统一释放
	acquiredLocks := make(map[string]bool)
	defer func() {
		// 释放所有获取的分布式锁
		for lockKey := range acquiredLocks {
			if err := p.redisClient.Del(ctx, lockKey).Err(); err != nil {
				p.log.Error("failed to release distributed lock", "lockKey", lockKey, "error", err.Error())
			}
		}
	}()

	// 遍历所有消息，进行分组处理
	for _, msg := range messages {
		// 忽略空的用户ID
		if msg.ToUserId == "" {
			p.log.Warn("ignore message with empty to_user_id", "msg_id", msg.MsgId)
			continue
		}

		// 设置分布式锁，防止重复发送
		lockKey := lockPrefix + msg.MsgId
		lockValue := "processing"
		success, err := p.redisClient.SetNX(ctx, lockKey, lockValue, lockExpiration).Result()
		if err != nil {
			p.log.Error("failed to set distributed lock", "msg_id", msg.MsgId, "error", err.Error())
			continue
		}
		if !success {
			p.log.Info("message is already being processed by another instance", "msg_id", msg.MsgId)
			continue
		}
		// 记录已获取的锁
		acquiredLocks[lockKey] = true

		// 查询用户的完整会话信息
	session, err := p.user.GetUserSession(ctx, msg.ToUserId)
		if err != nil {
			p.log.Error("failed to get user session", "to_user_id", msg.ToUserId, "error", err.Error())
			// 继续处理其他消息
			continue
		}

		// 如果用户没有连接，记录日志后继续处理其他消息
		if session == nil || session.ConnectionId == "" {
			p.log.Debug("user has no active connections", "to_user_id", msg.ToUserId)
			// 保存消息到数据库，但不发送
		dbMessage := &po.Message{
				ID:          msg.MsgId,
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
				Status:      "offline", // 用户离线
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}
			dbMessages = append(dbMessages, dbMessage)
			continue
		}

		// 按access服务实例ID分组
		accessServiceId := session.AccessServiceId
		if accessServiceId == "" {
			// 如果access服务实例ID为空，使用默认值
			accessServiceId = "default"
			p.log.Debug("using default access service id", "to_user_id", msg.ToUserId, "accessServiceId", accessServiceId)
		}

		// 检查该access服务实例是否已有消息组
		found := false
		for _, connMsg := range accessMessageGroups[accessServiceId] {
			if connMsg.ConnectionId == session.ConnectionId {
				// 如果该连接已有消息组，直接添加消息
				connMsg.Message = append(connMsg.Message, msg)
				found = true
				break
			}
		}

		if !found {
			// 如果该连接还没有消息组，创建新的
			connMsg := &accessV1.ConnectionMessage{
				ConnectionId: session.ConnectionId,
				Message:      []*im_v1.BaseMessage{msg},
			}
			accessMessageGroups[accessServiceId] = append(accessMessageGroups[accessServiceId], connMsg)
		}

		// 准备数据库存储的消息实体
	dbMessage := &po.Message{
			ID:          msg.MsgId,
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
			Status:      "pending", // 使用字符串状态
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		dbMessages = append(dbMessages, dbMessage)
	}

	// 3. 遍历每个access服务实例，发送消息
	for accessServiceId, connectionMessages := range accessMessageGroups {
		p.log.Debug("sending messages to access service", "accessServiceId", accessServiceId, "messageCount", len(connectionMessages))

		request := &accessV1.PushMessageRequest{
			ConnectionMessages: connectionMessages,
		}

		// 获取该access服务实例的客户端
		accessClient := p.accessClientManager.GetClient(accessServiceId)
		// 调用access服务发送消息
		resp, err := accessClient.PushMessage(ctx, request)
		if err != nil {
			p.log.Error("failed to call access service PushMessage", "accessServiceId", accessServiceId, "error", err.Error())
			// 继续执行，保存消息到数据库
		} else {
			// 处理发送结果
			if resp.Code == accessV1.PushMessageResponse_OK {
				p.log.Debug("successfully sent messages to access service", "accessServiceId", accessServiceId)
				// 更新已发送消息的状态
				for _, connMsg := range connectionMessages {
					for _, msg := range connMsg.Message {
						for i := range dbMessages {
							if dbMessages[i].MsgID == msg.MsgId {
								dbMessages[i].Status = "sent"
								dbMessages[i].UpdatedAt = time.Now()
								break
							}
						}
					}
				}
			} else {
				p.log.Warn("access service PushMessage returned non-OK code", "accessServiceId", accessServiceId, "code", resp.Code, "message", resp.Message)
			}
		}
	}

	// 4. 把数据存储到数据库中
	if len(dbMessages) > 0 {
		if err := p.message.SaveMessages(ctx, dbMessages); err != nil {
			p.log.Error("failed to save messages", "error", err.Error())
			return err
		}
	}

	p.log.Info("PushToUser completed", "taskID", taskID, "messageCount", len(messages), "savedCount", len(dbMessages))
	return nil
}
