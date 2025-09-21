package sign

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
)

var (
	ErrInvalidSign    = errors.New("invalid signature")
	ErrRequestExpired = errors.New("request expired")
	ErrRequestRepeat  = errors.New("replay attack detected")
)

// Cache 缓存接口，定义了缓存操作的基本方法
// 可以有不同的实现，如Redis缓存、内存缓存等
type Cache interface {
	// Exists 检查键是否存在
	Exists(ctx context.Context, key string) (bool, error)
	// Set 设置键值对，并指定过期时间
	Set(ctx context.Context, key string, value string, expiration time.Duration) error
}

// RedisCache Redis缓存实现
// 使用Redis作为缓存存储
type RedisCache struct {
	client *redis.Client
}

// MemoryCache 内存缓存实现
// 使用本地内存作为缓存存储，并支持设置最大存储数量
type MemoryCache struct {
	items      map[string]string    // 存储的键值对
	expiration map[string]time.Time // 过期时间映射
	maxSize    int                  // 最大存储数量
	keys       []string             // 用于FIFO策略的键顺序
	mutex      sync.RWMutex         // 用于并发控制的读写锁
}

// NewRedisCache 创建Redis缓存实例
func NewRedisCache(client *redis.Client) *RedisCache {
	return &RedisCache{
		client: client,
	}
}

// NewMemoryCache 创建内存缓存实例
// maxSize: 最大存储数量，小于等于0表示不限制
func NewMemoryCache(maxSize int) *MemoryCache {
	// 如果maxSize小于等于0，则设置为一个较大的值表示不限制
	if maxSize <= 0 {
		maxSize = 1000000 // 默认最大存储100万个键值对
	}

	return &MemoryCache{
		items:      make(map[string]string),
		expiration: make(map[string]time.Time),
		maxSize:    maxSize,
		keys:       make([]string, 0),
	}
}

// Exists 检查键是否存在
func (mc *MemoryCache) Exists(ctx context.Context, key string) (bool, error) {
	mc.mutex.RLock()
	defer mc.mutex.RUnlock()

	// 检查键是否存在
	_, exists := mc.items[key]
	if !exists {
		return false, nil
	}

	// 检查是否已过期
	if expiration, ok := mc.expiration[key]; ok {
		if time.Now().After(expiration) {
			// 异步删除过期的键（在写锁保护下）
			go func() {
				mc.mutex.Lock()
				defer mc.mutex.Unlock()
				mc.removeKey(key)
			}()
			return false, nil
		}
	}

	return true, nil
}

// Set 设置键值对，并指定过期时间
func (mc *MemoryCache) Set(ctx context.Context, key string, value string, expiration time.Duration) error {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	// 检查是否已达到最大存储数量，并且要添加的是新键
	if _, exists := mc.items[key]; !exists && len(mc.items) >= mc.maxSize {
		// 使用FIFO策略删除最旧的键
		if len(mc.keys) > 0 {
			oldestKey := mc.keys[0]
			mc.removeKey(oldestKey)
		}
	}

	// 更新或添加键值对
	if _, exists := mc.items[key]; !exists {
		// 如果是新键，添加到keys数组末尾
		mc.keys = append(mc.keys, key)
	}

	mc.items[key] = value

	// 设置过期时间（如果有的话）
	if expiration > 0 {
		mc.expiration[key] = time.Now().Add(expiration)
	} else {
		// 如果过期时间为0或负值，则移除过期时间设置
		delete(mc.expiration, key)
	}

	return nil
}

// removeKey 从缓存中删除指定的键
// 注意：调用此方法前必须获取写锁
func (mc *MemoryCache) removeKey(key string) {
	// 从items中删除
	delete(mc.items, key)
	// 从expiration中删除
	delete(mc.expiration, key)
	// 从keys数组中删除
	for i, k := range mc.keys {
		if k == key {
			mc.keys = append(mc.keys[:i], mc.keys[i+1:]...)
			break
		}
	}
}

