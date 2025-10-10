package biz

import (
	"github.com/google/wire"
)

// AuthService 认证服务接口
// type AuthService interface {
// 	Register(ctx context.Context, username, email, password, phone string) (string, error)
// 	Login(ctx context.Context, identifier, password string) (*LoginResult, error)
// 	Logout(ctx context.Context, userID string, refreshToken string) error
// 	RevokeUser(ctx context.Context, userID string, password string) error
// 	RefreshToken(ctx context.Context, refreshToken string) (*TokenResult, error)
// 	VerifyToken(ctx context.Context, accessToken string) (*VerifyTokenResult, error)
// }

// ProviderSet is biz providers.
var ProviderSet = wire.NewSet(NewAuth)
