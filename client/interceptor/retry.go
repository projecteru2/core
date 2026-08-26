package interceptor

import (
	"context"

	"google.golang.org/grpc"

	"github.com/projecteru2/core/log"
)

// RPCNeedRetry records rpc stream methods to retry
var RPCNeedRetry = map[string]struct{}{
	"/pb.CoreRPC/WorkloadStatusStream": {},
	"/pb.CoreRPC/WatchServiceStatus":   {},
}

// NewStreamRetry retries the stream methods listed in RPCNeedRetry.
func NewStreamRetry(retryOpts RetryOptions) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		stream, err := streamer(ctx, desc, cc, method, opts...)
		if _, ok := RPCNeedRetry[method]; !ok {
			return stream, err
		}
		logger := log.WithFunc("interceptor.NewStreamRetry")
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
