package interceptor

import (
	"context"
	"strings"

	"github.com/cenkalti/backoff/v4"
	"github.com/projecteru2/core/log"

	"google.golang.org/grpc"
)

// NewUnaryRetry makes unary RPC retry on error
func NewUnaryRetry(retryOpts RetryOptions) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return backoff.Retry(func() error {
			return invoker(ctx, method, req, reply, cc, opts...)
		}, backoff.WithMaxRetries(backoff.WithContext(backoff.NewExponentialBackOff(), ctx), uint64(retryOpts.Max)))
	}
}

// RPCNeedRetry records rpc stream methods to retry
var RPCNeedRetry = map[string]struct{}{
	"/pb.CoreRPC/WorkloadStatusStream": {},
	"/pb.CoreRPC/WatchServiceStatus":   {},
}

// NewStreamRetry make specific stream retry on error
func NewStreamRetry(retryOpts RetryOptions) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		stream, err := streamer(ctx, desc, cc, method, opts...)
		if _, ok := RPCNeedRetry[method]; !ok {
			return stream, err
		}
		log.Debugf("[NewStreamRetry] return retryStreawm for method %s", method)
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

func (s *retryStream) SendMsg(m interface{}) error {
	s.mux.Lock()
	s.sent = m
	s.mux.Unlock()
	return s.getStream().SendMsg(m)
}

func (s *retryStream) RecvMsg(m interface{}) (err error) {
	if err = s.ClientStream.RecvMsg(m); err == nil || strings.Contains(err.Error(), "context canceled") {
		return
	}

	return backoff.Retry(func() error {
		log.Debug("[retryStream] retry on new stream")
		stream, err := s.newStream()
		if err != nil {
			// even io.EOF triggers retry, and it's what we want!
			return err
		}
		s.setStream(stream)
		s.mux.RLock()
		err = s.getStream().SendMsg(s.sent)
		s.mux.RUnlock()
		if err != nil {
			return err
		}
		return s.getStream().RecvMsg(m)
	}, backoff.WithMaxRetries(backoff.WithContext(backoff.NewExponentialBackOff(), s.ctx), uint64(s.retryOpts.Max)))
}
