package data

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"auth/internal/biz"
	"auth/internal/biz/bo"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// NewUserRepo 创建用户数据仓库
func NewUserRepo(data *Data, logger log.Logger) biz.UserRepo {
	return &userRepo{
		data:   data,
		log:    log.NewHelper(logger),
		client: data.redis,
	}
}

// userRepo 用户数据仓库实现
type userRepo struct {
	data   *Data
	log    *log.Helper
	client *redis.Client
}

// 用户相关的Redis键
const (
	UserIDPrefix       = "user:id:"
	UsernamePrefix     = "user:username:"
	EmailPrefix        = "user:email:"
	PhonePrefix        = "user:phone:"
	RefreshTokenPrefix = "token:refresh:"
	UserTokenPrefix    = "user:token:"
)

// Create 创建用户
func (r *userRepo) Create(ctx context.Context, user *bo.User) error {
	// 生成唯一用户ID
	if user.ID == "" {
		user.ID = uuid.New().String()
	}

	// 序列化用户数据
	userData, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("failed to marshal user: %w", err)
	}

	// 事务：存储用户数据并建立索引
	pipe := r.client.Pipeline()
	pipe.Set(ctx, UserIDPrefix+user.ID, userData, 0)        // 永久存储
	pipe.Set(ctx, UsernamePrefix+user.Username, user.ID, 0) // 用户名到ID的映射
	pipe.Set(ctx, EmailPrefix+user.Email, user.ID, 0)       // 邮箱到ID的映射
	if user.Phone != "" {
		pipe.Set(ctx, PhonePrefix+user.Phone, user.ID, 0) // 手机号到ID的映射（如果有）
	}
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	r.log.Infof("Created user: %s", user.Username)
	return nil
}

// GetByID 根据ID获取用户
func (r *userRepo) GetByID(ctx context.Context, id string) (*bo.User, error) {
	userData, err := r.client.Get(ctx, UserIDPrefix+id).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("user not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get user by ID: %w", err)
	}

	var user bo.User
	if err := json.Unmarshal(userData, &user); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user: %w", err)
	}

	return &user, nil
}

// GetByUsername 根据用户名获取用户
func (r *userRepo) GetByUsername(ctx context.Context, username string) (*bo.User, error) {
	userID, err := r.client.Get(ctx, UsernamePrefix+username).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("user not found by username: %s", username)
		}
		return nil, fmt.Errorf("failed to get user by username: %w", err)
	}

	return r.GetByID(ctx, userID)
}

// GetByEmail 根据邮箱获取用户
func (r *userRepo) GetByEmail(ctx context.Context, email string) (*bo.User, error) {
	userID, err := r.client.Get(ctx, EmailPrefix+email).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("user not found by email: %s", email)
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}

	return r.GetByID(ctx, userID)
}

// GetByPhone 根据手机号获取用户
func (r *userRepo) GetByPhone(ctx context.Context, phone string) (*bo.User, error) {
	userID, err := r.client.Get(ctx, PhonePrefix+phone).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("user not found by phone: %s", phone)
		}
		return nil, fmt.Errorf("failed to get user by phone: %w", err)
	}

	return r.GetByID(ctx, userID)
}

// Update 更新用户信息
func (r *userRepo) Update(ctx context.Context, user *bo.User) error {
	// 序列化用户数据
	userData, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("failed to marshal user: %w", err)
	}

	// 更新用户数据
	if err := r.client.Set(ctx, UserIDPrefix+user.ID, userData, 0).Err(); err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	r.log.Infof("Updated user: %s", user.ID)
	return nil
}

// Delete 删除用户
func (r *userRepo) Delete(ctx context.Context, id string) error {
	// 获取用户信息
	user, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// 事务：删除用户数据和索引
	pipe := r.client.Pipeline()
	pipe.Del(ctx, UserIDPrefix+id)
	pipe.Del(ctx, UsernamePrefix+user.Username)
	pipe.Del(ctx, EmailPrefix+user.Email)
	if user.Phone != "" {
		pipe.Del(ctx, PhonePrefix+user.Phone)
	}
	// 删除用户的所有token
	pipe.Del(ctx, UserTokenPrefix+id)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	r.log.Infof("Deleted user: %s", id)
	return nil
}

// CheckPassword 验证密码
func (r *userRepo) CheckPassword(ctx context.Context, userID string, password string) (bool, error) {
	user, err := r.GetByID(ctx, userID)
	if err != nil {
		return false, err
	}

	// 这里应该使用密码验证逻辑，示例中简单比较
	// 实际应用中应使用bcrypt等加密方式
	return user.Password == password, nil
}

// SaveToken 保存用户的token信息
func (r *userRepo) SaveToken(ctx context.Context, userID string, accessToken string, refreshToken string, accessExpires, refreshExpires int64) error {
	// 事务：保存token信息
	pipe := r.client.Pipeline()
	// 存储refresh token到userID的映射，设置过期时间
	pipe.Set(ctx, RefreshTokenPrefix+refreshToken, userID, time.Duration(refreshExpires)*time.Second)
	// 存储用户的当前refresh token
	pipe.Set(ctx, UserTokenPrefix+userID, refreshToken, time.Duration(refreshExpires)*time.Second)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to save token: %w", err)
	}

	r.log.Infof("Saved token for user: %s", userID)
	return nil
}

// InvalidateToken 使token失效
func (r *userRepo) InvalidateToken(ctx context.Context, userID string, refreshToken string) error {
	// 事务：删除token信息
	pipe := r.client.Pipeline()
	pipe.Del(ctx, RefreshTokenPrefix+refreshToken)
	pipe.Del(ctx, UserTokenPrefix+userID)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to invalidate token: %w", err)
	}

	r.log.Infof("Invalidated token for user: %s", userID)
	return nil
}

// GetTokenInfo 获取token信息（通过refresh token获取userID）
func (r *userRepo) GetTokenInfo(ctx context.Context, refreshToken string) (string, error) {
	userID, err := r.client.Get(ctx, RefreshTokenPrefix+refreshToken).Result()
	if err != nil {
		if err == redis.Nil {
			return "", fmt.Errorf("invalid refresh token")
		}
		return "", fmt.Errorf("failed to get token info: %w", err)
	}

	return userID, nil
}
