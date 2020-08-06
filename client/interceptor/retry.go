package interceptor

import (
	"context"

	"github.com/cenkalti/backoff/v4"
	"google.golang.org/grpc"
)

func NewUnaryRetry(retryOpts RetryOptions) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return backoff.Retry(func() error {
			return invoker(ctx, method, req, reply, cc, opts...)
		}, backoff.WithMaxRetries(backoff.WithContext(backoff.NewExponentialBackOff(), ctx), uint64(retryOpts.Max)))
	}
}

func NewStreamRetry(retryOpts RetryOptions) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		stream, err := streamer(ctx, desc, cc, method, opts...)
		if method != "/pb.CoreRPC/ContainerStatusStream" {
			return stream, err
		}
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
	if err = s.ClientStream.RecvMsg(m); err == nil {
		return
	}

	return backoff.Retry(func() error {
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
