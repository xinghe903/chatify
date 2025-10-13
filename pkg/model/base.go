package model

import "time"

// BaseModel 自定义基础模型，继承 gorm.Model 但将 ID 类型改为 string
type BaseModel struct {
	ID        string     `gorm:"type:varchar(36);primaryKey" json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}