// Exists 检查键是否存在
func (rc *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	result, err := rc.client.Exists(ctx, key).Result()
	return result > 0, err
}

// Set 设置键值对，并指定过期时间
func (rc *RedisCache) Set(ctx context.Context, key string, value string, expiration time.Duration) error {
	_, err := rc.client.Set(ctx, key, value, expiration).Result()
	return err
}

// SignParam 签名参数结构体
// 用于请求的签名和防重放攻击校验
type SignParam struct {
	RequestID string `json:"request_id"` // 请求ID，要求唯一性，客户端生成，服务端返回
	Timestamp int64  `json:"timestamp"`  // 消息发送时间戳 Unix时间戳 单位毫秒
	Signature string `json:"signature"`  // 签名，用于去重与校验 request_id + timestamp
}

// ReplayChecker 防重放校验器
// 负责处理请求的签名验证和防重放攻击逻辑
type ReplayChecker struct {
	cache      Cache // 缓存接口，用于存储已处理的请求ID
	expireTime int   // 请求ID的过期时间（秒）
	timeWindow int64 // 时间窗口（毫秒），用于验证时间戳是否在有效期内
}

// NewReplayChecker 创建基于Redis的防重放校验器实例
// redisClient: Redis客户端
// expireTime: 请求ID的过期时间（秒），默认3600秒
// timeWindow: 时间窗口（毫秒），默认300000毫秒（5分钟）
func NewReplayChecker(redisClient *redis.Client, expireTime, timeWindow int) *ReplayChecker {
	// 设置默认值
	if expireTime <= 0 {
		expireTime = int(1 * time.Hour.Seconds()) // 默认1小时过期
	}
	if timeWindow <= 0 {
		timeWindow = int(5 * time.Minute.Milliseconds()) // 默认5分钟时间窗口
	}

	// 创建Redis缓存实现
	cache := NewRedisCache(redisClient)

	return &ReplayChecker{
		cache:      cache,
		expireTime: expireTime,
		timeWindow: int64(timeWindow),
	}
}

// NewMemoryReplayChecker 创建基于内存的防重放校验器实例
// logger: 日志记录器
// expiry: 过期时间，默认5分钟
func NewMemoryReplayChecker(logger log.Logger, expiry time.Duration) *ReplayChecker {
	if expiry <= 0 {
		// 默认过期时间为5分钟
		expiry = 5 * time.Minute
	}

	// 创建内存缓存实现
	cache := NewMemoryCache(10000) // 设置一个合理的容量上限

	// 转换为秒
	expireTime := int(expiry.Seconds())
	// 转换为毫秒
	timeWindow := int64(expiry.Milliseconds())

	checker := &ReplayChecker{
		cache:      cache,
		expireTime: expireTime,
		timeWindow: timeWindow,
	}

	// 创建日志辅助器
	logHelper := log.NewHelper(logger)

	// 启动后台清理任务
	go func() {
		ticker := time.NewTicker(expiry / 2)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// 清理过期的缓存项
				logHelper.Debug("Running memory cache cleanup")
			}
		}
	}()

	return checker
}

// NewReplayCheckerWithCache 创建带自定义缓存的防重放校验器实例
// cache: 自定义缓存实现
// expireTime: 请求ID的过期时间（秒），默认3600秒
// timeWindow: 时间窗口（毫秒），默认300000毫秒（5分钟）
func NewReplayCheckerWithCache(cache Cache, expireTime, timeWindow int) *ReplayChecker {
	// 设置默认值
	if expireTime <= 0 {
		expireTime = 3600 // 默认1小时过期
	}
	if timeWindow <= 0 {
		timeWindow = 300000 // 默认5分钟时间窗口
	}

	return &ReplayChecker{
		cache:      cache,
		expireTime: expireTime,
		timeWindow: int64(timeWindow),
	}
}

