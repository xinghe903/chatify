package bo

import "time"

// UserStatus 用户状态枚举
type UserStatus string

const (
	UserStatusActive  UserStatus = "active"  // 活跃
	UserStatusRevoked UserStatus = "revoked" // 已注销
	UserStatusLocked  UserStatus = "locked"  // 被锁定

	UserNameMaxLength = 20
	EmailMaxLength    = 50
)

// User 用户业务对象
type User struct {
	ID        string     `json:"id"`
	Username  string     `json:"username"`
	Email     string     `json:"email"`
	Password  string     `json:"password"` // 注意：实际应用中应存储哈希后的密码
	Phone     string     `json:"phone,omitempty"`
	Status    UserStatus `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

// NewUser 从请求创建新的用户业务对象
func NewUser(username, email, password, phone string) *User {
	now := time.Now()
	return &User{
		Username:  username,
		Email:     email,
		Password:  password,
		Phone:     phone,
		Status:    UserStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
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
	RefreshToken    string
	AccessExpiresIn int64
}

// VerifyTokenResult 验证令牌结果结构
type VerifyTokenResult struct {
	UserID    string
	Username  string
	Email     string
	ExpiresAt int64
}
