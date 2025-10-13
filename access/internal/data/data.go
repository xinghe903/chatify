package data

import (
	"access/internal/conf"
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(NewData, NewEtcdClient, NewRegistry, NewSessionRepo)

// Data .
type Data struct {
	mq          *MqConsumer
	redisClient *redis.Client
}

func NewData(c *conf.Bootstrap, logger log.Logger) (*Data, func(), error) {
	// 初始化Kafka消费者
	kafkaConsumer, err := NewMqConsumer(c.Data)
	if err != nil {
		return nil, nil, err
	}

	cleanup := func() {
		kafkaConsumer.Close()
		log.NewHelper(logger).Info("closing the data resources")
	}
	rdb, err := initRedisClient(c, logger)
	if err != nil {
		panic(err)
	}

	return &Data{
		mq:          kafkaConsumer,
		redisClient: rdb,
	}, cleanup, nil
}

type MqConsumer struct {
}

// NewMqConsumer 创建Kafka消费者实例
func NewMqConsumer(c *conf.Data) (*MqConsumer, error) {

	return &MqConsumer{}, nil
}

// Close 关闭消费者连接
func (kc *MqConsumer) Close() error {
	return nil
}

// GetReader 获取Kafka reader（供biz层调用）
func (kc *MqConsumer) GetReader() {

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
