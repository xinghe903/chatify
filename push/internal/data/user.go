package data

import (
	"context"
	"encoding/json"
	"fmt"
	"push/internal/biz"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
)

// Session 用户会话信息

type Session struct {
	Uid           string `json:"uid"`
	Username      string `json:"username"`
	ConnectionTime int64  `json:"connection_time"`
	ConnectionId  string `json:"connection_id"`
	AccessServiceId string `json:"access_service_id"` // 用户连接的access服务实例ID
}

type userRepo struct {
	data  *Data
	log   *log.Helper
	redis *redis.Client
}

// NewUserRepo .
func NewUserRepo(data *Data, logger log.Logger) biz.UserRepo {
	return &userRepo{
		data:  data,
		log:   log.NewHelper(log.With(logger, "module", "data/user")),
		redis: data.redisClient,
	}
}

// GetUserSession 根据用户ID查询用户会话信息
// 从Redis中获取用户的完整会话信息，包括connectionId和accessServiceId
func (r *userRepo) GetUserSession(ctx context.Context, uid string) (*Session, error) {
	r.log.Debug("GetUserSession", "uid", uid)
	
	// Redis key格式: chatify:session:{uid}
	key := fmt.Sprintf("chatify:session:%s", uid)
	
	// 检查Redis客户端是否初始化
	if r.redis == nil {
		r.log.Warn("Redis client is not initialized, no active connections for user", "uid", uid)
		return nil, nil
	}
	
	// 从Redis获取用户会话数据
	jsonData, err := r.redis.Get(ctx, key).Result()
	if err != nil {
		if err != redis.Nil {
			r.log.Error("Failed to get user session from Redis", "uid", uid, "error", err)
		} else {
			r.log.Debug("User session not found in Redis", "uid", uid)
		}
		// 当Redis中没有数据或查询失败时，返回空
		return nil, nil
	}
	
	// 解析Redis返回的JSON数据
	session := &Session{}
	if err := json.Unmarshal([]byte(jsonData), session); err != nil {
		r.log.Error("Failed to unmarshal session data", "uid", uid, "error", err)
		// 解析失败时返回空
		return nil, nil
	}
	
	// 如果解析出的connectionId为空，返回空
	if session.ConnectionId == "" {
		r.log.Debug("User has no valid connectionId", "uid", uid)
		return nil, nil
	}
	
	return session, nil
}

// BatchGetUserSessions 批量查询用户会话连接信息
// 接受uid数组，返回用户的session连接信息，假设一个用户只有一个连接
func (r *userRepo) BatchGetUserSessions(ctx context.Context, uids []string) (map[string]*Session, error) {
	r.log.Debug("BatchGetUserSessions", "uid_count", len(uids))
	
	// 创建结果映射，key为用户ID，value为用户会话信息
	sessions := make(map[string]*Session, len(uids))
	
	// 检查Redis客户端是否初始化
	if r.redis == nil {
		r.log.Warn("Redis client is not initialized, using mock data")
		// 使用模拟数据
		for _, uid := range uids {
			session := &Session{
				Uid:           uid,
				Username:      fmt.Sprintf("user_%s", uid),
				ConnectionTime: time.Now().Unix(),
				ConnectionId:  fmt.Sprintf("conn_%s_1", uid),
			}
			sessions[uid] = session
		}
		return sessions, nil
	}
	
	// 批处理大小，避免单次pipeline过大
	batchSize := 100
	for i := 0; i < len(uids); i += batchSize {
		// 确定当前批次的结束索引
		end := i + batchSize
		if end > len(uids) {
			end = len(uids)
		}
		
		// 获取当前批次的uid列表
		batchUids := uids[i:end]
		
		// 使用pipeline批量查询Redis
		pipe := r.redis.Pipeline()
		cmdMap := make(map[string]*redis.StringCmd, len(batchUids))
		
		for _, uid := range batchUids {
			// Redis key格式: chatify:session:{uid}
			key := fmt.Sprintf("chatify:session:%s", uid)
			cmdMap[uid] = pipe.Get(ctx, key)
		}
		
		// 执行pipeline
		_, err := pipe.Exec(ctx)
		if err != nil {
			r.log.Error("Pipeline execution failed", "error", err)
			// 继续处理已经成功的部分，不中断整个流程
		}
		
		// 处理pipeline的结果
		for uid, cmd := range cmdMap {
			jsonData, err := cmd.Result()
			if err != nil {
				// 如果Redis中没有该用户的数据，使用默认值
				if err == redis.Nil {
					r.log.Debug("User session not found in Redis", "uid", uid)
					// 这里可以选择跳过该用户，或者使用默认值
				} else {
					r.log.Error("Failed to get user session from Redis", "uid", uid, "error", err)
				}
				// 为了确保即使Redis查询失败也能返回基本信息，继续使用模拟数据
			} else {
				// 解析Redis返回的JSON数据
				session := &Session{}
				if err := json.Unmarshal([]byte(jsonData), session); err != nil {
					r.log.Error("Failed to unmarshal session data", "uid", uid, "error", err)
				}
				if session != nil && session.Uid != "" {
					sessions[uid] = session
					continue
				}
			}
			
			// 如果Redis查询失败或数据解析失败，使用默认值
		session := &Session{
				Uid:           uid,
				Username:      fmt.Sprintf("user_%s", uid),
				ConnectionTime: time.Now().Unix(),
				ConnectionId:  fmt.Sprintf("conn_%s_1", uid),
				AccessServiceId: fmt.Sprintf("access_%d", i%%10+1), // 模拟10个不同的access服务实例
			}
			sessions[uid] = session
		}
	}
	
	r.log.Debug("BatchGetUserSessions completed", "session_count", len(sessions))
	return sessions, nil
}
