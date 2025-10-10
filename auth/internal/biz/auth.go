package biz

import (
	v1 "api/auth/v1"
	"context"
	"errors"
	"fmt"
	"pkg/auth"
	"pkg/verify"
	"strings"

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
	SaveToken(ctx context.Context, tokenInfo *bo.LoginResult) error
	InvalidateToken(ctx context.Context, userID string, refreshToken string) error
	InvalidateAllTokens(ctx context.Context, userID string) error
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
	// 用户名规则： 不允许为手机号,且不能包含特殊符号（@, 空格），max length 20
	if len(user.Username) > bo.UserNameMaxLength {
		return "", v1.ErrorUserNameInvalid("username is too long")
	}
	if verify.IsMobileNumber(user.Username) {
		return "", v1.ErrorUserNameInvalid("username cannot be a mobile number")
	}
	//  检查是否包含禁止字符：@ 或 空格
	if strings.ContainsRune(user.Username, '@') || strings.ContainsAny(user.Username, " \t\n\r") {
		return "", v1.ErrorUserNameInvalid("username cannot contain special characters(@ or space)")
	}

	if len(user.Phone) != 0 {
		// 手机号规则： 校验规则为中国手机号
		if !verify.IsMobileNumber(user.Phone) {
			return "", v1.ErrorPhoneInvalid("phone cannot be a mobile number")
		}
	}
	// 邮箱规则： 校验规则为邮箱 max length 50
	if len(user.Email) > bo.EmailMaxLength {
		return "", v1.ErrorEmailInvalid("email is too long")
	}
	if !verify.IsEmail(user.Email) {
		return "", v1.ErrorEmailInvalid("invalid email")
	}

	// 验证用户名是否已存在
	if _, err := a.repo.GetByUsername(ctx, user.Username); err == nil {
		return "", v1.ErrorUserNameInvalid("username already exists")
	}

	// 验证邮箱是否已存在
	if _, err := a.repo.GetByEmail(ctx, user.Email); err == nil {
		return "", v1.ErrorEmailInvalid("email already exists")
	}

	// 验证手机号是否已存在
	if len(user.Phone) != 0 {
		if _, err := a.repo.GetByPhone(ctx, user.Phone); err == nil {
			return "", v1.ErrorPhoneInvalid("phone already exists")
		}
	}

	var err error
	user.Password, err = auth.HashPassword(user.Password)
	if err != nil {
		a.log.WithContext(ctx).Errorf("Failed to hash password: %v", err)
		return "", v1.ErrorPasswordBcryptFailed("failed to hash password")
	}
	if err := a.repo.Create(ctx, user); err != nil {
		a.log.WithContext(ctx).Errorf("Failed to create user: %v", err)
		return "", v1.ErrorCreateUserFailed("failed to create user")
	}

	return user.ID, nil
}

// Login 用户登录
func (a *Auth) Login(ctx context.Context, identifier, password string) (*bo.LoginResult, error) {
	var user *bo.User
	var err error
	// 尝试通过用户名、邮箱或手机号登录
	if user, err = a.repo.GetByUsername(ctx, identifier); err != nil {
		if user, err = a.repo.GetByEmail(ctx, identifier); err != nil {
			if user, err = a.repo.GetByPhone(ctx, identifier); err != nil {
				a.log.WithContext(ctx).Infof("Failed to get user: %v", err)
				return nil, v1.ErrorNameOrPasswordInvalid("user not found or invalid credentials")
			}
		}
	}

	// 检查用户状态
	if user.Status != bo.UserStatusActive {
		statusMsg := "user is not active"
		if user.Status == bo.UserStatusRevoked {
			statusMsg = "user account has been revoked"
		} else if user.Status == bo.UserStatusLocked {
			statusMsg = "user account is locked"
		}
		return nil, v1.ErrorUserStatusInvalid(statusMsg)
	}

	// 验证密码
	if err := auth.CheckPassword(user.Password, password); err != nil {
		a.log.WithContext(ctx).Infof("Failed to check password: %v", err)
		return nil, v1.ErrorNameOrPasswordInvalid("user not found or invalid credentials")
	}

	// 生成token
	accessToken, refreshToken, err := a.generateTokens(ctx, user.ID)
	if err != nil {
		a.log.WithContext(ctx).Errorf("Failed to generate tokens: %v", err)
		return nil, v1.ErrorGenerateIdFailed("failed to generate tokens")
	}

	// 保存token信息
	if err := a.repo.SaveToken(ctx, &bo.LoginResult{
		UserID:           user.ID,
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		AccessExpiresIn:  a.config.Auth.AccessTokenTtl,
		RefreshExpiresIn: a.config.Auth.RefreshTokenTtl,
	}); err != nil {
		a.log.Errorf("Failed to save token: %v", err)
		return nil, fmt.Errorf("failed to save token: %w", err)
	}

	return &bo.LoginResult{
		UserID: user.ID,
		// 原代码报错是因为 LoginResult 结构体中没有 Username 字段，此处移除该字段
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		AccessExpiresIn:  a.config.Auth.AccessTokenTtl,
		RefreshExpiresIn: a.config.Auth.RefreshTokenTtl,
	}, nil
}

