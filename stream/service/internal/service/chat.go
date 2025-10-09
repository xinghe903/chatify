package service

import (
	"context"
	"io"

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

func (s *ChatService) Chat(ctx context.Context, req *v1.ChatReq) (*v1.ChatRsp, error) {
	s.log.WithContext(ctx).Debugf("Received message: %+v\n", req)
	return &v1.ChatRsp{Message: "Hello " + req.Name}, nil
}

// Chat implements a bidirectional streaming RPC
func (s *ChatService) ChatStream(stream v1.Service_ChatStreamServer) error {
	ctx := stream.Context()
	// 建立连接
	s.log.WithContext(ctx).Debugf("Chat connection established")

	// 监听接收消息
	for {
		req, err := stream.Recv()
		ctx = stream.Context()
		if err != nil {
			// 客户端断开连接
			if ctx.Err() == context.Canceled {
				s.log.WithContext(ctx).Debugf("Client canceled the request")
				return nil
			}
			if err == io.EOF {
				s.log.WithContext(ctx).Debugf("Client disconnected")
				return nil
			}
			s.log.WithContext(ctx).Errorf("Stream closed or error occurred:", err)
			return err
		}

		// 打印输入内容
		s.log.WithContext(ctx).Debugf("Received message: %+v\n", req)
	}
}
