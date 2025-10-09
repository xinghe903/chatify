package auth

import "context"

// 定义自定义键类型来避免冲突
type contextKey string

const (
	USER_ID   = contextKey("x-user-id")
	USER_NAME = contextKey("x-user-name")
)

func GetUserID(ctx context.Context) string {
	if userID, ok := ctx.Value(USER_ID).(string); ok {
		return userID
	}
	return ""
}

func GetUserName(ctx context.Context) string {
	if userName, ok := ctx.Value(USER_NAME).(string); ok {
		return userName
	}
	return ""
}

func SetUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, USER_ID, userID)
}

func SetUserName(ctx context.Context, userName string) context.Context {
	return context.WithValue(ctx, USER_NAME, userName)
}

func NewContext(ctx context.Context, userID, userName string) context.Context {
	ctx = SetUserID(ctx, userID)
	ctx = SetUserName(ctx, userName)
	return ctx
}
