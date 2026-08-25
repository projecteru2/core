package interceptor

import (
	"context"

	"github.com/cenkalti/backoff/v4"
	"google.golang.org/grpc"

	"github.com/projecteru2/core/log"
)

// RPCNeedRetry records rpc stream methods to retry
var RPCNeedRetry = map[string]struct{}{
	"/pb.CoreRPC/WorkloadStatusStream": {},
	"/pb.CoreRPC/WatchServiceStatus":   {},
}

// NewUnaryRetry makes unary RPC retry on error
func NewUnaryRetry(retryOpts RetryOptions) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return backoff.Retry(func() error {
			return invoker(ctx, method, req, reply, cc, opts...)
		}, backoff.WithMaxRetries(backoff.WithContext(backoff.NewExponentialBackOff(), ctx), retryOpts.Max))
	}
}

// NewStreamRetry retries the stream methods listed in RPCNeedRetry.
func NewStreamRetry(retryOpts RetryOptions) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		stream, err := streamer(ctx, desc, cc, method, opts...)
		if _, ok := RPCNeedRetry[method]; !ok {
			return stream, err
		}
		logger := log.WithFunc("client.NewStreamRetry")
		logger.Debugf(ctx, "return retryStream for method %s", method)
		return &retryStream{
			ctx:          ctx,
			ClientStream: stream,
			newStream: func() (grpc.ClientStream, error) {
				return streamer(ctx, desc, cc, method, opts...)
			},
			retryOpts: retryOpts,
		}, err
	}
}
