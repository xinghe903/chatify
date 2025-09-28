package service

import (
	"context"

	v1 "service/api/service/v1"
	"service/internal/biz"
)

// GreeterService is a greeter service.
type GreeterService struct {
	v1.UnimplementedServiceServer

	uc *biz.GreeterUsecase
}

// NewGreeterService new a greeter service.
func NewGreeterService(uc *biz.GreeterUsecase) *GreeterService {
	return &GreeterService{uc: uc}
}

// SayHello implements helloworld.GreeterServer.
func (s *GreeterService) SayHello(ctx context.Context, in *v1.ChatReq) (*v1.ChatRsp, error) {
	g, err := s.uc.CreateGreeter(ctx, &biz.Greeter{Hello: in.Name})
	if err != nil {
		return nil, err
	}
	return &v1.ChatRsp{Message: "Hello " + g.Hello}, nil
}