// Logout 用户登出
func (a *Auth) Logout(ctx context.Context, userID string, refreshToken string) error {
	// 输入验证
	if userID == "" {
		return errors.New("userID cannot be empty")
	}

	if refreshToken == "" {
		return errors.New("refresh token cannot be empty")
	}

	// 验证用户是否存在
	user, err := a.repo.GetByID(ctx, userID)
	if err != nil {
		a.log.Warnf("Attempt to logout non-existent user: %s", userID)
		return errors.New("user not found")
	}

	// 检查刷新令牌是否属于该用户
	tokenUserID, err := a.repo.GetTokenInfo(ctx, refreshToken)
	if err != nil {
		a.log.Warnf("Attempt to logout with invalid token: user=%s", userID)
		return errors.New("invalid refresh token")
	}

	if tokenUserID != userID {
		a.log.Warnf("Token ownership mismatch: token belongs to %s, logout attempted by %s", tokenUserID, userID)
		return errors.New("token does not belong to user")
	}

	// 使令牌失效
	if err := a.repo.InvalidateToken(ctx, userID, refreshToken); err != nil {
		a.log.Errorf("Failed to invalidate token for user %s: %v", user.Username, err)
		return fmt.Errorf("failed to invalidate token: %w", err)
	}

	a.log.Infof("User logged out successfully: %s", user.Username)
	return nil
}

// RevokeUser 注销用户
func (a *Auth) RevokeUser(ctx context.Context, userID string, password string) error {
	// 输入验证
	if userID == "" {
		return errors.New("userID cannot be empty")
	}
	if password == "" {
		return errors.New("password cannot be empty")
	}

	// 验证用户是否存在
	user, err := a.repo.GetByID(ctx, userID)
	if err != nil {
		return errors.New("user not found")
	}

	// 检查用户状态
	if user.Status == bo.UserStatusRevoked {
		return errors.New("user account is already revoked")
	}

	// 验证密码
	valid, err := a.repo.CheckPassword(ctx, user.ID, password)
	if err != nil {
		a.log.Errorf("Failed to check password: %v", err)
		return errors.New("failed to verify password")
	}
	if !valid {
		a.log.Warnf("Failed revocation attempt for user: %s", user.Username)
		return errors.New("invalid password")
	}

	// 注销用户
	if err := a.repo.Delete(ctx, user.ID); err != nil {
		a.log.Errorf("Failed to revoke user: %v", err)
		return fmt.Errorf("failed to revoke user: %w", err)
	}

	// 使所有令牌失效
	if err := a.repo.InvalidateAllTokens(ctx, user.ID); err != nil {
		a.log.Errorf("Failed to invalidate tokens: %v", err)
		// 不阻止注销操作，但记录错误
	}

	return nil
}

