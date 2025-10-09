package biz

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"auth/internal/biz/bo"
	"auth/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
)

// UserRepo 用户数据仓库接口
type UserRepo interface {
	Create(ctx context.Context, user *bo.User) error
	GetByID(ctx context.Context, id string) (*bo.User, error)
	GetByUsername(ctx context.Context, username string) (*bo.User, error)
	GetByEmail(ctx context.Context, email string) (*bo.User, error)
	GetByPhone(ctx context.Context, phone string) (*bo.User, error)
	Update(ctx context.Context, user *bo.User) error
	Delete(ctx context.Context, id string) error
	CheckPassword(ctx context.Context, userID string, password string) (bool, error)
	SaveToken(ctx context.Context, userID string, accessToken string, refreshToken string, accessExpires, refreshExpires int64) error
	InvalidateToken(ctx context.Context, userID string, refreshToken string) error
	GetTokenInfo(ctx context.Context, refreshToken string) (string, error)
	VerifyAccessToken(ctx context.Context, accessToken string) (string, int64, error) // 验证访问令牌
}

// CacheRepo 缓存仓库接口
type CacheRepo interface {
	GetClient() *redis.Client
}

// Auth 认证业务逻辑结构
type Auth struct {
	log         *log.Helper
	repo        UserRepo
	redisClient *redis.Client
	config      *conf.Bootstrap
}

// NewAuth 创建认证业务逻辑实例
func NewAuth(
	logger log.Logger,
	repo UserRepo,
	cache CacheRepo,
	c *conf.Bootstrap,
) *Auth {
	return &Auth{
		log:         log.NewHelper(logger),
		repo:        repo,
		redisClient: cache.GetClient(),
		config:      c,
	}
}

// Register 用户注册
func (a *Auth) Register(ctx context.Context, user *bo.User) (string, error) {
	// 验证用户名是否已存在
	if _, err := a.repo.GetByUsername(ctx, user.Username); err == nil {
		// return "", v1.ErrorUsernameAlreadyExists("username already exists")
		return "", errors.New("username already exists")
	}

	// 验证邮箱是否已存在
	if _, err := a.repo.GetByEmail(ctx, user.Email); err == nil {
		// return "", v1.ErrorEmailAlreadyExists("email already exists")
		return "", errors.New("email already exists")
	}

	// 验证手机号是否已存在（如果提供）
	if _, err := a.repo.GetByPhone(ctx, user.Phone); err == nil {
		// return "", v1.ErrorPhoneAlreadyExists("phone already exists")
		return "", errors.New("phone already exists")
	}

	// 创建用户
	if err := a.repo.Create(ctx, user); err != nil {
		a.log.WithContext(ctx).Errorf("Failed to create user: %v", err)
		// return "", v1.ErrorInternalError("failed to register user")
		return "", fmt.Errorf("failed to create user: %w", err)
	}

	return user.ID, nil
}

// Login 用户登录
func (a *Auth) Login(ctx context.Context, identifier, password string) (*LoginResult, error) {
	var user *bo.User
	var err error

	// 根据标识符类型（用户名、邮箱或手机号）查找用户
	if identifier == "" {
		// return nil, v1.ErrorInvalidArgument("identifier cannot be empty")
		return nil, errors.New("identifier cannot be empty")
	}

	// 尝试用用户名查找
	user, err = a.repo.GetByUsername(ctx, identifier)
	if err != nil {
		// 尝试用邮箱查找
		user, err = a.repo.GetByEmail(ctx, identifier)
		if err != nil {
			// 尝试用手机号查找
			user, err = a.repo.GetByPhone(ctx, identifier)
			if err != nil {
				// return nil, v1.ErrorUserNotFound("user not found")
				return nil, errors.New("user not found")
			}
		}
	}

	// 验证密码
	valid, err := a.repo.CheckPassword(ctx, user.ID, password)
	if err != nil {
		a.log.WithContext(ctx).Errorf("Failed to check password: %v", err)
		// return nil, v1.ErrorInternalError("failed to login")
		return nil, errors.New("failed to login")
	}
	if !valid {
		// return nil, v1.ErrorInvalidPassword("invalid password")
		return nil, errors.New("invalid password")
	}

	// 生成token
	accessToken, refreshToken, err := a.generateTokens(ctx, user.ID)
	if err != nil {
		a.log.WithContext(ctx).Errorf("Failed to generate tokens: %v", err)
		// return nil, v1.ErrorInternalError("failed to generate tokens")
		return nil, errors.New("failed to generate tokens")
	}

	// 保存token信息
	if err := a.repo.SaveToken(ctx, user.ID, accessToken, refreshToken, a.config.Auth.AccessTokenTtl, a.config.Auth.RefreshTokenTtl); err != nil {
		a.log.WithContext(ctx).Errorf("Failed to save token: %v", err)
		// return nil, v1.ErrorInternalError("failed to save token")
		return nil, fmt.Errorf("failed to save token: %w", err)
	}

	return &LoginResult{
		UserID:           user.ID,
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		AccessExpiresIn:  a.config.Auth.AccessTokenTtl,
		RefreshExpiresIn: a.config.Auth.RefreshTokenTtl,
	}, nil
}

