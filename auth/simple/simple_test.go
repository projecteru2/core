package simple

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	grpcmocks "github.com/projecteru2/core/3rdmocks"
)

const defaultSrv = 1024

func TestBasicAuthStream(t *testing.T) {
	user := "test"
	pass := "pass"
	ba := NewBasicAuth(user, pass)
	ctx := t.Context()

	mockServerStream := &grpcmocks.ServerStream{}
	mockServerStream.On("Context").Return(ctx)
	err := ba.StreamInterceptor(defaultSrv, mockServerStream, nil, streamHandler)
	assert.Error(t, err)
	mockServerStream = &grpcmocks.ServerStream{}
	incomingCtx := metadata.NewIncomingContext(ctx, metadata.MD{"what": []string{}})
	mockServerStream.On("Context").Return(incomingCtx)
	err = ba.StreamInterceptor(defaultSrv, mockServerStream, nil, streamHandler)
	assert.Error(t, err)
	mockServerStream = &grpcmocks.ServerStream{}
	incomingCtx = metadata.NewIncomingContext(ctx, metadata.MD{user: []string{}})
	mockServerStream.On("Context").Return(incomingCtx)
	err = ba.StreamInterceptor(defaultSrv, mockServerStream, nil, streamHandler)
	assert.Error(t, err)
	mockServerStream = &grpcmocks.ServerStream{}
	incomingCtx = metadata.NewIncomingContext(ctx, metadata.MD{user: []string{pass}})
	mockServerStream.On("Context").Return(incomingCtx)
	err = ba.StreamInterceptor(defaultSrv, mockServerStream, nil, streamHandler)
	assert.NoError(t, err)
}

func TestBasicAuthUnary(t *testing.T) {
	user := "test"
	pass := "pass"
	ba := NewBasicAuth(user, pass)
	ctx := t.Context()
	_, err := ba.UnaryInterceptor(ctx, defaultSrv, nil, unaryHandler)
	assert.Error(t, err)
	incomingCtx := metadata.NewIncomingContext(ctx, metadata.MD{"what": []string{}})
	r, err := ba.UnaryInterceptor(incomingCtx, defaultSrv, nil, unaryHandler)
	assert.Error(t, err)
	assert.Nil(t, r)
	incomingCtx = metadata.NewIncomingContext(ctx, metadata.MD{user: []string{}})
	r, err = ba.UnaryInterceptor(incomingCtx, defaultSrv, nil, unaryHandler)
	assert.Error(t, err)
	assert.Nil(t, r)
	incomingCtx = metadata.NewIncomingContext(ctx, metadata.MD{user: []string{pass}})
	r, err = ba.UnaryInterceptor(incomingCtx, defaultSrv, nil, unaryHandler)
	assert.NoError(t, err)
	s, ok := r.(int)
	assert.True(t, ok)
	assert.Equal(t, s, defaultSrv)
}

func streamHandler(srv any, stream grpc.ServerStream) error {
	s, ok := srv.(int)
	if !ok {
		return errors.New("failed")
	}
	if s != defaultSrv {
		return errors.New("wrong srv")
	}
	return nil
}

func unaryHandler(ctx context.Context, req any) (any, error) {
	s, ok := req.(int)
	if !ok {
		return nil, errors.New("failed")
	}
	if s != defaultSrv {
		return nil, errors.New("wrong srv")
	}
	return s, nil
}
