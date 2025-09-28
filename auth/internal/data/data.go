package data

import (
	"context"
	"fmt"
	"time"

	"auth/internal/biz"
	"auth/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(NewData, NewUserRepo, NewCacheRepo)

// Data 数据层主结构
type Data struct {
	redis *redis.Client
}

// NewData 创建数据层实例
func NewData(cb *conf.Bootstrap, logger log.Logger) (*Data, func(), error) {
	// 初始化Redis客户端
	redisClient, err := initRedisClient(cb.Data, logger)
	if err != nil {
		return nil, nil, err
	}

	cleanup := func() {
		if redisClient != nil {
			if err := redisClient.Close(); err != nil {
				log.NewHelper(logger).Errorf("Failed to close redis client: %v", err)
			}
		}
		log.NewHelper(logger).Info("closing the data resources")
	}

	return &Data{
			redis: redisClient,
		},
		cleanup,
		nil
}

// initRedisClient 初始化Redis客户端
func initRedisClient(c *conf.Data, logger log.Logger) (*redis.Client, error) {
	return nil, nil
	// 默认配置
	network := "tcp"
	addr := "localhost:6379"
	readTimeout := 3 * time.Second
	writeTimeout := 3 * time.Second

	redisConfig := c.Redis
	if redisConfig != nil {
		if redisConfig.Network != "" {
			network = redisConfig.Network
		}
		if redisConfig.Addr != "" {
			addr = redisConfig.Addr
		}
		if redisConfig.ReadTimeout != nil {
			readTimeout = redisConfig.ReadTimeout.AsDuration()
		}
		if redisConfig.WriteTimeout != nil {
			writeTimeout = redisConfig.WriteTimeout.AsDuration()
		}
	}

	client := redis.NewClient(&redis.Options{
		Network:      network,
		Addr:         addr,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		Password:     "mysecretpassword",
		DB:           0,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	log.NewHelper(logger).Info("Redis client initialized successfully")

	return client, nil
}

// NewCacheRepo 创建缓存仓库
func NewCacheRepo(data *Data) biz.CacheRepo {
	return &cacheRepo{data: data}
}

// cacheRepo 缓存仓库实现
type cacheRepo struct {
	data *Data
}

// GetClient 获取Redis客户端
func (c *cacheRepo) GetClient() *redis.Client {
	return c.data.redis
}
