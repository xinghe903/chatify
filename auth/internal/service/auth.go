package service

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/xinghe903/chatify/pkg/auth"

	"auth/internal/biz"
	"auth/internal/biz/bo"

	v1 "github.com/xinghe903/chatify/apiauth/v1"

	kerrors "github.com/go-kratos/kratos/v2/errors"
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
	// 验证请求参数
	if in.Username == "" {
		return nil, v1.ErrorParamInvalid("username cannot be empty")
	}
	if in.Email == "" {
		return nil, v1.ErrorParamInvalid("email cannot be empty")
	}
	if in.Password == "" {
		return nil, v1.ErrorParamInvalid("password cannot be empty")
	}

	// 调用业务逻辑层进行用户注册
	userID, err := s.uc.Register(ctx, &bo.User{
		Username: in.Username,
		Email:    in.Email,
		Password: in.Password,
		Phone:    in.Phone,
	})
	if err != nil {
		s.log.WithContext(ctx).Errorf("Failed to register user: %v", err)
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
		return nil, v1.ErrorParamInvalid("username, email or phone is required")
	}

	if in.Password == "" {
		return nil, v1.ErrorParamInvalid("password cannot be empty")
	}
	// 调用业务逻辑层进行用户登录
	result, err := s.uc.Login(ctx, identifier, in.Password)
	if err != nil {
		s.log.WithContext(ctx).Errorf("Failed to login user: %v", err)
		return nil, err
	}

	// 返回登录结果，包含token信息
	return &v1.LoginResponse{
			UserId:           result.UserID,
			Username:         result.Username,
			AccessToken:      result.AccessToken,
			RefreshToken:     result.RefreshToken,
			AccessExpiresIn:  strconv.FormatInt(result.AccessExpiresIn, 10),
			RefreshExpiresIn: strconv.FormatInt(result.RefreshExpiresIn, 10),
		},
		nil
}

// VerifyToken 验证访问令牌
func (s *AuthService) VerifyToken(ctx context.Context, in *v1.VerifyTokenRequest) (*v1.VerifyTokenResponse, error) {
	// 将用户信息写入http响应头里面
	tp, ok := transport.FromServerContext(ctx)
	if !ok {
		s.log.WithContext(ctx).Errorf("Failed to get transport context failed")
		return nil, v1.ErrorTokenInvalid("transport not found")
	}
	in.AccessToken = tp.RequestHeader().Get(string(auth.ACCESS_TOKEN))
	setHeader := func(err *kerrors.Error) error {
		errJson, _ := json.Marshal(err)
		tp.ReplyHeader().Set(string(auth.AUTH_ERROR), string(errJson))
		return err
	}
	if len(in.AccessToken) == 0 {
		return nil, setHeader(v1.ErrorTokenInvalid("%s is required", string(auth.ACCESS_TOKEN)))
	}

	// 调用业务逻辑层验证令牌
	result, err := s.uc.VerifyToken(ctx, in.AccessToken)
	if err != nil {
		e, ok := err.(*kerrors.Error)
		if !ok {
			e = v1.ErrorTokenInvalid("invalid access token")
		}
		return nil, setHeader(e)
	}
	tp.ReplyHeader().Set(string(auth.USER_ID), result.UserID)
	tp.ReplyHeader().Set(string(auth.USER_NAME), result.Username)
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
		return nil, v1.ErrorParamInvalid("user_id cannot be empty")
	}
	if in.RefreshToken == "" {
		return nil, v1.ErrorParamInvalid("refresh_token cannot be empty")
	}
	// 调用业务逻辑层进行用户登出
	if err := s.uc.Logout(ctx, in.UserId, in.RefreshToken); err != nil {
		s.log.WithContext(ctx).Errorf("Failed to logout err: %s", err)
		return nil, err
	}
	// 返回成功响应
	return &v1.LogoutResponse{}, nil
}

// RevokeUser 注销用户
func (s *AuthService) RevokeUser(ctx context.Context, in *v1.RevokeUserRequest) (*v1.RevokeUserResponse, error) {
	// 验证请求参数
	if in.UserId == "" {
		return nil, v1.ErrorParamInvalid("user_id cannot be empty")
	}
	if in.Password == "" {
		return nil, v1.ErrorParamInvalid("password cannot be empty")
	}
	// 调用业务逻辑层进行用户注销
	if err := s.uc.RevokeUser(ctx, in.UserId, in.Password); err != nil {
		s.log.WithContext(ctx).Errorf("Failed to revoke user: %v", err)
		return nil, err
	}
	// 返回成功响应
	return &v1.RevokeUserResponse{}, nil
}

// RefreshToken 刷新访问令牌
func (s *AuthService) RefreshToken(ctx context.Context, in *v1.RefreshTokenRequest) (*v1.RefreshTokenResponse, error) {
	// 验证请求参数
	if in.RefreshToken == "" {
		return nil, v1.ErrorParamInvalid("refresh_token cannot be empty")
	}

	// 调用业务逻辑层进行令牌刷新
	result, err := s.uc.RefreshToken(ctx, in.RefreshToken)
	if err != nil {
		s.log.WithContext(ctx).Errorf("Failed to refresh token: %v", err)
		return nil, err
	}

	// 返回新的访问令牌
	return &v1.RefreshTokenResponse{
			AccessToken:      result.AccessToken,
			AccessExpiresIn:  strconv.FormatInt(result.AccessExpiresIn, 10),
			NewRefreshToken:  result.RefreshToken,
			RefreshExpiresIn: strconv.FormatInt(result.RefreshExpiresIn, 10),
		},
		nil
}
