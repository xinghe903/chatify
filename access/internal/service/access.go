package service

import (
	"access/internal/biz"
	"access/internal/conf"
	v1 "api/access/v1"
	"context"
	"errors"
	"net/http"
	"pkg/auth"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/gorilla/websocket"
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
	consumer    *biz.Consumer
	svrInstance *conf.ServerInstance
}

func NewAccessService(
	logger log.Logger,
	consumer *biz.Consumer,
	manager *biz.Manager,
	svrInstance *conf.ServerInstance,
) *AccessService {
	svc := &AccessService{
		log:         log.NewHelper(logger),
		connManager: manager,
		consumer:    consumer,
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
	ctx = auth.NewContext(ctx, r.Header.Get(string(auth.USER_ID)), r.Header.Get(string(auth.USER_NAME)))
	client := &biz.Client{
		Conn:           conn,
		Send:           make(chan []byte, 256),
		UserID:         auth.GetUserID(ctx),
		UserName:       auth.GetUserName(ctx),
		ConnectionTime: time.Now().Unix(),
		ConnectionId:   s.svrInstance.Id,
	}
	s.log.WithContext(ctx).Debugf("Client connected: %s, serviceId: %s", conn.RemoteAddr(), s.svrInstance.Id)
	s.log.WithContext(ctx).Debugf("Client userId: %s, username: %s", client.UserID, auth.GetUserName(ctx))
	s.connManager.StartClient(ctx, client)
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
	for _, reqMessage := range req.ConnectionMessages {
		if reqMessage.ConnectionId != s.svrInstance.Id {
			s.log.WithContext(ctx).Errorf("Received message from unknown connection: %s", reqMessage.ConnectionId)
			return nil, errors.New("unknown connection id")
		}
		for _, message := range reqMessage.Message {
			s.connManager.SendToUser(ctx, message.ToUserId, []byte(message.String()))
		}
	}
	return nil, nil
}
