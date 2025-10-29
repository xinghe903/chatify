package data

import (
	"context"
	"fmt"
	"logic/internal/biz"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
)

const (
	redisDedupKeyPrefix  = "kafka:dedup:msg:"
	redisDedupExpiration = 24 * time.Hour // 1天过期时间
)

var _ biz.MessageDedupRepo = (*messageDedupRepo)(nil)

// messageDedupRepo 消息去重仓库实现
type messageDedupRepo struct {
	redisClient *redis.Client
	log         *log.Helper
}

// NewMessageDedupRepo 创建消息去重仓库
func NewMessageDedupRepo(data *Data, logger log.Logger) biz.MessageDedupRepo {
	return &messageDedupRepo{
		redisClient: data.redisClient,
		log:         log.NewHelper(logger),
	}
}

// CheckAndSetDedup 检查消息是否已消费，如果未消费则标记为已消费
// 使用 Redis Set 集合存储消息ID，key格式为：kafka:dedup:msg:{msgId}
// 返回true表示消息未被消费过，false表示消息已被消费过
func (r *messageDedupRepo) CheckAndSetDedup(ctx context.Context, msgId string) (bool, error) {
	if msgId == "" {
		return false, fmt.Errorf("msgId cannot be empty")
	}

	// 格式化Redis key: kafka:dedup:msg:{msgId}
	key := redisDedupKeyPrefix + msgId

	// 使用 Redis Set 的 SADD 命令添加消息ID到集合中
	// SADD 返回添加的元素个数：
	// - 返回 1：表示该元素是新的，成功添加到集合
	// - 返回 0：表示该元素已存在于集合中，已消费过
	result, err := r.redisClient.SAdd(ctx, key, msgId).Result()
	if err != nil {
		return false, fmt.Errorf("redis sadd failed: %w", err)
	}

	// 如果消息ID是新添加的（result > 0），设置过期时间为1天
	if result > 0 {
		if err := r.redisClient.Expire(ctx, key, redisDedupExpiration).Err(); err != nil {
			r.log.Warnf("failed to set expire for key %s: %v", key, err)
			// 不返回错误，因为主要功能（去重）已经完成
		}
		return true, nil // 消息未被消费过
	}

	return false, nil // 消息已被消费过（Set 中已存在）
}

