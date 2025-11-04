package data

import (
	"context"
	"fmt"
	"time"

	"github.com/xinghe903/chatify/access/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(NewData, NewEtcdClient, NewRegistry, NewSessionRepo, NewKafkaProducer)

// Data .
type Data struct {
	redisClient   *redis.Client
	kafkaProducer *KafkaProducer
}

func NewData(c *conf.Bootstrap, logger log.Logger) (*Data, func(), error) {
	cleanup := func() {
		log.NewHelper(logger).Info("closing the data resources")
	}
	rdb, err := initRedisClient(c, logger)
	if err != nil {
		panic(err)
	}

	return &Data{
		redisClient: rdb,
	}, cleanup, nil
}

func initRedisClient(cb *conf.Bootstrap, logg log.Logger) (*redis.Client, error) {
	c := cb.Data
	// 默认配置
	network := "tcp"
	addr := "localhost:6379"
	readTimeout := 3 * time.Second
	writeTimeout := 3 * time.Second

	// 从配置中读取Redis参数
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
		Password:     "", // 默认无密码，实际环境中应从配置读取
		DB:           0,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	log.NewHelper(logg).Info("Redis client initialized successfully")

	return client, nil
}
