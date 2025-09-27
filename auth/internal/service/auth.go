package service

import (
	"context"

	v1 "auth/api/auth/v1"
	"auth/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

// AuthService 认证服务实现
type AuthService struct {
	v1.UnimplementedAuthServiceServer

	uc *biz.Auth
}

// NewAuthService 创建认证服务
func NewAuthService(uc *biz.Auth) *AuthService {
	return &AuthService{uc: uc}
}

// Register 注册用户
func (s *AuthService) Register(ctx context.Context, in *v1.RegisterRequest) (*v1.RegisterResponse, error) {
	// 验证请求参数
	if in.Username == "" {
		return nil, v1.ErrorInvalidArgument("username cannot be empty")
	}
	if in.Email == "" {
		return nil, v1.ErrorInvalidArgument("email cannot be empty")
	}
	if in.Password == "" {
		return nil, v1.ErrorInvalidArgument("password cannot be empty")
	}

	// 调用业务逻辑层进行用户注册
	userID, err := s.uc.Register(ctx, in.Username, in.Email, in.Password, in.Phone)
	if err != nil {
		return nil, err
	}

	// 返回注册成功的用户ID
	return &v1.RegisterResponse{
		UserId: userID,
	},
		nil
}

// Login 用户登录
func (s *AuthService) Login(ctx context.Context, in *v1.LoginRequest) (*v1.LoginResponse, error) {
	// 验证请求参数
	var identifier string
	switch {
	case in.Username != "":
		identifier = in.Username
	case in.Email != "":
		identifier = in.Email
	case in.Phone != "":
		identifier = in.Phone
	default:
		return nil, v1.ErrorInvalidArgument("username, email or phone is required")
	}

	if in.Password == "" {
		return nil, v1.ErrorInvalidArgument("password cannot be empty")
	}

	// 调用业务逻辑层进行用户登录
	result, err := s.uc.Login(ctx, identifier, in.Password)
	if err != nil {
		return nil, err
	}

	// 返回登录结果，包含token信息
	return &v1.LoginResponse{
		UserId:           result.UserID,
		AccessToken:      result.AccessToken,
		RefreshToken:     result.RefreshToken,
		AccessExpiresIn:  result.AccessExpiresIn,
		RefreshExpiresIn: result.RefreshExpiresIn,
	},
		nil
}

// Logout 用户登出
func (s *AuthService) Logout(ctx context.Context, in *v1.LogoutRequest) (*v1.LogoutResponse, error) {
	// 验证请求参数
	if in.UserId == "" {
		return nil, v1.ErrorInvalidArgument("user_id cannot be empty")
	}
	if in.RefreshToken == "" {
		return nil, v1.ErrorInvalidArgument("refresh_token cannot be empty")
	}

	// 调用业务逻辑层进行用户登出
	if err := s.uc.Logout(ctx, in.UserId, in.RefreshToken); err != nil {
		return nil, err
	}

	// 返回成功响应
	return &v1.LogoutResponse{}, nil
}

// RevokeUser 注销用户（永久删除账户）
func (s *AuthService) RevokeUser(ctx context.Context, in *v1.RevokeUserRequest) (*v1.RevokeUserResponse, error) {
	// 验证请求参数
	if in.UserId == "" {
		return nil, v1.ErrorInvalidArgument("user_id cannot be empty")
	}
	if in.Password == "" {
		return nil, v1.ErrorInvalidArgument("password cannot be empty")
	}

	// 调用业务逻辑层进行用户注销
	if err := s.uc.RevokeUser(ctx, in.UserId, in.Password); err != nil {
		return nil, err
	}

	// 返回成功响应
	return &v1.RevokeUserResponse{}, nil
}

// RefreshToken 刷新访问令牌
func (s *AuthService) RefreshToken(ctx context.Context, in *v1.RefreshTokenRequest) (*v1.RefreshTokenResponse, error) {
	// 验证请求参数
	if in.RefreshToken == "" {
		return nil, v1.ErrorInvalidArgument("refresh_token cannot be empty")
	}

	// 调用业务逻辑层进行令牌刷新
	result, err := s.uc.RefreshToken(ctx, in.RefreshToken)
	if err != nil {
		return nil, err
	}

	// 返回新的访问令牌
	return &v1.RefreshTokenResponse{
		AccessToken:     result.AccessToken,
		AccessExpiresIn: result.AccessExpiresIn,
	},
		nil
}