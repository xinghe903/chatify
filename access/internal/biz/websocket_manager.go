package biz

import (
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Client 代表一个 WebSocket 客户端连接
type Client struct {
	Conn   *websocket.Conn
	Send   chan []byte
	UserID string
}

// Manager 管理所有客户端连接
type Manager struct {
	clients    map[string]*Client
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

// NewManager 创建新的连接管理器
func NewManager() *Manager {
	return &Manager{
		clients:    make(map[string]*Client),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Start 启动管理器事件循环
func (m *Manager) Start() {
	log.Println("WebSocket manager started")
	go m.broadcastListener()
}

func (m *Manager) RegisterClient(client *Client) {
	m.register <- client
}

func (m *Manager) UnregisterClient(client *Client) {
	m.unregister <- client
}

func (m *Manager) broadcastListener() {
	for {
		select {
		case message := <-m.broadcast:
			m.mu.RLock()
			for id, client := range m.clients {
				select {
				case client.Send <- message:
				default:
					// 发送阻塞，关闭连接
					close(client.Send)
					delete(m.clients, id)
				}
			}
			m.mu.RUnlock()

		case client := <-m.register:
			m.mu.Lock()
			m.clients[client.UserID] = client
			m.mu.Unlock()
			log.Printf("Client registered: %s", client.UserID)

		case client := <-m.unregister:
			m.mu.Lock()
			if _, ok := m.clients[client.UserID]; ok {
				close(client.Send)
				delete(m.clients, client.UserID)
				log.Printf("Client unregistered: %s", client.UserID)
			}
			m.mu.Unlock()
		}
	}
}

// SendToUser 向指定用户发送消息
func (m *Manager) SendToUser(userID string, message []byte) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if client, ok := m.clients[userID]; ok {
		select {
		case client.Send <- message:
		default:
			// 队列满，主动踢出
			m.unregister <- client
		}
	}
}

// Count 返回当前连接数
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.clients)
}

func (m *Manager) readPump(client *Client) {
	defer func() {
		m.unregister <- client
		client.Conn.Close()
	}()

	client.Conn.SetReadLimit(512 << 10) // 512KB
	client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.Conn.SetPongHandler(func(string) error {
		client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := client.Conn.ReadMessage()
		if err != nil {
			break
		}
		// 处理上行消息（可转发给 Logic Service）
		log.Printf("Received from %s: %s", client.UserID, message)
	}
}

func (m *Manager) writePump(client *Client) {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		m.unregister <- client
	}()

	for {
		select {
		case message, ok := <-client.Send:
			client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				client.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			err := client.Conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				return
			}
		case <-ticker.C:
			client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
