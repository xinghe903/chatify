package middleware

import (
	"context"
	"strconv"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
)

func ErrorEncoder() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			// 调用实际的 handler
			reply, err := handler(ctx, req)
			if err != nil {
				// 如果已经是 gRPC status error，直接返回
				// if _, ok := status.FromError(err); ok {
				// 	return reply, err
				// }
				// 尝试转换为 Kratos error
				if kratosErr := errors.FromError(err); kratosErr != nil {
					// 映射 Code 到 gRPC Code（简单示例）
					grpcCode := codes.Internal
					if kratosErr.Code >= 400 && kratosErr.Code < 500 {
						grpcCode = codes.InvalidArgument
					}
					if kratosErr.Metadata == nil {
						kratosErr.Metadata = map[string]string{}
					}
					kratosErr.Metadata["code"] = strconv.Itoa(int(kratosErr.Code))
					// 创建 gRPC status
					st, _ := status.New(grpcCode, kratosErr.Message).
						WithDetails(&errdetails.ErrorInfo{
							Reason:   kratosErr.Reason,
							Metadata: kratosErr.Metadata,
						})
					err = st.Err()
				}
			}

			return reply, err
		}
	}
}