// GenerateSignature 生成签名
// requestID: 请求ID
// timestamp: 时间戳
// secretKey: 密钥
// 返回: 十六进制编码的签名字符串
func GenerateSignature(requestID string, timestamp int64, secretKey string) string {
	// 构建待签名字符串
	message := requestID + "_" + string(timestamp)

	// 使用HMAC-SHA256生成签名
	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write([]byte(message))
	signature := h.Sum(nil)

	// 转换为十六进制字符串
	return hex.EncodeToString(signature)
}

// VerifySignature 验证签名
// signParam: 签名参数
// secretKey: 密钥
// 返回: 是否验证通过
func VerifySignature(signParam *SignParam, secretKey string) bool {
	// 生成期望的签名
	expectedSignature := GenerateSignature(signParam.RequestID, signParam.Timestamp, secretKey)

	// 比较签名是否匹配
	return hmac.Equal([]byte(signParam.Signature), []byte(expectedSignature))
}

// CheckReplay 检查请求是否为重放攻击
// ctx: 上下文
// requestID: 请求ID
// 返回: 是否为重放请求，错误信息
func (rc *ReplayChecker) CheckReplay(ctx context.Context, requestID string) (bool, error) {
	// 构建缓存键
	key := formatReplayCheckKey(requestID)

	// 检查请求ID是否已存在
	exists, err := rc.cache.Exists(ctx, key)
	if err != nil {
		return false, err
	}

	// 如果请求ID已存在，说明是重放请求
	if exists {
		return true, nil
	}

	// 将请求ID存入缓存，并设置过期时间
	err = rc.cache.Set(ctx, key, "1", time.Duration(rc.expireTime)*time.Second)
	if err != nil {
		return false, err
	}

	// 不是重放请求
	return false, nil
}

// CheckAndMark 检查消息ID是否已处理，并标记为已处理（兼容原有API）
// func (rc *ReplayChecker) CheckAndMark(ctx context.Context, messageID string) bool {
// 	if messageID == "" {
// 		return false
// 	}

// 	// 调用CheckReplay方法
// 	isReplay, err := rc.CheckReplay(ctx, messageID)
// 	if err != nil {
// 		return false
// 	}

// 	// 如果不是重放请求，则标记为已处理
// 	return !isReplay
// }

// ValidateRequest 验证请求是否有效（包括签名验证、时间窗口验证和防重放验证）
// ctx: 上下文
// signParam: 签名参数
// secretKey: 密钥
// 返回: 是否验证通过，错误信息
func (rc *ReplayChecker) ValidateRequest(ctx context.Context, signParam *SignParam, secretKey string) error {
	// 1. 验证签名
	if !VerifySignature(signParam, secretKey) {
		return ErrInvalidSign
	}

	// 2. 验证时间窗口
	now := time.Now().UnixNano() / int64(time.Millisecond)
	if now-signParam.Timestamp > rc.timeWindow {
		return ErrRequestExpired
	}

	// 3. 防重放验证
	isReplay, err := rc.CheckReplay(ctx, signParam.RequestID)
	if err != nil {
		return err
	}
	if isReplay {
		return ErrRequestRepeat
	}

	// 验证通过
	return nil
}

// EncodeSignParam 将SignParam编码为可传输的格式
// signParam: 签名参数
// 返回: 编码后的字节数组
func EncodeSignParam(signParam *SignParam) ([]byte, error) {
	return json.Marshal(signParam)
}

// DecodeSignParam 解码SignParam
// data: 编码后的字节数组
// 返回: 解码后的SignParam结构体
func DecodeSignParam(data []byte) (*SignParam, error) {
	var signParam SignParam
	err := json.Unmarshal(data, &signParam)
	if err != nil {
		return nil, err
	}
	return &signParam, nil
}

// formatReplayCheckKey 格式化缓存键
func formatReplayCheckKey(requestID string) string {
	return "chatify:sign:replay_check:" + requestID
}
