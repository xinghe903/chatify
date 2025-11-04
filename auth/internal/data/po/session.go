package po

import (
	"strings"

	"github.com/xinghe903/chatify/pkg/model"

	"gorm.io/gorm"
)

// Session 登录会话表
type Session struct {
	model.BaseModel
	UserID           string `gorm:"type:varchar(36);index:idx_user_id;not null" json:"user_id"`
	Username         string `gorm:"type:varchar(255);not null" json:"username"`
	AccessToken      string `gorm:"type:varchar(255);not null;uniqueIndex:idx_access_token" json:"access_token"`
	RefreshToken     string `gorm:"type:varchar(255);not null;uniqueIndex:idx_refresh_token" json:"refresh_token"`
	AccessExpiresIn  int64  `gorm:"not null" json:"access_expires_in"`  // 秒级时间戳
	RefreshExpiresIn int64  `gorm:"not null" json:"refresh_expires_in"` // 秒级时间戳
}

// TableName 指定表名
func (Session) TableName() string {
	return "chatify_auth_session"
}

func (s *Session) BeforeCreate(tx *gorm.DB) error {
	if !strings.HasPrefix(s.ID, "sid") {
		s.ID = "sid" + s.ID
	}
	return nil
}