// RefreshToken 刷新访问令牌
func (a *Auth) RefreshToken(ctx context.Context, refreshToken string) (*bo.TokenResult, error) {
	// 输入验证
	if refreshToken == "" {
		return nil, errors.New("refresh token cannot be empty")
	}

	// 检查刷新令牌是否有效
	userID, err := a.repo.GetTokenInfo(ctx, refreshToken)
	if err != nil {
		a.log.Warnf("Attempt to use invalid refresh token")
		return nil, errors.New("invalid refresh token")
	}

	// 验证用户是否存在且处于活动状态
	user, err := a.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	if user.Status != bo.UserStatusActive {
		statusMsg := "user is not active"
		if user.Status == bo.UserStatusRevoked {
			statusMsg = "user account has been revoked"
		} else if user.Status == bo.UserStatusLocked {
			statusMsg = "user account is locked"
		}
		return nil, errors.New(statusMsg)
	}

	// 事务处理：使旧令牌失效并生成新令牌
	// 1. 使旧令牌失效
	if err := a.repo.InvalidateToken(ctx, userID, refreshToken); err != nil {
		a.log.Errorf("Failed to invalidate token: %v", err)
		// 不阻止刷新操作，但记录错误
	}

	// 2. 生成新令牌
	newAccessToken, newRefreshToken, err := a.generateTokens(ctx, userID)
	if err != nil {
		a.log.Errorf("Failed to generate tokens: %v", err)
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	// 3. 保存新令牌
	if err := a.repo.SaveToken(ctx, &bo.LoginResult{
		UserID:           userID,
		AccessToken:      newAccessToken,
		RefreshToken:     newRefreshToken,
		AccessExpiresIn:  a.config.Auth.AccessTokenTtl,
		RefreshExpiresIn: a.config.Auth.RefreshTokenTtl,
	}); err != nil {
		a.log.Errorf("Failed to save token: %v", err)
		return nil, fmt.Errorf("failed to save token: %w", err)
	}

	return &bo.TokenResult{
			AccessToken:     newAccessToken,
			AccessExpiresIn: a.config.Auth.AccessTokenTtl,
		},
		nil
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

// generateAccessToken 生成访问令牌（Opaque Token方式）
func (a *Auth) generateAccessToken(ctx context.Context, userID string) (string, error) {
	// Opaque Token不包含任何用户信息，仅作为服务器端的引用
	// 使用base64.URLEncoding.WithPadding(base64.NoPadding)生成24长度的随机字符串
	token, err := auth.GenerateRandomString(24)
	if err != nil {
		a.log.Errorf("Failed to generate secure random bytes for access token: %v", err)
		return "", fmt.Errorf("failed to generate access token: %w", err)
	}

	// 添加前缀标识令牌类型，但不包含用户ID或时间戳等敏感信息
	return fmt.Sprintf("access_%s", token), nil
}

// generateRefreshToken 生成刷新令牌（Opaque Token方式）
func (a *Auth) generateRefreshToken(ctx context.Context, userID string) (string, error) {
	// 使用base64.URLEncoding.WithPadding(base64.NoPadding)生成24长度的随机字符串
	token, err := auth.GenerateRandomString(24)
	if err != nil {
		a.log.Errorf("Failed to generate secure random bytes for refresh token: %v", err)
		return "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// 添加前缀标识令牌类型
	return fmt.Sprintf("refresh_%s", token), nil
}

// VerifyToken 验证访问令牌
func (a *Auth) VerifyToken(ctx context.Context, accessToken string) (*bo.VerifyTokenResult, error) {
	// 输入验证
	if accessToken == "" {
		a.log.Error("Access token cannot be empty")
		return nil, errors.New("access token cannot be empty")
	}

	// 调用数据仓库验证令牌
	userID, expiresAt, err := a.repo.VerifyAccessToken(ctx, accessToken)
	if err != nil {
		a.log.Warnf("Failed to verify access token: %v", err)
		return nil, errors.New("invalid access token")
	}

	// 验证用户是否存在且处于活动状态
	user, err := a.repo.GetByID(ctx, userID)
	if err != nil {
		a.log.Warnf("Token is valid but user not found: %s", userID)
		return nil, errors.New("user not found")
	}

	if user.Status != bo.UserStatusActive {
		statusMsg := "user is not active"
		if user.Status == bo.UserStatusRevoked {
			statusMsg = "user account has been revoked"
		} else if user.Status == bo.UserStatusLocked {
			statusMsg = "user account is locked"
		}
		a.log.Warnf("Token valid but user status invalid: %s, status: %s", user.Username, user.Status)
		return nil, errors.New(statusMsg)
	}

	// 验证成功
	return &bo.VerifyTokenResult{
			UserID:    userID,
			Username:  user.Username,
			Email:     user.Email,
			ExpiresAt: expiresAt,
		},
		nil
}
