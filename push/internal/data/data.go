package data

import (
	"context"
	"sync"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"push/internal/conf"
	"push/internal/data/po"
	"time"
	accessV1 "api/access/v1"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(NewData, NewUserRepo, NewMessageRepo, NewEtcdClient, NewRegistry,
	func(d *Data) *redis.Client { return d.redisClient },
	NewAccessServiceClient, NewAccessServiceClientManager)

// Data .
type Data struct {
	// 数据库连接
	db *gorm.DB
	// Redis客户端
	redisClient *redis.Client
}

// NewData .
func NewData(cb *conf.Bootstrap, logger log.Logger) (*Data, func(), error) {
	logHelper := log.NewHelper(logger)
	
	// 初始化数据库连接
	var db *gorm.DB
	var err error
	if cb.Data.Database.Driver == "mysql" {
		// 配置gorm日志
		gormLogger := logger.New(
			logHelper,
			logger.Config{
				SlowThreshold: time.Second,
				LogLevel:      logger.Info,
				Colorful:      true,
			},
		)
		
		// 连接数据库
		db, err = gorm.Open(mysql.Open(cb.Data.Database.Source), &gorm.Config{
			Logger: gormLogger,
		})
		if err != nil {
			logHelper.Error("failed to connect database", err.Error())
			return nil, nil, err
		}
		
		// 自动迁移表结构
		if err = db.AutoMigrate(&po.Message{}); err != nil {
			logHelper.Error("failed to migrate database", err.Error())
			return nil, nil, err
		}
	}
	
	// 初始化Redis客户端
	redisClient := redis.NewClient(&redis.Options{
		Network:  cb.Data.Redis.Network,
		Addr:     cb.Data.Redis.Addr,
		ReadTimeout:  cb.Data.Redis.ReadTimeout.AsDuration(),
		WriteTimeout: cb.Data.Redis.WriteTimeout.AsDuration(),
	})
	
	// 测试Redis连接
	if _, err := redisClient.Ping(context.Background()).Result(); err != nil {
		logHelper.Error("failed to connect redis", err.Error())
		return nil, nil, err
	}
	
	d := &Data{
		db:          db,
		redisClient: redisClient,
	}
	
	cleanup := func() {
		logHelper.Info("closing the data resources")
		// 关闭数据库连接
		if sqlDB, err := d.db.DB(); err == nil {
			sqlDB.Close()
		}
		// 关闭Redis连接
		if err := d.redisClient.Close(); err != nil {
			logHelper.Error("failed to close redis", err.Error())
		}
	}
	return d, cleanup, nil
}

// AccessServiceClientManager 管理多个access服务客户端
type AccessServiceClientManager struct {
	clients     map[string]accessV1.AccessServiceClient
	discovery   registry.Discovery
	mutex       sync.RWMutex
	defaultConn accessV1.AccessServiceClient
}

// NewAccessServiceClientManager 创建access服务客户端管理器
func NewAccessServiceClientManager(r registry.Discovery) *AccessServiceClientManager {
	manager := &AccessServiceClientManager{
		clients:   make(map[string]accessV1.AccessServiceClient),
		discovery: r,
	}
	
	// 创建默认客户端（用于无特定服务实例ID的情况）
	conn, err := grpc.DialInsecure(
		context.Background(),
		grpc.WithEndpoint("discovery:///access.service"),
		grpc.WithDiscovery(r),
		grpc.WithMiddleware(
			recovery.Recovery(),
		),
	)
	if err != nil {
		panic(err)
	}
	manager.defaultConn = accessV1.NewAccessServiceClient(conn)
	
	return manager
}

// GetClient 获取指定access服务实例的客户端
func (m *AccessServiceClientManager) GetClient(accessServiceId string) accessV1.AccessServiceClient {
	// 如果accessServiceId为空，返回默认客户端
	if accessServiceId == "" || accessServiceId == "default" {
		return m.defaultConn
	}
	
	// 尝试从缓存中获取客户端
	m.mutex.RLock()
	client, exists := m.clients[accessServiceId]
	m.mutex.RUnlock()
	
	if exists {
		return client
	}
	
	// 如果缓存中没有，创建新的客户端
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	// 二次检查，防止并发创建
	if client, exists := m.clients[accessServiceId]; exists {
		return client
	}
	
	// 构建特定服务实例的endpoint
	// 注意：这里假设access服务实例注册时使用了特定的ID作为元数据
	// 实际实现可能需要根据服务发现机制调整
	endpoint := fmt.Sprintf("discovery:///access.service?filter=id%%3D%s", accessServiceId)
	
	conn, err := grpc.DialInsecure(
		context.Background(),
		grpc.WithEndpoint(endpoint),
		grpc.WithDiscovery(m.discovery),
		grpc.WithMiddleware(
			recovery.Recovery(),
		),
	)
	if err != nil {
		// 如果创建特定实例的客户端失败，返回默认客户端
		return m.defaultConn
	}
	
	client = accessV1.NewAccessServiceClient(conn)
	m.clients[accessServiceId] = client
	
	return client
}

// NewAccessServiceClient 创建access服务的grpc客户端（兼容旧接口）
func NewAccessServiceClient(r registry.Discovery) accessV1.AccessServiceClient {
	// 创建grpc客户端，使用服务发现
	conn, err := grpc.DialInsecure(
		context.Background(),
		grpc.WithEndpoint("discovery:///access.service"),
		grpc.WithDiscovery(r),
		grpc.WithMiddleware(
			recovery.Recovery(),
		),
	)
	if err != nil {
		panic(err)
	}
	return accessV1.NewAccessServiceClient(conn)
}
