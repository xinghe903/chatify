package data

import (
	"context"
	"fmt"
	"time"

	"github.com/xinghe903/chatify/pkg/auth"
	"github.com/xinghe903/chatify/pkg/model"

	v1 "github.com/xinghe903/chatify/apiauth/v1"

	"auth/internal/biz/bo"
	"auth/internal/data/po"

	"auth/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/gorm"
)

var _ biz.UserRepo = &userRepo{}

// 实现了biz.UserRepo接口
type userRepo struct {
	data      *Data
	log       *log.Helper
	sonyFlake *auth.Sonyflake
}

// NewUserRepo 创建用户仓库
func NewUserRepo(data *Data, logger log.Logger) biz.UserRepo {
	return &userRepo{
		data:      data,
		log:       log.NewHelper(logger),
		sonyFlake: auth.NewSonyflake(),
	}
}

// Create 创建用户
func (r *userRepo) Create(ctx context.Context, user *bo.User) error {
	var err error
	// 生成唯一用户ID
	if user.ID == "" {
		if user.ID, err = r.sonyFlake.GenerateBase62(); err != nil {
			return v1.ErrorGenerateIdFailed("user ID")
		}
	}
	user.Status = bo.UserStatusActive
	now := time.Now()
	// 转换为数据库实体
	phone := user.Phone
	userPO := &po.User{
		BaseModel: model.BaseModel{
			ID:        user.ID,
			CreatedAt: now,
			UpdatedAt: now,
		},
		Username: user.Username,
		Email:    user.Email,
		Password: user.Password,
		Phone:    &phone,
		Status:   string(user.Status),
	}

	// 保存到数据库
	if err := r.data.db.WithContext(ctx).Create(userPO).Error; err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	user.ID = userPO.ID
	r.log.WithContext(ctx).Infof("Created user: %s", user.Username)
	return nil
}

// GetByID 根据ID获取用户
func (r *userRepo) GetByID(ctx context.Context, id string) (*bo.User, error) {
	var userPO po.User
	if err := r.data.db.WithContext(ctx).Where("id = ?", id).First(&userPO).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by ID: %w", err)
	}
	return userPO.ToBo(), nil
}

// GetByUsername 根据用户名获取用户
func (r *userRepo) GetByUsername(ctx context.Context, username string) (*bo.User, error) {
	var userPO po.User
	if err := r.data.db.WithContext(ctx).Where("username = ?", username).First(&userPO).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by username: %w", err)
	}
	return userPO.ToBo(), nil
}

// GetByEmail 根据邮箱获取用户
func (r *userRepo) GetByEmail(ctx context.Context, email string) (*bo.User, error) {
	var userPO po.User
	if err := r.data.db.WithContext(ctx).Where("email = ?", email).First(&userPO).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	return userPO.ToBo(), nil
}

// GetByPhone 根据手机号获取用户
func (r *userRepo) GetByPhone(ctx context.Context, phone string) (*bo.User, error) {
	var userPO po.User
	if err := r.data.db.WithContext(ctx).Where("phone = ?", phone).First(&userPO).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by phone: %w", err)
	}
	return userPO.ToBo(), nil
}

// Update 更新用户信息
func (r *userRepo) Update(ctx context.Context, user *bo.User) error {
	// 获取现有用户信息
	existingUser, err := r.GetByID(ctx, user.ID)
	if err != nil {
		return err
	}
	// 更新用户信息
	existingUser.Username = user.Username
	existingUser.Email = user.Email
	existingUser.Phone = user.Phone
	existingUser.Status = user.Status
	existingUser.UpdatedAt = time.Now()
	// 更新数据库
	if err := r.data.db.WithContext(ctx).Save(po.NewUserWithBo(existingUser)).Error; err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	r.log.WithContext(ctx).Infof("Updated user: %s", existingUser.ID)
	return nil
}

// Delete 删除用户
func (r *userRepo) Delete(ctx context.Context, id string) error {
	// 获取用户信息
	_, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}
	// 更新用户状态为注销并记录注销时间
	now := time.Now()
	if err := r.data.db.WithContext(ctx).Model(&po.User{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     bo.UserStatusRevoked,
		"revoked_at": now,
		"deleted_at": &now,
	}).Error; err != nil {
		return fmt.Errorf("failed to update user status: %w", err)
	}
	return nil
}
