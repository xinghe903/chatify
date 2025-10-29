package data

import (
	v1 "api/access/v1"
	im_v1 "api/im/v1"
	"context"
	"errors"
	"fmt"
	"push/internal/biz"
	"push/internal/conf"
	"strings"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/circuitbreaker"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	ggrpc "google.golang.org/grpc"
)

var _ biz.AccessNodeManager = (*accessNodeManager)(nil)

// accessNodeManager 管理 access 服务节点，支持通过用户ID路由并发送gRPC消息
type accessNodeManager struct {
	discovery registry.Discovery
	data      *Data
	log       *log.Helper
	// access gRPC 连接池：sid -> conn
	accesss  map[string]v1.AccessServiceClient
	connsMux sync.Mutex
	conns    map[string]*ggrpc.ClientConn
	// ctx      context.Context
	cancel context.CancelCauseFunc
	// 配置
	// servicePrefix string
	// redisKeyFmt   string
}

// NewAccessNodeManager 创建新的管理器
func NewAccessNodeManager(
	c *conf.Bootstrap,
	discovery registry.Discovery,
	data *Data,
	logger log.Logger,
) (biz.AccessNodeManager, func(), error) {
	ctx, cancel := context.WithCancelCause(context.Background())

	manager := &accessNodeManager{
		discovery: discovery,
		data:      data,
		log:       log.NewHelper(logger),
		accesss:   make(map[string]v1.AccessServiceClient),
		conns:     make(map[string]*ggrpc.ClientConn),
		// ctx:           ctx,
		cancel: cancel,
		// servicePrefix: "access", // Kratos 默认服务名，实际注册为 /ms/instance/{sid}
		// redisKeyFmt:   "chatify:session:%s",
	}
	cleanup := func() {
		cancel(errors.New("access node manager closed"))
		manager.Close()
	}
	// 启动服务发现监听
	if err := manager.watchAccessNodes(ctx, c.Client.AccessClient); err != nil {
		manager.log.Errorf("Failed to watch access nodes: %v", err)
		return nil, cleanup, fmt.Errorf("failed to watch access nodes: %w", err)
	}

	return manager, cleanup, nil
}

// watchAccessNodes 监听 access 服务实例的变化
// 注意：Kratos 的 registry 以服务名（如 "access"）为 key，但你在 etcd 存的是 /microservices/access/{sid}
// 所以我们需要自定义 resolver 或理解其编码方式。
// 实际上，Kratos 在 etcd 写入格式为：
//
//	Key: /ms/access/{instance_id} → Value: {"addrs":["grpc://host:port"]}
//
// 因此我们监听服务名 "access"
func (m *accessNodeManager) watchAccessNodes(ctx context.Context, clientConf *conf.AccessClient) error {
	watcher, err := m.discovery.Watch(ctx, clientConf.Addr) // 服务名是 "access"
	if err != nil {
		return err
	}

	// 先获取一次全量实例
	if err = m.updateNodesFromInstances(ctx, watcher); err != nil {
		m.log.WithContext(ctx).Warnf("Failed to init access nodes: %v", err)
		return fmt.Errorf("failed to init access nodes: %w", err)
	}

	// 异步监听变化
	go func() {
		ticker := time.NewTicker(30 * time.Second) // 定期兜底拉取
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				m.log.WithContext(ctx).Infof("Access node manager closed")
				return
			case <-ticker.C:
				// 定期刷新防止事件丢失
				if err := m.updateNodesFromInstances(ctx, watcher); err != nil {
					m.log.WithContext(ctx).Errorf("Error watching access instances: %v", err)
				} else {
					m.connsMux.Lock()
					nodeLen := len(m.accesss)
					m.connsMux.Unlock()
					m.log.WithContext(ctx).Debugf("Updated access nodes count: %d", nodeLen)
				}
			}
		}
	}()

	return nil
}

// updateNodesFromInstances 从 watcher 获取实例并更新本地缓存
func (m *accessNodeManager) updateNodesFromInstances(ctx context.Context, watcher registry.Watcher) error {
	res, err := watcher.Next()
	if err != nil {
		return errors.Join(errors.New("watch access instances"), err)
	}
	var count int
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
					tracing.Client(),
					circuitbreaker.Client(),
				),
			)
			if err != nil {
				m.log.WithContext(ctx).Errorf("Failed to dial %s: %v", addr, err)
				continue
			}
			count++
			m.connsMux.Lock()
			m.accesss[sid] = v1.NewAccessServiceClient(conn)
			m.conns[sid] = conn
			m.connsMux.Unlock()
		}
	}
	m.log.WithContext(ctx).Debugf("Updated access nodes count: %d", count)
	return nil
}

// SendToUser 根据 userId 发送消息到其所在的 access 节点
// returns: 1. 成功发送的消息 ID  2. 错误
func (m *accessNodeManager) SendToUser(ctx context.Context, connectId string, messages []*im_v1.BaseMessage) ([]string, error) {
	var successMsgIDs []string
	var ok bool
	var client v1.AccessServiceClient
	m.connsMux.Lock()
	defer m.connsMux.Unlock()
	if client, ok = m.accesss[connectId]; !ok {
		return nil, errors.New("access node not found")
	}
	m.log.WithContext(ctx).Debugf("Send message to access node: %s", connectId)
	rsp, err := client.PushMessage(ctx, &v1.PushMessageRequest{
		ConnectionId: connectId,
		Message:      messages,
	})
	// 全部发送成功
	if err == nil {
		successMsgIDs = append(successMsgIDs, rsp.SuccessMessageIds...)
		return successMsgIDs, nil
	}
	// 部分发送成功
	if v1.IsPartialMessageFailed(err) {
		successMsgIDs = append(successMsgIDs, rsp.SuccessMessageIds...)
		return successMsgIDs, biz.ErrPartialSuccess
	}
	// 所有发送失败
	if v1.IsAllMessageFailed(err) {
		return nil, biz.ErrAllFailed
	}
	// 其他错误
	return nil, errors.Join(err, errors.New("failed to send message to access node"))
}

// Close 关闭所有连接
func (m *accessNodeManager) Close() error {
	m.connsMux.Lock()
	for _, conn := range m.conns {
		conn.Close()
	}
	m.conns = make(map[string]*ggrpc.ClientConn)
	m.accesss = make(map[string]v1.AccessServiceClient)
	m.connsMux.Unlock()
	return nil
}
