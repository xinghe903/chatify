package bo

import "time"

// User 用户业务对象
type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Password  string    `json:"password"` // 注意：实际应用中应存储哈希后的密码
	Phone     string    `json:"phone,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewUser 从请求创建新的用户业务对象
func NewUser(username, email, password, phone string) *User {
	now := time.Now()
	return &User{
		Username:  username,
		Email:     email,
		Password:  password,
		Phone:     phone,
		CreatedAt: now,
		UpdatedAt: now,
	}
}