package service

import (
	v1 "api/access/v1"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/circuitbreaker"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/redis/go-redis/v9"
	ggrpc "google.golang.org/grpc"
)

// PushRequest 是你定义在 .proto 中的消息结构（示例）
type PushRequest struct {
	UserId string `json:"user_id"`
	Data   []byte `json:"data"`
}

type PushResponse struct {
	Code int32  `json:"code"`
	Msg  string `json:"msg"`
}

// AccessNodeManager 管理 access 服务节点，支持通过用户ID路由并发送gRPC消息
type AccessNodeManager struct {
	discovery registry.Discovery
	redis     *redis.Client
	log       *log.Helper

	// sid -> host:port 缓存
	// nodes    map[string]string
	nodesMux sync.RWMutex

	// access gRPC 连接池：sid -> conn
	accesss  map[string]v1.AccessServiceClient
	connsMux sync.Mutex
	conns    map[string]*ggrpc.ClientConn
	ctx      context.Context
	cancel   context.CancelCauseFunc

	// 配置
	servicePrefix string
	redisKeyFmt   string
}

// NewAccessNodeManager 创建新的管理器
func NewAccessNodeManager(
	discovery registry.Discovery,
	redis *redis.Client,
	logger log.Logger,
) (*AccessNodeManager, error) {
	ctx, cancel := context.WithCancelCause(context.Background())

	manager := &AccessNodeManager{
		discovery:     discovery,
		redis:         redis,
		log:           log.NewHelper(log.With(logger, "module", "access_node_manager")),
		accesss:       make(map[string]v1.AccessServiceClient),
		ctx:           ctx,
		cancel:        cancel,
		servicePrefix: "access", // Kratos 默认服务名，实际注册为 /ms/instance/{sid}
		redisKeyFmt:   "chatify:session:%s",
	}

	// 启动服务发现监听
	if err := manager.watchAccessNodes(); err != nil {
		cancel(errors.New("access node manager closed"))
		return nil, fmt.Errorf("failed to watch access nodes: %w", err)
	}

	return manager, nil
}

// watchAccessNodes 监听 access 服务实例的变化
// 注意：Kratos 的 registry 以服务名（如 "access"）为 key，但你在 etcd 存的是 /microservices/access/{sid}
// 所以我们需要自定义 resolver 或理解其编码方式。
// 实际上，Kratos 在 etcd 写入格式为：
//
//	Key: /ms/access/{instance_id} → Value: {"addrs":["grpc://host:port"]}
//
// 因此我们监听服务名 "access"
func (m *AccessNodeManager) watchAccessNodes() error {
	watcher, err := m.discovery.Watch(m.ctx, "access") // 服务名是 "access"
	if err != nil {
		return err
	}

	// 先获取一次全量实例
	if err = m.updateNodesFromInstances(watcher); err != nil {
		m.log.Warnf("Failed to init access nodes: %v", err)
	}

	// 异步监听变化
	go func() {
		ticker := time.NewTicker(30 * time.Second) // 定期兜底拉取
		defer ticker.Stop()

		for {
			select {
			case <-m.ctx.Done():
				return
			case <-ticker.C:
				// 定期刷新防止事件丢失
				_ = m.updateNodesFromInstances(watcher)
			default:
				// 尝试更新
				if err := m.updateNodesFromInstances(watcher); err != nil {
					m.log.Errorf("Error watching access instances: %v", err)
				}
				time.Sleep(1 * time.Second)
			}
		}
	}()

	return nil
}

// updateNodesFromInstances 从 watcher 获取实例并更新本地缓存
func (m *AccessNodeManager) updateNodesFromInstances(watcher registry.Watcher) error {
	res, err := watcher.Next()
	if err != nil {
		return err
	}

	updatedNodes := make(map[string]string)
	for _, ins := range res {
		// instance ID 即为 sid
		sid := ins.ID
		// 获取 grpc 地址
		var addr string
		for _, endpoint := range ins.Endpoints {
			if strings.HasPrefix(endpoint, "grpc://") {
				addr = strings.TrimPrefix(endpoint, "grpc://")
				break
			}
		}
		if addr == "" && len(ins.Endpoints) > 0 {
			addr = ins.Endpoints[0]
			if strings.HasPrefix(addr, "http://") {
				addr = addr[len("http://"):]
			}
		}

		if addr != "" {
			conn, err := grpc.DialInsecure(
				context.Background(),
				grpc.WithEndpoint(addr),
				grpc.WithMiddleware(
					recovery.Recovery(),
					circuitbreaker.Client(),
				),
			)
			if err != nil {
				m.log.Errorf("Failed to dial %s: %v", addr, err)
				continue
			}
			m.accesss[sid] = v1.NewAccessServiceClient(conn)
			m.conns[sid] = conn
		}
	}

	m.log.Infof("Updated access nodes count: %d", len(updatedNodes))
	return nil
}

// SendToUser 根据 userId 发送消息到其所在的 access 节点
func (m *AccessNodeManager) SendToUser(ctx context.Context, sid string, message *v1.ConnectionMessage) (*PushResponse, error) {
	// Step 1: 从 Redis 获取该用户当前连接的 access 实例 ID (sid)
	redisKey := fmt.Sprintf(m.redisKeyFmt, userID)
	sid, err := m.redis.Get(ctx, redisKey).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("user %s is not connected", userID)
	} else if err != nil {
		return nil, fmt.Errorf("redis error: %w", err)
	}

	// Step 2: 获取该 sid 的 gRPC 客户端
	conn, err := m.getGRPCClient(sid)
	if err != nil {
		return nil, err
	}

	// Step 3: 构造请求并发送（这里假设你生成了 pb.PushClient）
	client := NewPushClient(conn) // 替换为你实际生成的 client
	req := &PushRequest{
		UserId: userID,
		Data:   data,
	}

	return client.Push(ctx, req)
}

// Close 关闭所有连接
func (m *AccessNodeManager) Close() error {
	m.cancel(errors.New("access node manager closed"))

	m.connsMux.Lock()
	for _, conn := range m.conns {
		conn.Close()
	}
	m.conns = make(map[string]*ggrpc.ClientConn)
	m.connsMux.Unlock()

	return nil
}
