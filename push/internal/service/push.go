package service

import (
	"context"

	v1 "push/api/push/v1"
	"push/internal/biz"
)

// pushService is a greeter service.
type PushService struct {
	v1.UnimplementedPushServiceServer

	uc *biz.Push
}

// NewPushService new a greeter service.
func NewPushService(uc *biz.Push) *PushService {
	return &PushService{uc: uc}
}

// SayHello implements helloworld.GreeterServer.
func (s *PushService) PushServiceClient(ctx context.Context, in *v1.PushRequest) (*v1.PushResponse, error) {
	return nil, nil
}
