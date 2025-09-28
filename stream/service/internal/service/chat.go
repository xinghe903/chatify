package service

import (
	"context"

	v1 "service/api/service/v1"
	"service/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

// ChatService is a greeter service.
type ChatService struct {
	v1.UnimplementedServiceServer
	log *log.Helper
	uc  *biz.GreeterUsecase
}

// NewChatService new a greeter service.
func NewChatService(uc *biz.GreeterUsecase, logger log.Logger) *ChatService {
	return &ChatService{uc: uc, log: log.NewHelper(logger)}
}

// SayHello implements helloworld.GreeterServer.
func (s *ChatService) SayHello(ctx context.Context, in *v1.ChatReq) (*v1.ChatRsp, error) {
	g, err := s.uc.CreateGreeter(ctx, &biz.Greeter{Hello: in.Name})
	if err != nil {
		return nil, err
	}
	return &v1.ChatRsp{Message: "Hello " + g.Hello}, nil
}

// Chat implements a bidirectional streaming RPC
func (s *ChatService) Chat(stream v1.Service_ChatServer) error {
	ctx := stream.Context()
	// 建立连接
	s.log.WithContext(ctx).Debugf("Chat connection established")

	// 监听接收消息
	for {
		req, err := stream.Recv()
		ctx = stream.Context()
		if err != nil {
			s.log.WithContext(ctx).Errorf("Stream closed or error occurred:", err)
			return err
		}

		// 打印输入内容
		s.log.WithContext(ctx).Debugf("Received message: %+v\n", req)
	}
}