// Logout 用户登出
func (a *Auth) Logout(ctx context.Context, userID string, refreshToken string) error {
	// 使token失效
	if err := a.repo.InvalidateToken(ctx, userID, refreshToken); err != nil {
		a.log.WithContext(ctx).Errorf("Failed to invalidate token: %v", err)
		// return v1.ErrorInternalError("failed to logout")
		return fmt.Errorf("failed to invalidate token: %w", err)
	}

	return nil
}

// RevokeUser 注销用户（永久删除账户）
func (a *Auth) RevokeUser(ctx context.Context, userID string, password string) error {
	// 验证用户密码
	valid, err := a.repo.CheckPassword(ctx, userID, password)
	if err != nil {
		a.log.WithContext(ctx).Errorf("Failed to check password: %v", err)
		// return v1.ErrorInternalError("failed to revoke user")
		return errors.New("failed to revoke user")
	}
	if !valid {
		// return v1.ErrorInvalidPassword("invalid password")
		return errors.New("invalid password")
	}

	// 删除用户
	if err := a.repo.Delete(ctx, userID); err != nil {
		a.log.WithContext(ctx).Errorf("Failed to delete user: %v", err)
		// return v1.ErrorInternalError("failed to revoke user")
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}

// RefreshToken 刷新访问令牌
func (a *Auth) RefreshToken(ctx context.Context, refreshToken string) (*TokenResult, error) {
	// 获取用户ID
	userID, err := a.repo.GetTokenInfo(ctx, refreshToken)
	if err != nil {
		a.log.WithContext(ctx).Errorf("Failed to get token info: %v", err)
		// return nil, v1.ErrorInvalidToken("invalid refresh token")
		return nil, errors.New("invalid refresh token")
	}

	// 生成新的access token
	newAccessToken, err := a.generateAccessToken(ctx, userID)
	if err != nil {
		a.log.WithContext(ctx).Errorf("Failed to generate access token: %v", err)
		// return nil, v1.ErrorInternalError("failed to refresh token")
		return nil, errors.New("failed to generate access token")
	}

	// 保存新的access token信息（这里复用原有的refresh token）
	if err := a.repo.SaveToken(ctx, userID, newAccessToken, refreshToken, a.config.Auth.AccessTokenTtl, a.config.Auth.RefreshTokenTtl); err != nil {
		a.log.WithContext(ctx).Errorf("Failed to save token: %v", err)
		// return nil, v1.ErrorInternalError("failed to save token")
		return nil, fmt.Errorf("failed to save token: %w", err)
	}

	return &TokenResult{
			AccessToken:     newAccessToken,
			AccessExpiresIn: a.config.Auth.AccessTokenTtl,
		},
		nil
}

// LoginResult 登录结果结构
type LoginResult struct {
	UserID           string
	AccessToken      string
	RefreshToken     string
	AccessExpiresIn  int64
	RefreshExpiresIn int64
}

// TokenResult 令牌结果结构
type TokenResult struct {
	AccessToken     string
	AccessExpiresIn int64
}

// VerifyTokenResult 验证令牌结果结构
type VerifyTokenResult struct {
	Username  string
	UserID    string
	ExpiresAt int64
}

// generateTokens 生成访问令牌和刷新令牌
func (a *Auth) generateTokens(ctx context.Context, userID string) (accessToken string, refreshToken string, err error) {
	accessToken, err = a.generateAccessToken(ctx, userID)
	if err != nil {
		return "", "", err
	}

	refreshToken, err = a.generateRefreshToken(ctx, userID)
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

// generateAccessToken 生成访问令牌
func (a *Auth) generateAccessToken(ctx context.Context, userID string) (string, error) {
	// 在实际应用中，这里应该使用JWT等标准方式生成token
	// 这里简化实现，生成一个随机字符串作为示例
	token, err := generateRandomString(32)
	if err != nil {
		return "", fmt.Errorf("failed to generate access token: %w", err)
	}

	// 添加用户标识和时间戳以增强唯一性
	return fmt.Sprintf("access_%s_%s_%d", userID[:8], token, time.Now().Unix()), nil
}

// generateRefreshToken 生成刷新令牌
func (a *Auth) generateRefreshToken(ctx context.Context, userID string) (string, error) {
	// 在实际应用中，这里应该使用安全的随机生成方式
	token, err := generateRandomString(48)
	if err != nil {
		return "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// 添加用户标识和时间戳以增强唯一性
	return fmt.Sprintf("refresh_%s_%s_%d", userID[:8], token, time.Now().Unix()), nil
}

// generateRandomString 生成指定长度的随机字符串
func generateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length], nil
}

// VerifyToken 验证访问令牌
func (a *Auth) VerifyToken(ctx context.Context, accessToken string) (*VerifyTokenResult, error) {
	if accessToken == "" {
		a.log.WithContext(ctx).Error("Access token cannot be empty")
		return nil, errors.New("access token cannot be empty")
	}

	// 调用数据仓库验证令牌
	userID, expiresAt, err := a.repo.VerifyAccessToken(ctx, accessToken)
	if err != nil {
		a.log.WithContext(ctx).Errorf("Failed to verify access token: %v", err)
		return nil, err
	}

	// 获取用户信息以获取用户名
	user, err := a.repo.GetByID(ctx, userID)
	if err != nil {
		a.log.WithContext(ctx).Errorf("Failed to get user info: %v", err)
		return nil, err
	}

	// 验证成功
	return &VerifyTokenResult{
			Username:  user.Username,
			UserID:    userID,
			ExpiresAt: expiresAt,
		},
		nil
}
