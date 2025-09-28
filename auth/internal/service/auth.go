package service

import (
	"context"
	"errors"
	"strconv"

	v1 "api/auth/v1"
	"auth/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"
)

// AuthService 认证服务实现
type AuthService struct {
	v1.UnimplementedAuthServiceServer
	log *log.Helper
	uc  *biz.Auth
}

// NewAuthService 创建认证服务
func NewAuthService(logger log.Logger, uc *biz.Auth) *AuthService {
	return &AuthService{
		log: log.NewHelper(log.With(logger)),
		uc:  uc,
	}
}

// Register 注册用户
func (s *AuthService) Register(ctx context.Context, in *v1.RegisterRequest) (*v1.RegisterResponse, error) {
	s.log.WithContext(ctx).Debugf("Register: %v", in)
	return &v1.RegisterResponse{
		UserId: "001",
	}, nil
	// 验证请求参数
	if in.Username == "" {
		// return nil, v1.ErrorInvalidArgument("username cannot be empty")
		return nil, errors.New("username cannot be empty")
	}
	if in.Email == "" {
		// return nil, v1.ErrorInvalidArgument("email cannot be empty")
		return nil, errors.New("email cannot be empty")
	}
	if in.Password == "" {
		// return nil, v1.ErrorInvalidArgument("password cannot be empty")
		return nil, errors.New("password cannot be empty")
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
	s.log.WithContext(ctx).Debugf("Login: %v", in)
	tr, ok := transport.FromServerContext(ctx)
	if !ok {
		s.log.WithContext(ctx).Error("Failed to get transport from context")
		return nil, errors.New("Failed to get transport from context")
	}
	uid := tr.RequestHeader().Get("X-User-ID")
	s.log.WithContext(ctx).Debugf("X-User-ID: %s", uid)
	return &v1.LoginResponse{
		UserId:       "001",
		AccessToken:  "access_token",
		RefreshToken: "refresh_token",
	}, nil
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
		// return nil, v1.ErrorInvalidArgument("username, email or phone is required")
		return nil, errors.New("username, email or phone is required")
	}

	if in.Password == "" {
		// return nil, v1.ErrorInvalidArgument("password cannot be empty")
		return nil, errors.New("password cannot be empty")
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
			AccessExpiresIn:  strconv.FormatInt(result.AccessExpiresIn, 10),
			RefreshExpiresIn: strconv.FormatInt(result.RefreshExpiresIn, 10),
		},
		nil
}

// VerifyToken 验证访问令牌
func (s *AuthService) VerifyToken(ctx context.Context, in *v1.VerifyTokenRequest) (*v1.VerifyTokenResponse, error) {
	s.log.WithContext(ctx).Debugf("VerifyToken: %v", in)
	tr, ok := transport.FromServerContext(ctx)
	if !ok {
		s.log.WithContext(ctx).Error("Failed to get transport from context")
		return nil, errors.New("Failed to get transport from context")
	}
	auth := tr.RequestHeader().Get("Authorizatio11111")
	if auth == "" {
		s.log.WithContext(ctx).Error("Authorization header is missing")
		return nil, errors.New("Authorization header is missing")
	}
	s.log.WithContext(ctx).Debugf("Authorization: %s", auth)
	tr.ReplyHeader().Set("X-User-ID", "response001")
	tr.ReplyHeader().Set("X-Is-VIP", "false")
	return &v1.VerifyTokenResponse{}, nil

	// 调用业务逻辑层验证令牌
	result, err := s.uc.VerifyToken(ctx, in.AccessToken)
	if err != nil {
		s.log.Errorf("Failed to verify token: %v", err)
		// 如果验证失败，返回错误
		return nil, err
	}

	// 返回验证成功的结果，将expires_at转换为字符串
	return &v1.VerifyTokenResponse{
			Username:  result.Username,
			UserId:    result.UserID,
			ExpiresAt: strconv.FormatInt(result.ExpiresAt, 10),
		},
		nil
}

// Logout 用户登出
func (s *AuthService) Logout(ctx context.Context, in *v1.LogoutRequest) (*v1.LogoutResponse, error) {
	// 验证请求参数
	if in.UserId == "" {
		// return nil, v1.ErrorInvalidArgument("user_id cannot be empty")
		return nil, errors.New("user_id cannot be empty")
	}
	if in.RefreshToken == "" {
		// return nil, v1.ErrorInvalidArgument("refresh_token cannot be empty")
		return nil, errors.New("refresh_token cannot be empty")
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
		// return nil, v1.ErrorInvalidArgument("user_id cannot be empty")
		return nil, errors.New("user_id cannot be empty")
	}
	if in.Password == "" {
		// return nil, v1.ErrorInvalidArgument("password cannot be empty")
		return nil, errors.New("password cannot be empty")
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
		// return nil, v1.ErrorInvalidArgument("refresh_token cannot be empty")
		return nil, errors.New("refresh_token cannot be empty")
	}

	// 调用业务逻辑层进行令牌刷新
	result, err := s.uc.RefreshToken(ctx, in.RefreshToken)
	if err != nil {
		return nil, err
	}

	// 返回新的访问令牌
	return &v1.RefreshTokenResponse{
			AccessToken:     result.AccessToken,
			AccessExpiresIn: strconv.FormatInt(result.AccessExpiresIn, 10),
		},
		nil
}
