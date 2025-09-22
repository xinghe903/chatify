package biz

import (
	"context"
	"errors"
	"fmt"
	v1 "logic/api/logic/v1"
	"logic/internal/conf"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"logic/internal/biz/bo"
	"pkg/auth/sign"
)

type UserRepo interface {
	IsUserOnline(ctx context.Context, userID string) (bool, error)
}

type CacheRepo interface {
	GetClient() *redis.Client
}

type PushRepo interface {
	SendMessage(ctx context.Context, message *bo.Message) error
}

type Logic struct {
	log           *log.Helper
	user          UserRepo
	replayChecker *sign.ReplayChecker
	redisClient   *redis.Client
	pushClient    PushRepo
	config        *conf.Bootstrap
}

// NewLogic 构造函数，通过依赖注入获取所有必要的服务
func NewLogic(
	logger log.Logger,
	user UserRepo,
	cache CacheRepo,
	pushClient PushRepo,
	c *conf.Bootstrap,
) *Logic {

	replayChecker := sign.NewReplayChecker(cache.GetClient(), 0, 0)
	return &Logic{
		log:           log.NewHelper(logger),
		user:          user,
		replayChecker: replayChecker,
		redisClient:   cache.GetClient(),
		pushClient:    pushClient,
		config:        c,
	}
}

// SendMessage 发送消息
// @Param message 发送消息的参数
// @Return messageId 消息id
// @Return error 错误
func (l *Logic) SendMessage(ctx context.Context, message *bo.Message) (string, error) {
	if message == nil {
		return "", v1.ErrorInternalError("message is empty")
	}
	if len(message.Target) == 0 {
		return "", v1.ErrorUserNotFound("target is empty")
	}
	// 生成唯一消息ID
	messageId := generateUniqueMessageID()
	message.MessageId = messageId
	onlines := make([]*bo.TargetUser, 0)
	offlines := make([]string, 0)
	// 单聊
	for _, target := range message.Target {
		if target.GroupId != "" {
			continue
		}
		// 查询用户在线状态
		isOnline, err := l.user.IsUserOnline(ctx, target.UserId)
		if err != nil {
			l.log.WithContext(ctx).Errorf("Failed to check user %s online status: %v", target.UserId, err)
			continue
		}
		if isOnline {
			onlines = append(onlines, &bo.TargetUser{UserId: target.UserId, FromUserId: "xinghe"})
		} else {
			offlines = append(offlines, target.UserId)
		}
	}

	if err := l.pushClient.SendMessage(ctx, &bo.Message{
		Target:      onlines,
		Content:     message.Content,
		ContentType: message.ContentType,
		ExpireTime:  message.ExpireTime,
		MessageId:   messageId,
	}); err != nil {
		return "", v1.ErrorInternalError("failed to send message")
	}

	// 处理离线数据
	l.log.WithContext(ctx).Debugf("Offline users: %v", offlines)

	return messageId, nil
}

// ValidateRequest 校验消息签名
func (l *Logic) ValidateRequest(ctx context.Context, message *sign.SignParam) error {
	if message == nil {
		return fmt.Errorf("message is nil")
	}

	// 使用消息ID作为请求ID进行防重放校验
	err := l.replayChecker.ValidateRequest(context.Background(), message, l.config.Server.Secret)
	if err != nil {
		l.log.WithContext(ctx).Warnf("Failed to validate request: %s", err.Error())
		if errors.Is(err, sign.ErrRequestExpired) {
			return v1.ErrorMessageExpired("request expired. request_time=%d, now=%d", message.Timestamp, time.Now().UnixMilli())
		}

		if errors.Is(err, sign.ErrRequestRepeat) {
			return v1.ErrorMessageRepeat("message repeat. request_id=%s", message.RequestID)
		}

		return v1.ErrorPermissionDenied("invalid signature~~")
	}
	return nil
}

// generateUniqueMessageID 生成唯一消息ID
func generateUniqueMessageID() string {
	return uuid.New().String()
}

// pushMessageToUser 推送消息给用户
func (l *Logic) pushMessageToUser(ctx context.Context, userId string, message *bo.Message) error {
	// 将消息内容转换为字节数组
	// content, err := json.Marshal(message)
	// if err != nil {
	// 	return fmt.Errorf("failed to marshal message: %w", err)
	// }
	l.log.Debugf("Pushing message to user %s: %s", userId, message)
	// 调用Push服务的gRPC接口
	if err := l.pushClient.SendMessage(ctx, message); err != nil {
		return fmt.Errorf("failed to push message to user %s: %w", userId, err)
	}

	return nil
}
