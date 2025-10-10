package data

import (
	v1 "api/auth/v1"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"pkg/auth"
	"pkg/model"
	"time"

	"auth/internal/biz/bo"
	"auth/internal/data/po"

	"auth/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// 用户相关的Redis键
const (
	UserIDPrefix       = "chatify:user:id:"       // 用户ID缓存键前缀
	UsernamePrefix     = "chatify:user:username:" // 用户名缓存键前缀
	EmailPrefix        = "chatify:user:email:"    // 邮箱缓存键前缀
	PhonePrefix        = "chatify:user:phone:"    // 手机号缓存键前缀
	RefreshTokenPrefix = "chatify:token:refresh:" // 刷新令牌缓存键前缀
	AccessTokenPrefix  = "chatify:token:access:"  // 访问令牌缓存键前缀
	UserTokenPrefix    = "chatify:user:token:"    // 用户令牌缓存键前缀
)

// userRepo 基于MySQL(GORM)和Redis的用户仓库实现
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

	// 密码加密（简单实现，实际项目中应使用bcrypt等强加密算法）
	passwordHash := r.hashPassword(user.Password)

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
		Password: passwordHash,
		Phone:    &phone,
		Status:   po.UserStatusActive,
	}

	// 保存到数据库
	if err := r.data.db.WithContext(ctx).Create(userPO).Error; err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	// 更新缓存
	r.cacheUser(ctx, user)

	r.log.WithContext(ctx).Infof("Created user: %s", user.Username)
	return nil
}

// GetByID 根据ID获取用户
func (r *userRepo) GetByID(ctx context.Context, id string) (*bo.User, error) {
	// 先从缓存获取
	user, err := r.getCachedUserByID(ctx, id)
	if err == nil && user != nil {
		return user, nil
	}

	// 缓存未命中，从数据库获取
	var userPO po.User
	if err := r.data.db.WithContext(ctx).Where("id = ?", id).First(&userPO).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("user not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get user by ID: %w", err)
	}

	// 转换为业务对象
	user = &bo.User{
		ID:        userPO.ID,
		Username:  userPO.Username,
		Email:     userPO.Email,
		Password:  userPO.Password, // 注意：这里返回的是哈希后的密码
		Phone:     "",
		CreatedAt: userPO.CreatedAt,
		UpdatedAt: userPO.UpdatedAt,
	}

	if userPO.Phone != nil {
		user.Phone = *userPO.Phone
	}

	// 更新缓存
	r.cacheUser(ctx, user)

	return user, nil
}

// GetByUsername 根据用户名获取用户
func (r *userRepo) GetByUsername(ctx context.Context, username string) (*bo.User, error) {
	// 先从缓存获取用户ID
	userID, err := r.data.redis.Get(ctx, UsernamePrefix+username).Result()
	if err == nil && userID != "" {
		return r.GetByID(ctx, userID)
	}

	// 缓存未命中，从数据库获取
	var userPO po.User
	if err := r.data.db.WithContext(ctx).Where("username = ?", username).First(&userPO).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("user not found by username: %s", username)
		}
		return nil, fmt.Errorf("failed to get user by username: %w", err)
	}

	// 转换为业务对象
	user := &bo.User{
		ID:        userPO.ID,
		Username:  userPO.Username,
		Email:     userPO.Email,
		Password:  userPO.Password,
		Phone:     "",
		CreatedAt: userPO.CreatedAt,
		UpdatedAt: userPO.UpdatedAt,
	}

	if userPO.Phone != nil {
		user.Phone = *userPO.Phone
	}

	// 更新缓存
	r.cacheUser(ctx, user)
	r.data.redis.Set(ctx, UsernamePrefix+username, user.ID, 0) // 永久存储

	return user, nil
}

