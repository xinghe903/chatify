package biz

import (
	"access/internal/biz/bo"
	"context"
	"errors"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	"github.com/gorilla/websocket"
)

type SessionRepo interface {
	SetSession(ctx context.Context, session *bo.Session) error
	BatchClearSession(ctx context.Context, uids []string) error
	GetSession(ctx context.Context, uid string) (*bo.Session, error)
	ClearSession(ctx context.Context, uid string) error
}

// Client 代表一个 WebSocket 客户端连接
type Client struct {
	Conn           *websocket.Conn
	Send           chan []byte
	UserID         string
	UserName       string
	readCtxCancel  context.CancelCauseFunc
	writeCtxCancel context.CancelCauseFunc
	ConnectionId   string
	ConnectionTime int64
}

// Manager 管理所有客户端连接
type Manager struct {
	clients map[string]*Client
	mu      sync.RWMutex
	log     *log.Helper
	session SessionRepo
}

// NewManager 创建新的连接管理器
func NewManager(logger log.Logger, session SessionRepo) (*Manager, func()) {
	manager := &Manager{
		clients: make(map[string]*Client),
		log:     log.NewHelper(logger),
	}
	cleanup := func() {
		manager.log.Info("closing the manager resources")
		uids := make([]string, 0, len(manager.clients))
		for _, client := range manager.clients {
			client.Conn.Close()
			uids = append(uids, client.UserID)
		}
		manager.session.BatchClearSession(context.Background(), uids)
	}
	return manager, cleanup
}

func (m *Manager) StartClient(ctx context.Context, client *Client) {
	err := m.session.SetSession(ctx, &bo.Session{
		Uid:            client.UserID,
		Username:       client.UserName,
		ConnectionTime: client.ConnectionTime,
		ConnectionId:   client.ConnectionId,
	})
	if err != nil {
		m.log.WithContext(ctx).Errorf("Set session error: %v", err)
		client.Conn.Close()
		return
	}
	var rctx, wctx context.Context
	rctx, client.readCtxCancel = context.WithCancelCause(ctx)
	wctx, client.writeCtxCancel = context.WithCancelCause(ctx)
	go m.writePump(wctx, client)
	m.mu.Lock()
	m.clients[client.UserID] = client
	m.mu.Unlock()
	m.readPump(rctx, client)
}

func (m *Manager) StopClient(ctx context.Context, client *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.clients[client.UserID]; !ok {
		m.log.WithContext(ctx).Warnf("User %s is not exist", client.UserID)
		return
	}
	m.log.WithContext(ctx).Debugf("User %s disconnected", client.UserID)
	client.Conn.Close()
	delete(m.clients, client.UserID)
	m.session.ClearSession(ctx, client.UserID)
}

// SendToUser 向指定用户发送消息
func (m *Manager) SendToUser(ctx context.Context, userID string, message []byte) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if client, ok := m.clients[userID]; ok {
		select {
		case client.Send <- message:
		default:
			// 队列满，主动踢出
			m.log.WithContext(ctx).Warnf("User %s is full, kick out", userID)
		}
	}
}

// Count 返回当前连接数
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.clients)
}

func (m *Manager) readPump(ctx context.Context, client *Client) {
	defer func() {
		m.StopClient(ctx, client)
		client.readCtxCancel(errors.New("read unregister cause"))
	}()
	// 无法写入消息，则认为改连接已经断开
	client.Conn.SetReadLimit(512 << 10) // 512KB
	client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.Conn.SetPongHandler(func(string) error {
		client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := client.Conn.ReadMessage()
		if err != nil {
			m.log.WithContext(ctx).Warnf("Read message error: %v", err)
			break
		}
		select {
		case <-ctx.Done():
			m.log.WithContext(ctx).Errorf("read context done")
			return
		default:
		}
		// 处理上行消息
		m.log.WithContext(ctx).Debugf("Received from %s: %s", client.UserID, string(message))
	}
}

func (m *Manager) writePump(ctx context.Context, client *Client) {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		m.StopClient(ctx, client)
		client.writeCtxCancel(errors.New("write unregister cause"))
	}()

	for {
		select {
		case message, ok := <-client.Send:
			client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				m.log.WithContext(ctx).Errorf("userId=%s, client disconnected", client.UserID)
				client.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			err := client.Conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				m.log.WithContext(ctx).Errorf("userId=%s, Write message error: %v", client.UserID, err)
				return
			}
			m.log.WithContext(ctx).Debugf("userId=%s, Sent to %s", client.UserID, string(message))
		case <-ticker.C:
			client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				m.log.WithContext(ctx).Errorf("userId=%s, Write ping error: %v", client.UserID, err)
				return
			}
			m.log.WithContext(ctx).Debugf("userId=%s, Sent ping", client.UserID)
		}
	}
}
