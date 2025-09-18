package service

import (
	"context"

	v1 "logic/api/logic/v1"
	"logic/internal/biz"
)

// LogicService is a greeter service.
type LogicService struct {
	v1.UnimplementedLogicServiceServer

	uc *biz.Logic
}

// NewLogicService new a greeter service.
func NewLogicService(uc *biz.Logic) *LogicService {
	return &LogicService{uc: uc}
}

// SayHello implements helloworld.GreeterServer.
func (s *LogicService) SendMessage(ctx context.Context, in *v1.SendMessageRequest) (*v1.SendMessageResponse, error) {
	return nil, nil
}
