package service

import (
	"access/internal/biz"
	"access/internal/biz/bo"
	"access/internal/conf"
	v1 "api/access/v1"
	"context"
	"errors"
	"net/http"
	"pkg/auth"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/metadata"
	"github.com/gorilla/websocket"
	"go.opentelemetry.io/otel/trace"
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		// 允许跨域（生产环境建议配置具体域名）
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
)

type AccessService struct {
	v1.UnimplementedAccessServiceServer
	log         *log.Helper
	connManager *biz.Manager
	svrInstance *conf.ServerInstance
}

func NewAccessService(
	logger log.Logger,
	manager *biz.Manager,
	svrInstance *conf.ServerInstance,
) *AccessService {
	svc := &AccessService{
		log:         log.NewHelper(logger),
		connManager: manager,
		svrInstance: svrInstance,
	}
	return svc
}

func (s *AccessService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// 升级协议
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.WithContext(ctx).Errorf("WebSocket upgrade error: %v", err)
		return
	}
	ctx, cancel := withoutTimeout(ctx)
	defer cancel(errors.New("root disconnected"))
	ctx = auth.NewContext(ctx, r.Header.Get(string(auth.USER_ID)), r.Header.Get(string(auth.USER_NAME)))
	client := &biz.Client{
		Conn:           conn,
		Send:           make(chan *bo.SendContext, 100),
		UserID:         auth.GetUserID(ctx),
		UserName:       auth.GetUserName(ctx),
		ConnectionTime: time.Now().Unix(),
		ConnectionId:   s.svrInstance.Id,
	}
	s.log.WithContext(ctx).Debugf("Client connected: %s, serviceId: %s", conn.RemoteAddr(), s.svrInstance.Id)
	s.log.WithContext(ctx).Debugf("Client userId: %s, username: %s", client.UserID, auth.GetUserName(ctx))
	s.connManager.StartClient(ctx, client)
}

func withoutTimeout(parent context.Context) (context.Context, context.CancelCauseFunc) {
	ctx, cancel := context.WithCancelCause(context.Background())
	// 复制 metadata
	if md, ok := metadata.FromServerContext(parent); ok {
		ctx = metadata.NewServerContext(ctx, md)
	}
	// 复制 tracing
	if sc := trace.SpanFromContext(parent).SpanContext(); sc.IsValid() {
		ctx = trace.ContextWithSpanContext(ctx, sc)
	}
	return ctx, cancel
}

// func (s *AccessService) dispatch(message *v1.ClientToAccessMessage) {
// 	switch message.Type {
// 	case v1.ClientToAccessMessage_AUTH:
// 		// 处理认证
// 	case v1.ClientToAccessMessage_HEARTBEAT:
// 		// 处理心跳
// 	case v1.ClientToAccessMessage_CHAT:
// 		// 处理聊天消息
// 	case v1.ClientToAccessMessage_ACK:
// 		// 确认收到消息
// 	default:
// 		// s.log.WithContext(ctx).Warnf("Received unknown message type: %s", message.Type)
// 	}
// }

func (s *AccessService) PushMessage(ctx context.Context, req *v1.PushMessageRequest) (*v1.PushMessageResponse, error) {
	s.log.WithContext(ctx).Debugf("Received message: %+v\n", req)
	if req.ConnectionId != s.svrInstance.Id {
		s.log.WithContext(ctx).Errorf("Received message from unknown connection: %s", req.ConnectionId)
		return nil, errors.New("unknown connection id")
	}
	var successMsgIDs []string
	for _, message := range req.Message {
		s.connManager.SendToUser(ctx, message.ToUserId, []byte(message.String()))
		successMsgIDs = append(successMsgIDs, message.MsgId)
	}

	return &v1.PushMessageResponse{
		Code:              v1.PushMessageResponse_ALL_SUCCESS,
		Message:           "success",
		SuccessMessageIds: successMsgIDs,
	}, nil
}
