package biz

import (
	"context"
)

// ServiceClient 是调用 service 服务的客户端接口
type ServiceClient interface {
	// Chat 调用 service 服务的 Chat 方法
	Chat(ctx context.Context, name string) (string, error)

	// ChatStream 处理双向流式 RPC，返回一个可以发送和接收消息的流
	ChatStream(ctx context.Context, name string) (string, error)
}
