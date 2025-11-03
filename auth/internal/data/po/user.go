package po

import (
	"auth/internal/biz/bo"
	"pkg/model"
	"strings"
	"time"

	"gorm.io/gorm"
)

// User 用户表
type User struct {
	model.BaseModel
	Username string  `gorm:"type:varchar(50);uniqueIndex:idx_username;not null" json:"username"`
	Email    string  `gorm:"type:varchar(100);uniqueIndex:idx_email;not null" json:"email"`
	Phone    *string `gorm:"type:varchar(20)" json:"phone,omitempty"`
	Password string  `gorm:"type:varchar(255);not null" json:"-"` // 不返回给前端

	// 👇 新增状态字段
	Status string `gorm:"type:varchar(20);default:'active';not null;index:idx_status" json:"status"`

	// 👇 可选：记录注销时间（与 DeletedAt 区分）
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

func NewUserWithBo(user *bo.User) *User {
	return &User{
		BaseModel: model.BaseModel{
			ID:        user.ID,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
		},
		Username:  user.Username,
		Email:     user.Email,
		Phone:     &user.Phone,
		Password:  user.Password,
		Status:    string(user.Status),
		RevokedAt: user.RevokedAt,
	}
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

func (u *User) ToBo() *bo.User {
	ret := &bo.User{
		ID:        u.ID,
		Username:  u.Username,
		Email:     u.Email,
		Password:  u.Password,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
		RevokedAt: u.RevokedAt,
	}
	if u.Phone != nil {
		ret.Phone = *u.Phone
	}
	ret.Status = bo.UserStatus(u.Status)
	return ret
}
