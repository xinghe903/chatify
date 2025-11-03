package biz

import (
	v1 "api/auth/v1"
	"context"
	"errors"
	"fmt"
	"pkg/auth"

	"auth/internal/biz/bo"
	"auth/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
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
}

type SessionRepo interface {
	Upsert(ctx context.Context, tokenInfo *bo.LoginResult) error
	Delete(ctx context.Context, id string) error
	GetTokenInfo(ctx context.Context, refreshToken string) (string, error)
	VerifyAccessToken(ctx context.Context, accessToken string) (string, int64, error)
}

// Auth 认证业务逻辑结构
type Auth struct {
	log         *log.Helper
	userRepo    UserRepo
	sessionRepo SessionRepo
	config      *conf.Bootstrap
}

// NewAuth 创建认证业务逻辑实例
func NewAuth(
	logger log.Logger,
	userRepo UserRepo,
	sessionRepo SessionRepo,
	c *conf.Bootstrap,
) *Auth {
	return &Auth{
		log:         log.NewHelper(logger),
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		config:      c,
	}
}

// Register 用户注册
func (a *Auth) Register(ctx context.Context, user *bo.User) (string, error) {
	if err := checkField(NewUsernameCheck(a.userRepo, a.log), ctx, user.Username); err != nil {
		return "", err
	}
	if err := checkField(NewEmailCheck(a.userRepo, a.log), ctx, user.Email); err != nil {
		return "", err
	}
	if err := checkField(NewPhoneCheck(a.userRepo, a.log), ctx, user.Phone); err != nil {
		return "", err
	}
	var err error
	user.Password, err = auth.HashPassword(user.Password)
	if err != nil {
		return "", v1.ErrorPasswordBcryptFailed("failed to hash password")
	}
	if err := a.userRepo.Create(ctx, user); err != nil {
		return "", v1.ErrorCreateUserFailed("failed to create user")
	}

	return user.ID, nil
}

// Login 用户登录
func (a *Auth) Login(ctx context.Context, identifier, password string) (*bo.LoginResult, error) {
	var user *bo.User
	var err error
	// 尝试通过用户名、邮箱或手机号登录
	if user, err = a.userRepo.GetByUsername(ctx, identifier); err != nil || user == nil {
		if user, err = a.userRepo.GetByEmail(ctx, identifier); err != nil || user == nil {
			if user, err = a.userRepo.GetByPhone(ctx, identifier); err != nil || user == nil {
				if err == nil {
					err = errors.New("user not found")
				}
				a.log.WithContext(ctx).Infof("Failed to get user err=%v, identifier=%s", err, identifier)
				return nil, v1.ErrorNameOrPasswordInvalid("The username or password is incorrect")
			}
		}
	}
	// 检查用户状态
	if err := checkField(NewUserStatusCheck(a.log), ctx, string(user.Status)); err != nil {
		return nil, err
	}
	// 验证密码
	if err := auth.CheckPassword(user.Password, password); err != nil {
		a.log.WithContext(ctx).Infof("Failed to check password: %v", err)
		return nil, v1.ErrorNameOrPasswordInvalid("The username or password is incorrect")
	}

	// 清除旧token
	if err := a.sessionRepo.Delete(ctx, user.ID); err != nil {
		a.log.WithContext(ctx).Errorf("Failed to invalidate all tokens: %v", err)
		return nil, v1.ErrorGenerateIdFailed("failed to invalidate all tokens")
	}

	// 生成token
	accessToken, refreshToken, err := a.generateTokens(ctx, user.ID)
	if err != nil {
		a.log.WithContext(ctx).Errorf("Failed to generate tokens: %v", err)
		return nil, v1.ErrorGenerateIdFailed("failed to generate tokens")
	}

	// 保存token信息
	if err := a.sessionRepo.Upsert(ctx, &bo.LoginResult{
		UserID:           user.ID,
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		AccessExpiresIn:  a.config.Auth.AccessTokenTtl,
		RefreshExpiresIn: a.config.Auth.RefreshTokenTtl,
	}); err != nil {
		a.log.WithContext(ctx).Errorf("Failed to save token: %v", err)
		return nil, v1.ErrorSaveSessionFailed("failed to save token")
	}

	return &bo.LoginResult{
		UserID:           user.ID,
		Username:         user.Username,
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		AccessExpiresIn:  a.config.Auth.AccessTokenTtl,
		RefreshExpiresIn: a.config.Auth.RefreshTokenTtl,
	}, nil
}

// Logout 用户登出
func (a *Auth) Logout(ctx context.Context, userID string, refreshToken string) error {
	// 验证用户是否存在
	user, err := a.userRepo.GetByID(ctx, userID)
	if err != nil {
		a.log.WithContext(ctx).Infof("Failed to get user: %v", err)
		return v1.ErrorUserNotFound("user not found")
	}
	// 检查刷新令牌是否属于该用户
	tokenUserID, err := a.sessionRepo.GetTokenInfo(ctx, refreshToken)
	if err != nil {
		a.log.WithContext(ctx).Infof("Failed to get token info: %v. userId=%s, refreshToken=%s",
			err, userID, refreshToken)
		return v1.ErrorTokenInvalid("invalid refresh token")
	}
	if tokenUserID != userID {
		return v1.ErrorParamInvalid("token does not belong to user. userId=%s, refreshToken=%s",
			userID, refreshToken)
	}
	if err := a.sessionRepo.Delete(ctx, userID); err != nil {
		a.log.WithContext(ctx).Errorf("Failed to invalidate token for user userId=%s. err=%v", user.Username, err)
		return v1.ErrorClearSessionFailed("failed to clear session")
	}
	a.log.WithContext(ctx).Debugf("User logout successfully: %s", user.Username)
	return nil
}

