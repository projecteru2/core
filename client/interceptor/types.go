package interceptor

import (
	"context"
	"sync"

	"google.golang.org/grpc"
)

// RetryOptions .
type RetryOptions struct {
	Max int
}

type retryStream struct {
	ctx context.Context
	grpc.ClientStream
	mux       sync.RWMutex
	sent      any
	newStream func() (grpc.ClientStream, error)
	retryOpts RetryOptions
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
