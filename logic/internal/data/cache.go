package data

import (
	"logic/internal/biz"

	"github.com/redis/go-redis/v9"
)

type cache struct {
	redisClient *redis.Client
}

func NewCache(data *Data) biz.CacheRepo {
	return &cache{redisClient: data.redis}
}

func (c *cache) GetClient() *redis.Client {
	return c.redisClient
}
