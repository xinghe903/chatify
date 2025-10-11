package service

import (
	"access/internal/biz"
	v1 "api/access/v1"
	"context"
	"net/http"
	"pkg/auth"

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
	log       *log.Helper
	wsManager *biz.Manager
	consumer  *biz.Consumer
}

func NewAccessService(logger log.Logger, consumer *biz.Consumer) *AccessService {
	svc := &AccessService{
		log:       log.NewHelper(logger),
		wsManager: biz.NewManager(logger),
		consumer:  consumer,
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
		Conn:   conn,
		Send:   make(chan []byte, 256),
		UserID: auth.GetUserID(ctx),
	}
	s.wsManager.RegisterClient(ctx, client)
	s.log.WithContext(ctx).Debugf("Client connected: %s", conn.RemoteAddr())
	// client.Conn.SetReadLimit(512 << 10) // 512KB
	// client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	// client.Conn.SetPongHandler(func(string) error {
	// 	client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	// 	return nil
	// })

	// for {
	// 	messageType, message, err := client.Conn.ReadMessage()
	// 	if err != nil {
	// 		s.wsManager.UnregisterClient(client)
	// 		client.Conn.Close()
	// 		break
	// 	}
	// 	if messageType != websocket.TextMessage {
	// 		s.log.WithContext(ctx).Errorf("Invalid message type: %d", messageType)
	// 		continue
	// 	}
	// 	var clientMsg v1.ClientToAccessMessage
	// 	if err := proto.Unmarshal(message, &clientMsg); err != nil {
	// 		s.log.WithContext(ctx).Errorf("proto unmarshal error: %v", err)
	// 		s.sendMessage(client, &v1.AccessToClientMessage{
	// 			Type:   v1.AccessToClientMessage_ACK,
	// 			Status: v1.AccessToClientMessage_FAILURE,
	// 		})
	// 		continue
	// 	}

	// 	s.log.WithContext(ctx).Debugf("Received: %s", message)
	// 	s.dispatch(&clientMsg)
	// }
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
	return nil, nil
}
