package interceptor

import (
	"context"
	"sync"

	"github.com/cenkalti/backoff/v4"
	"github.com/cockroachdb/errors"
	"google.golang.org/grpc"

	"github.com/projecteru2/core/log"
)

type RetryOptions struct {
	Max uint64
}

type retryStream struct {
	ctx context.Context
	grpc.ClientStream
	mux       sync.RWMutex
	sent      any
	newStream func() (grpc.ClientStream, error)
	retryOpts RetryOptions
}

func (s *retryStream) SendMsg(m any) error {
	s.mux.Lock()
	s.sent = m
	s.mux.Unlock()
	return s.getStream().SendMsg(m)
}

func (s *retryStream) RecvMsg(m any) (err error) {
	if err = s.ClientStream.RecvMsg(m); err == nil || errors.Is(err, context.Canceled) {
		return err
	}
	logger := log.WithFunc("client.RecvMsg")

	return backoff.Retry(func() error {
		logger.Debug(s.ctx, "retry on new stream")
		stream, err := s.newStream()
		if err != nil {
			// io.EOF must trigger a retry too
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
	}, backoff.WithMaxRetries(backoff.WithContext(backoff.NewExponentialBackOff(), s.ctx), s.retryOpts.Max))
}

func (s *retryStream) getStream() grpc.ClientStream {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return s.ClientStream
}

func (s *retryStream) setStream(stream grpc.ClientStream) {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.ClientStream = stream
}
