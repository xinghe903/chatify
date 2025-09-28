// 修改 stream/service/internal/server/wrapper_stream.go 文件
package server

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type StreamTracingInterceptor struct {
	serviceName string
}

func NewStreamTracingInterceptor(serviceName string) *StreamTracingInterceptor {
	return &StreamTracingInterceptor{serviceName: serviceName}
}

// 服务端流拦截器
func (sti *StreamTracingInterceptor) StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		// 从上下文中提取 trace 上下文
		ctx := ss.Context()
		tracer := otel.Tracer(sti.serviceName)

		// 创建连接级别的 span
		ctx, connectionSpan := tracer.Start(ctx, "stream-connection-"+info.FullMethod)
		defer connectionSpan.End()

		tracingStream := &serverTracingStream{
			ServerStream:   ss,
			ctx:            ctx,
			connectionSpan: connectionSpan,
			tracer:         tracer,
			fullMethod:     info.FullMethod,
		}

		err := handler(srv, tracingStream)
		return err
	}
}

// 包装的服务端Stream
type serverTracingStream struct {
	grpc.ServerStream
	ctx            context.Context
	connectionSpan trace.Span
	tracer         trace.Tracer
	fullMethod     string
	currentMessageCtx context.Context
}

func (sts *serverTracingStream) Context() context.Context {
	if sts.currentMessageCtx != nil {
		return sts.currentMessageCtx
	}
	return sts.ctx
}

func (sts *serverTracingStream) RecvMsg(m interface{}) error {
	// 从 gRPC 上下文中提取 metadata
	md, ok := metadata.FromIncomingContext(sts.ctx)
	if ok {
		// 创建一个新的上下文，将 trace 上下文从 metadata 中提取出来
		propagator := otel.GetTextMapPropagator()
		messageCtx := propagator.Extract(sts.ctx, propagation.HeaderCarrier(md))

		// 创建消息级别的 span，并确保它是连接级 span 的子 span
		messageCtx, messageSpan := sts.tracer.Start(messageCtx, "stream-message-receive",
			trace.WithAttributes(attribute.String("grpc.method", sts.fullMethod)),
			trace.WithLinks(trace.Link{SpanContext: sts.connectionSpan.SpanContext()}))
		defer messageSpan.End()

		// 存储当前消息上下文
		sts.currentMessageCtx = messageCtx
	}

	err := sts.ServerStream.RecvMsg(m)
	return err
}

func (sts *serverTracingStream) SendMsg(m interface{}) error {
	err := sts.ServerStream.SendMsg(m)
	return err
}
