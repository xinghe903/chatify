package po

import (
	"pkg/model"
	"strings"
	"time"

	"gorm.io/gorm"
)

// UserStatus 用户状态枚举
type UserStatus string

const (
	UserStatusActive  UserStatus = "active"  // 活跃
	UserStatusRevoked UserStatus = "revoked" // 已注销
	UserStatusLocked  UserStatus = "locked"  // 被锁定（可扩展）
)

// User 用户表
type User struct {
	model.BaseModel
	Username string  `gorm:"type:varchar(50);uniqueIndex:idx_username;not null" json:"username"`
	Email    string  `gorm:"type:varchar(100);uniqueIndex:idx_email;not null" json:"email"`
	Phone    *string `gorm:"type:varchar(20)" json:"phone,omitempty"`
	Password string  `gorm:"type:varchar(255);not null" json:"-"` // 不返回给前端

	// 👇 新增状态字段
	Status UserStatus `gorm:"type:varchar(20);default:'active';not null;index:idx_status" json:"status"`

	// 👇 可选：记录注销时间（与 DeletedAt 区分）
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

// TableName 指定表名
func (User) TableName() string {
	return "chatify_auth_user"
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if !strings.HasPrefix(u.ID, "uid") {
		u.ID = "uid" + u.ID
	}
	return nil
}
