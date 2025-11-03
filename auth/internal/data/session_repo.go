package data

import (
	v1 "api/auth/v1"
	"auth/internal/biz"
	"auth/internal/biz/bo"
	"auth/internal/data/po"
	"context"
	"encoding/json"
	"fmt"
	"pkg/auth"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 用户相关的Redis键
const (
	UserTokenPrefix = "chatify:user:token:" // 用户令牌缓存键前缀
)

var _ biz.SessionRepo = (*sessionRepo)(nil)

type sessionRepo struct {
	data      *Data
	log       *log.Helper
	sonyFlake *auth.Sonyflake
}

func NewSessionRepo(data *Data, logger log.Logger) biz.SessionRepo {
	return &sessionRepo{
		data:      data,
		log:       log.NewHelper(logger),
		sonyFlake: auth.NewSonyflake(),
	}
}

func (r *sessionRepo) Upsert(ctx context.Context, tokenInfo *bo.LoginResult) error {
	// 计算过期时间戳
	currentUnix := time.Now().Unix()
	accessExpiresAt := currentUnix + tokenInfo.AccessExpiresIn
	refreshExpiresAt := currentUnix + tokenInfo.RefreshExpiresIn
	// 1. 将令牌信息保存到数据库
	session := &po.Session{
		UserID:           tokenInfo.UserID,
		AccessToken:      tokenInfo.AccessToken,
		RefreshToken:     tokenInfo.RefreshToken,
		AccessExpiresIn:  accessExpiresAt,
		RefreshExpiresIn: refreshExpiresAt,
	}
	var err error
	if session.ID, err = r.sonyFlake.GenerateBase62(); err != nil {
		return v1.ErrorGenerateIdFailed("session ID")
	}
	// user_id 存在则更新， 不存在则插入
	if err := r.data.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}},
		DoUpdates: []clause.Assignment{clause.Assignment{Column: clause.Column{Name: "access_token"}, Value: session.AccessToken},
			clause.Assignment{Column: clause.Column{Name: "refresh_token"}, Value: session.RefreshToken},
			clause.Assignment{Column: clause.Column{Name: "access_expires_in"}, Value: session.AccessExpiresIn},
			clause.Assignment{Column: clause.Column{Name: "refresh_expires_in"}, Value: session.RefreshExpiresIn}},
	}).Create(session).Error; err != nil {
		return fmt.Errorf("failed to save token to database: %w", err)
	}
	sessionBytes, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}
	// 存储用户的当前refresh token
	resultCmd := r.data.redis.Set(ctx, UserTokenPrefix+tokenInfo.AccessToken, sessionBytes,
		time.Duration(tokenInfo.AccessExpiresIn)*time.Second)
	if resultCmd.Err() != nil {
		r.log.WithContext(ctx).Errorf("Failed to save token to Redis: %v", resultCmd.Err())
		// 注意：这里没有返回错误，因为数据库操作已成功，Redis失败可以接受
	}
	return nil
}

func (r *sessionRepo) Delete(ctx context.Context, uid string) error {
	var session po.Session
	if err := r.data.db.WithContext(ctx).Where("user_id = ?", uid).First(&session).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}
	if err := r.data.db.WithContext(ctx).Where("user_id = ?", uid).Delete(&session).Error; err != nil {
		return err
	}
	return r.data.redis.Del(ctx, UserTokenPrefix+session.AccessToken).Err()
}

// GetTokenInfo 获取token信息（通过refresh token获取userID）
func (r *sessionRepo) GetTokenInfo(ctx context.Context, refreshToken string) (string, error) {
	var session po.Session
	resultTx := r.data.db.WithContext(ctx).Where("refresh_token = ?", refreshToken).First(&session)
	if resultTx.Error != nil {
		return "", fmt.Errorf("failed to get token info: %w", resultTx.Error)
	}
	return session.UserID, nil
}

// VerifyAccessToken 验证访问令牌
// @param ctx 上下文
// @param accessToken 访问令牌
// @return string 用户ID
// @return int64 过期时间戳, 单位秒
// @return error 错误信息
func (r *sessionRepo) VerifyAccessToken(ctx context.Context, accessToken string) (string, int64, error) {
	// 获取存储的令牌信息
	sessionStr, err := r.data.redis.Get(ctx, UserTokenPrefix+accessToken).Result()
	if err != nil {
		if err == redis.Nil {
			return "", 0, v1.ErrorTokenInvalid("access token is invalid or expired")
		}
		return "", 0, fmt.Errorf("failed to verify access token: %w", err)
	}
	var session po.Session
	if err := json.Unmarshal([]byte(sessionStr), &session); err != nil {
		return "", 0, fmt.Errorf("failed to unmarshal session: %w", err)
	}
	return session.UserID, session.AccessExpiresIn, nil
}
