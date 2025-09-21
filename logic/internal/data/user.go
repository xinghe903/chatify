package data

import (
	"context"
	"logic/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

type userRepo struct {
	data *Data
	log  *log.Helper
}

// NewUserRepo .
func NewUserRepo(data *Data, logger log.Logger) biz.UserRepo {
	return &userRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

// IsUserOnline 检查用户是否在线
func (r *userRepo) IsUserOnline(ctx context.Context, userID string) (bool, error) {
	if userID == "" {
		return false, nil
	}

	// 构建Redis键
	redisKey := formatUserOnlineKey(userID)

	// 查询用户是否在线
	exists, err := r.data.RedisClient().HGet(ctx, redisKey, "status").Result()
	if err != nil {
		r.log.WithContext(ctx).Errorf("Failed to check user online status: %s", err.Error())
		return false, err
	}

	// 如果exists > 0，表示用户在线
	return len(exists) > 0, nil
}

func formatUserOnlineKey(userID string) string {
	return "chatify:user:session:" + userID
}