// RevokeUser 注销用户
func (a *Auth) RevokeUser(ctx context.Context, userID string, password string) error {
	// 验证用户是否存在
	user, err := a.userRepo.GetByID(ctx, userID)
	if err != nil {
		a.log.WithContext(ctx).Infof("Failed to get user: %v", err)
		return v1.ErrorUserNotFound("user not found")
	}
	// 检查用户状态
	if user.Status != bo.UserStatusActive {
		return errors.New("user account is already revoked")
	}
	// 验证密码
	if err := auth.CheckPassword(user.Password, password); err != nil {
		a.log.WithContext(ctx).Infof("Failed to check password: %v", err)
		return v1.ErrorPasswordInvalid("The password is incorrect")
	}
	// 注销用户
	if err := a.userRepo.Delete(ctx, user.ID); err != nil {
		a.log.WithContext(ctx).Errorf("Failed to revoke user: %v", err)
		return v1.ErrorRevokeUserFailed("failed to revoke user")
	}
	if err := a.sessionRepo.Delete(ctx, user.ID); err != nil {
		a.log.WithContext(ctx).Errorf("Failed to invalidate tokens: %v", err)
		// 不阻止注销操作，但记录错误
	}
	return nil
}

// RefreshToken 刷新访问令牌
func (a *Auth) RefreshToken(ctx context.Context, refreshToken string) (*bo.TokenResult, error) {
	// 检查刷新令牌是否有效
	userId, err := a.sessionRepo.GetTokenInfo(ctx, refreshToken)
	if err != nil {
		a.log.WithContext(ctx).Infof("Failed to get token info: %v. refreshToken=%s", err, refreshToken)
		return nil, v1.ErrorTokenInvalid("invalid refresh token")
	}
	// 验证用户是否存在且处于活动状态
	user, err := a.userRepo.GetByID(ctx, userId)
	if err != nil {
		a.log.WithContext(ctx).Infof("Failed to get user: %v", err)
		return nil, v1.ErrorUserNotFound("user not found")
	}
	// 检查用户状态
	if err := checkField(NewUserStatusCheck(a.log), ctx, string(user.Status)); err != nil {
		return nil, err
	}
	// 生成新令牌
	newAccessToken, newRefreshToken, err := a.generateTokens(ctx, userId)
	if err != nil {
		a.log.WithContext(ctx).Errorf("Failed to generate tokens: %v", err)
		return nil, v1.ErrorRefreshTokenFailed("generate tokens failed")
	}
	// 旧令牌失效
	if err := a.sessionRepo.Delete(ctx, userId); err != nil {
		a.log.WithContext(ctx).Errorf("Failed to invalidate token: %v", err)
		return nil, v1.ErrorRefreshTokenFailed("invalidating old token failed")
	}
	// 保存新令牌
	if err := a.sessionRepo.Upsert(ctx, &bo.LoginResult{
		UserID:           userId,
		AccessToken:      newAccessToken,
		RefreshToken:     newRefreshToken,
		AccessExpiresIn:  a.config.Auth.AccessTokenTtl,
		RefreshExpiresIn: a.config.Auth.RefreshTokenTtl,
	}); err != nil {
		a.log.WithContext(ctx).Errorf("Failed to save token: %v", err)
		return nil, v1.ErrorRefreshTokenFailed("save new token failed")
	}
	return &bo.TokenResult{
			AccessToken:      newAccessToken,
			RefreshToken:     newRefreshToken,
			AccessExpiresIn:  a.config.Auth.AccessTokenTtl,
			RefreshExpiresIn: a.config.Auth.RefreshTokenTtl,
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
		return "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// 添加前缀标识令牌类型
	return fmt.Sprintf("refresh_%s", token), nil
}

// VerifyToken 验证访问令牌
func (a *Auth) VerifyToken(ctx context.Context, accessToken string) (*bo.VerifyTokenResult, error) {
	// 调用数据仓库验证令牌
	userID, expiresAt, err := a.sessionRepo.VerifyAccessToken(ctx, accessToken)
	if err != nil {
		a.log.WithContext(ctx).Warnf("Failed to verify token: %v", err)
		return nil, v1.ErrorVerifyTokenFailed("invalid access token")
	}

	// 验证用户是否存在且处于活动状态
	user, err := a.userRepo.GetByID(ctx, userID)
	if err != nil {
		a.log.WithContext(ctx).Infof("Failed to get user: %v", err)
		return nil, v1.ErrorUserNotFound("user not found")
	}
	// 检查用户状态
	if err := checkField(NewUserStatusCheck(a.log), ctx, string(user.Status)); err != nil {
		return nil, err
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
