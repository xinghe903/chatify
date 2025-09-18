package service

import (
	v1 "access/api/access/v1"
	"access/internal/biz"
	"context"
	"net/http"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
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
		wsManager: biz.NewManager(),
		consumer:  consumer,
	}
	return svc
}

func (s *AccessService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 升级协议
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Errorf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()
	client := &biz.Client{
		Conn:   conn,
		Send:   make(chan []byte, 256),
		UserID: "", // 后续从context里面获取
	}
	s.wsManager.RegisterClient(client)

	s.log.Debugf("Client connected: %s", conn.RemoteAddr())
	defer func() {
		s.wsManager.UnregisterClient(client)
		client.Conn.Close()
	}()

	client.Conn.SetReadLimit(512 << 10) // 512KB
	client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.Conn.SetPongHandler(func(string) error {
		client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		messageType, message, err := client.Conn.ReadMessage()
		if err != nil {
			s.wsManager.UnregisterClient(client)
			client.Conn.Close()
			break
		}
		if messageType != websocket.TextMessage {
			s.log.Errorf("Invalid message type: %d", messageType)
			continue
		}
		var clientMsg v1.ClientToAccessMessage
		if err := proto.Unmarshal(message, &clientMsg); err != nil {
			s.log.Errorf("proto unmarshal error: %v", err)
			s.sendMessage(client, &v1.AccessToClientMessage{
				Type:   v1.AccessToClientMessage_ACK,
				Status: v1.AccessToClientMessage_FAILURE,
			})
			continue
		}

		s.log.Debugf("Received: %s", message)
		s.dispatch(&clientMsg)
	}
}
func (s *AccessService) dispatch(message *v1.ClientToAccessMessage) {
	switch message.Type {
	case v1.ClientToAccessMessage_AUTH:
		// 处理认证
	case v1.ClientToAccessMessage_HEARTBEAT:
		// 处理心跳
	case v1.ClientToAccessMessage_CHAT:
		// 处理聊天消息
	case v1.ClientToAccessMessage_ACK:
		// 确认收到消息
	default:
		s.log.Warnf("Received unknown message type: %s", message.Type)
	}
}

func (s *AccessService) sendMessage(client *biz.Client, msg *v1.AccessToClientMessage) {
	msg.MessageId = uuid.New().String()
	msgBytes, err := proto.Marshal(msg)
	if err != nil {
		s.log.Errorf("Error marshaling response:%s", err.Error())
		return
	}
	if err := client.Conn.WriteMessage(websocket.TextMessage, msgBytes); err != nil {
		s.log.Errorf("Error sending message:%s", err.Error())
	}
}

// 下行消息推送（从PushService到客户端）- 单推
func (s *AccessService) PushToClient(ctx context.Context, req *v1.PushRequest) (*v1.PushResponse, error) {
	return nil, nil
}

// 下行消息批量推送（从PushService到多个客户端）
func (s *AccessService) BatchPushToClient(ctx context.Context, req *v1.BatchPushRequest) (*v1.BatchPushResponse, error) {
	return nil, nil
}

// 广播消息到所有连接到此节点的客户端
func (s *AccessService) BroadcastToClients(ctx context.Context, req *v1.BroadcastRequest) (*v1.BroadcastResponse, error) {
	return nil, nil
}

// 检查客户端连接状态
func (s *AccessService) CheckClientConnection(ctx context.Context, req *v1.ConnectionCheckRequest) (*v1.ConnectionCheckResponse, error) {
	return nil, nil
}