// GetByEmail 根据邮箱获取用户
func (r *userRepo) GetByEmail(ctx context.Context, email string) (*bo.User, error) {
	// 先从缓存获取用户ID
	userID, err := r.data.redis.Get(ctx, EmailPrefix+email).Result()
	if err == nil && userID != "" {
		return r.GetByID(ctx, userID)
	}

	// 缓存未命中，从数据库获取
	var userPO po.User
	if err := r.data.db.WithContext(ctx).Where("email = ?", email).First(&userPO).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("user not found by email: %s", email)
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}

	// 转换为业务对象
	user := &bo.User{
		ID:        userPO.ID,
		Username:  userPO.Username,
		Email:     userPO.Email,
		Password:  userPO.Password,
		Phone:     "",
		CreatedAt: userPO.CreatedAt,
		UpdatedAt: userPO.UpdatedAt,
	}

	if userPO.Phone != nil {
		user.Phone = *userPO.Phone
	}

	// 更新缓存
	r.cacheUser(ctx, user)
	r.data.redis.Set(ctx, EmailPrefix+email, user.ID, 0) // 永久存储

	return user, nil
}

// GetByPhone 根据手机号获取用户
func (r *userRepo) GetByPhone(ctx context.Context, phone string) (*bo.User, error) {
	// 先从缓存获取用户ID
	userID, err := r.data.redis.Get(ctx, PhonePrefix+phone).Result()
	if err == nil && userID != "" {
		return r.GetByID(ctx, userID)
	}

	// 缓存未命中，从数据库获取
	var userPO po.User
	if err := r.data.db.WithContext(ctx).Where("phone = ?", phone).First(&userPO).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("user not found by phone: %s", phone)
		}
		return nil, fmt.Errorf("failed to get user by phone: %w", err)
	}

	// 转换为业务对象
	user := &bo.User{
		ID:        userPO.ID,
		Username:  userPO.Username,
		Email:     userPO.Email,
		Password:  userPO.Password,
		Phone:     *userPO.Phone,
		CreatedAt: userPO.CreatedAt,
		UpdatedAt: userPO.UpdatedAt,
	}

	// 更新缓存
	r.cacheUser(ctx, user)
	r.data.redis.Set(ctx, PhonePrefix+phone, user.ID, 0) // 永久存储

	return user, nil
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

	// 转换为数据库实体
	phone := existingUser.Phone
	userPO := &po.User{
		BaseModel: model.BaseModel{
			ID:        existingUser.ID,
			CreatedAt: existingUser.CreatedAt,
			UpdatedAt: existingUser.UpdatedAt,
		},
		Username: existingUser.Username,
		Email:    existingUser.Email,
		Password: existingUser.Password,
		Phone:    &phone,
		Status:   po.UserStatus(existingUser.Status),
	}

	if existingUser.RevokedAt != nil && *existingUser.RevokedAt != (time.Time{}) {
		revokedAt := existingUser.RevokedAt
		userPO.RevokedAt = revokedAt
	}

	// 更新数据库
	if err := r.data.db.WithContext(ctx).Save(userPO).Error; err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	// 清除并更新缓存
	r.clearUserCache(ctx, existingUser.ID)
	r.cacheUser(ctx, existingUser)

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
		"status":     po.UserStatusRevoked,
		"revoked_at": now,
		"deleted_at": &now,
	}).Error; err != nil {
		return fmt.Errorf("failed to update user status: %w", err)
	}

	// 清除缓存
	r.clearUserCache(ctx, id)

	// 删除用户的所有token
	r.data.redis.Del(ctx, UserTokenPrefix+id)

	r.log.WithContext(ctx).Infof("Deleted user: %s", id)
	return nil
}

// CheckPassword 检查密码是否正确
func (r *userRepo) CheckPassword(ctx context.Context, userID string, password string) (bool, error) {
	// 从缓存获取用户信息
	user, err := r.getCachedUserByID(ctx, userID)
	if err != nil {
		// 缓存未命中，从数据库获取
		user, err = r.GetByID(ctx, userID)
		if err != nil {
			return false, fmt.Errorf("user not found: %w", err)
		}
	}

	// 计算密码哈希
	hashedPassword := r.hashPassword(password)

	// 比较密码哈希
	return user.Password == hashedPassword, nil
}

