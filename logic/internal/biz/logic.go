package biz

import (
	"context"
	"logic/internal/conf"

	v1 "github.com/xinghe903/chatify/api/logic/v1"

	"logic/internal/biz/bo"

	"github.com/xinghe903/chatify/pkg/auth"

	"github.com/go-kratos/kratos/v2/log"
)

type PushRepo interface {
	SendMessage(ctx context.Context, taskId string, message []*bo.Message) error
}

type Logic struct {
	log        *log.Helper
	pushClient PushRepo
	config     *conf.Bootstrap
	sonyFlake  *auth.Sonyflake
}

// NewLogic 构造函数，通过依赖注入获取所有必要的服务
func NewLogic(
	logger log.Logger,
	pushClient PushRepo,
	c *conf.Bootstrap,
) *Logic {
	return &Logic{
		log:        log.NewHelper(logger),
		pushClient: pushClient,
		config:     c,
		sonyFlake:  auth.NewSonyflake(),
	}
}

// SendSystemPush 处理系统推送消息
// @Param ctx 上下文
// @Param req 系统推送请求
// @Return *v1.SystemPushResponse 响应
// @Return error 错误
// SendSystemPush 处理系统推送请求
// @Summary 系统推送接口
// @Description 处理系统发送的推送消息，支持黑白名单过滤
// @Tags 系统推送
// @Param req body v1.SystemPushRequest true "推送请求"
// @Return *v1.SystemPushResponse 响应
// @Return error 错误
func (l *Logic) SendSystemPush(ctx context.Context, req *v1.SystemPushRequest) (*v1.SystemPushResponse, error) {
	l.log.WithContext(ctx).Infof("Receive system push request from %s, content_id: %s", req.FromUserId, req.ContentId)

	// 1. 检查目标用户数量是否超过限制
	if len(req.ToUserIds) > bo.MaxTargetUsers {
		l.log.WithContext(ctx).Errorf("too many target users limit=%d, input=%d", bo.MaxTargetUsers, len(req.ToUserIds))
		return nil, v1.ErrorTooManyTargets("too many target users limit=%d, input=%d",
			bo.MaxTargetUsers, len(req.ToUserIds))
	}

	// 2. 进行用户校验和黑白名单过滤

	// 3. 创建消息列表
	messages := bo.NewMessagesByUserIDs(req)

	// 4. 调用push服务发送消息
	var err error
	var contentId string
	if contentId, err = l.sonyFlake.GenerateBase62(); err != nil {
		l.log.WithContext(ctx).Errorf("failed to generate content id: %v", err)
		return nil, v1.ErrorInternalError("failed to generate content id: %v", err)
	}
	for _, message := range messages {
		message.ContentId = "content" + contentId
		if message.MsgId, err = l.sonyFlake.GenerateBase62(); err != nil {
			l.log.WithContext(ctx).Errorf("failed to generate message id: %v", err)
			return nil, v1.ErrorInternalError("failed to generate message id: %v", err)
		}
		message.MsgId = "msg" + message.MsgId
	}
	var taskId string
	if taskId, err = l.sonyFlake.GenerateBase62(); err != nil {
		l.log.WithContext(ctx).Errorf("failed to generate task id: %v", err)
		return nil, v1.ErrorInternalError("failed to generate task id: %v", err)
	}
	if err = l.pushClient.SendMessage(ctx, "task"+taskId, messages); err != nil {
		l.log.WithContext(ctx).Errorf("failed to send message to push service: %v", err)
		return nil, v1.ErrorInvokePushFailed("failed to send message to push service: %v", err)
	}
	l.log.WithContext(ctx).Infof("Sent message to push service. TaskID: %s, len: %d", taskId, len(messages))
	// 5. 返回成功响应
	return &v1.SystemPushResponse{}, nil
}
