package service

import (
	"context"
	"pkg/auth/sign"

	v1 "logic/api/logic/v1"
	"logic/internal/biz"
	"logic/internal/biz/bo"
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

// SendMessage 发送消息
func (s *LogicService) SendMessage(ctx context.Context, in *v1.SendMessageRequest) (*v1.SendMessageResponse, error) {
	if err := s.uc.ValidateRequest(ctx, &sign.SignParam{
		RequestID: in.RequestId,
		Signature: string(in.Signature),
		Timestamp: in.Timestamp,
	}); err != nil {
		return nil, err
	}
	messageId, err := s.uc.SendMessage(ctx, bo.NewMessage(in))
	if err != nil {
		return nil, err
	}
	return &v1.SendMessageResponse{MessageId: messageId}, nil
}