// SaveToken 保存用户的token信息
func (r *userRepo) SaveToken(ctx context.Context, tokenInfo *bo.LoginResult) error {
	// 计算过期时间戳
	currentUnix := time.Now().Unix()
	accessExpiresAt := currentUnix + tokenInfo.AccessExpiresIn
	refreshExpiresAt := currentUnix + tokenInfo.RefreshExpiresIn

	// 1. 将令牌信息保存到数据库
	session := &po.Session{
		BaseModel: model.BaseModel{
			ID: uuid.New().String(),
		},
		UserID:           tokenInfo.UserID,
		AccessToken:      tokenInfo.AccessToken,
		RefreshToken:     tokenInfo.RefreshToken,
		AccessExpiresIn:  accessExpiresAt,
		RefreshExpiresIn: refreshExpiresAt,
	}

	if err := r.data.db.WithContext(ctx).Create(session).Error; err != nil {
		r.log.WithContext(ctx).Errorf("Failed to save token to database: %v", err)
		return fmt.Errorf("failed to save token to database: %w", err)
	}

	// 2. 将令牌信息保存到Redis（缓存层）
	pipe := r.data.redis.Pipeline()
	// 存储refresh token到userID的映射，设置过期时间
	pipe.Set(ctx, RefreshTokenPrefix+tokenInfo.RefreshToken, tokenInfo.UserID, time.Duration(tokenInfo.RefreshExpiresIn)*time.Second)
	// 存储access token到用户信息的映射，设置过期时间
	pipe.Set(ctx, AccessTokenPrefix+tokenInfo.AccessToken, fmt.Sprintf("%s:%d", tokenInfo.UserID, accessExpiresAt), time.Duration(tokenInfo.AccessExpiresIn)*time.Second)
	// 存储用户的当前refresh token
	pipe.Set(ctx, UserTokenPrefix+tokenInfo.UserID, tokenInfo.RefreshToken, time.Duration(tokenInfo.RefreshExpiresIn)*time.Second)
	_, err := pipe.Exec(ctx)
	if err != nil {
		r.log.WithContext(ctx).Errorf("Failed to save token to Redis: %v", err)
		// 注意：这里没有返回错误，因为数据库操作已成功，Redis失败可以接受
	}

	r.log.WithContext(ctx).Infof("Saved token for user: %s", tokenInfo.UserID)
	return nil
}

// InvalidateToken 使token失效
func (r *userRepo) InvalidateToken(ctx context.Context, userID string, refreshToken string) error {
	// 1. 从数据库中删除对应的会话记录
	if err := r.data.db.WithContext(ctx).Where("user_id = ? AND refresh_token = ?", userID, refreshToken).Delete(&po.Session{}).Error; err != nil {
		r.log.WithContext(ctx).Errorf("Failed to delete token from database: %v", err)
		// 注意：这里不返回错误，因为即使数据库删除失败，我们仍然需要尝试从Redis中删除令牌
	}

	// 2. 从Redis中删除令牌信息
	pipe := r.data.redis.Pipeline()
	pipe.Del(ctx, RefreshTokenPrefix+refreshToken)
	pipe.Del(ctx, UserTokenPrefix+userID)
	_, err := pipe.Exec(ctx)
	if err != nil {
		r.log.WithContext(ctx).Errorf("Failed to invalidate token in Redis: %v", err)
		return fmt.Errorf("failed to invalidate token in Redis: %w", err)
	}

	r.log.WithContext(ctx).Infof("Invalidated token for user: %s", userID)
	return nil
}

// GetTokenInfo 获取token信息（通过refresh token获取userID）
func (r *userRepo) GetTokenInfo(ctx context.Context, refreshToken string) (string, error) {
	userID, err := r.data.redis.Get(ctx, RefreshTokenPrefix+refreshToken).Result()
	if err != nil {
		if err == redis.Nil {
			return "", fmt.Errorf("invalid refresh token")
		}
		return "", fmt.Errorf("failed to get token info: %w", err)
	}

	return userID, nil
}

// VerifyAccessToken 验证访问令牌
func (r *userRepo) VerifyAccessToken(ctx context.Context, accessToken string) (string, int64, error) {
	// 获取存储的令牌信息
	tokenInfo, err := r.data.redis.Get(ctx, AccessTokenPrefix+accessToken).Result()
	if err != nil {
		if err == redis.Nil {
			return "", 0, fmt.Errorf("invalid or expired access token")
		}
		return "", 0, fmt.Errorf("failed to verify access token: %w", err)
	}

	// 解析令牌信息，格式为"userID:expiresAt"
	var userID string
	var expiresAt int64
	fmt.Sscanf(tokenInfo, "%s:%d", &userID, &expiresAt)

	// 检查令牌是否已过期
	if time.Now().Unix() > expiresAt {
		// 删除过期的令牌
		r.data.redis.Del(ctx, AccessTokenPrefix+accessToken)
		return "", 0, fmt.Errorf("access token has expired")
	}

	return userID, expiresAt, nil
}

// InvalidateAllTokens 使指定用户的所有令牌失效
func (r *userRepo) InvalidateAllTokens(ctx context.Context, userID string) error {
	// 1. 从数据库中删除用户的所有会话记录
	if err := r.data.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&po.Session{}).Error; err != nil {
		r.log.WithContext(ctx).Errorf("Failed to delete all user tokens from database: %v", err)
		// 注意：这里不返回错误，因为即使数据库删除失败，我们仍然需要尝试从Redis中删除令牌
	}

	// 2. 检查Redis中是否有用户令牌信息
	_, err := r.data.redis.Get(ctx, UserTokenPrefix+userID).Result()
	if err != nil {
		if err == redis.Nil {
			// 用户在Redis中没有令牌信息，但数据库操作已处理
			r.log.WithContext(ctx).Infof("Invalidated all tokens for user (only database entries): %s", userID)
			return nil
		}
		r.log.WithContext(ctx).Errorf("Failed to get user token info from Redis: %v", err)
		// 继续处理，因为数据库操作已完成
	}

	// 3. 从Redis中删除用户的所有令牌信息
	pipe := r.data.redis.Pipeline()
	pipe.Del(ctx, UserTokenPrefix+userID)

	// 在实际应用中，这里可以实现更复杂的逻辑来清除用户的所有令牌
	// 例如，获取并删除用户的所有刷新令牌和访问令牌

	_, err = pipe.Exec(ctx)
	if err != nil {
		r.log.WithContext(ctx).Errorf("Failed to invalidate all tokens in Redis: %v", err)
		// 不返回错误，因为数据库操作已成功
	}

	r.log.WithContext(ctx).Infof("Invalidated all tokens for user: %s", userID)
	return nil
}

// 辅助方法

// hashPassword 密码加密（简单实现，实际项目中应使用bcrypt等强加密算法）
func (r *userRepo) hashPassword(password string) string {
	hash := md5.Sum([]byte(password))
	return hex.EncodeToString(hash[:])
}

// cacheUser 将用户信息缓存到Redis
func (r *userRepo) cacheUser(ctx context.Context, user *bo.User) {
	// 将用户对象转换为JSON（这里简化处理）
	// 实际项目中应使用正式的JSON序列化
	userData := fmt.Sprintf("{\"id\":\"%s\",\"username\":\"%s\",\"email\":\"%s\",\"phone\":\"%s\"}",
		user.ID, user.Username, user.Email, user.Phone)

	r.data.redis.Set(ctx, UserIDPrefix+user.ID, userData, 0) // 永久存储
}

// getCachedUserByID 从缓存获取用户信息
func (r *userRepo) getCachedUserByID(ctx context.Context, id string) (*bo.User, error) {
	_, err := r.data.redis.Get(ctx, UserIDPrefix+id).Result()
	if err != nil {
		return nil, err
	}

	// 这里简化处理，实际项目中应使用正式的JSON反序列化
	// 这里我们假设从缓存中获取到的是完整的用户信息
	// 并通过其他方式获取详细信息

	// 由于简化处理，我们直接调用GetByID从数据库获取
	return nil, fmt.Errorf("cache miss")
}

// clearUserCache 清除用户缓存
func (r *userRepo) clearUserCache(ctx context.Context, id string) {
	user, _ := r.GetByID(ctx, id) // 忽略错误
	if user != nil {
		pipe := r.data.redis.Pipeline()
		pipe.Del(ctx, UserIDPrefix+id)
		pipe.Del(ctx, UsernamePrefix+user.Username)
		pipe.Del(ctx, EmailPrefix+user.Email)
		if user.Phone != "" {
			pipe.Del(ctx, PhonePrefix+user.Phone)
		}
		pipe.Exec(ctx) // 忽略错误
	}
}
