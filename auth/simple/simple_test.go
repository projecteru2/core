package simple

import (
	"context"
	"errors"
	"testing"

	grpcmocks "github.com/projecteru2/core/3rdmocks"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

var defaultSrv = 1024

func streamHandler(srv interface{}, stream grpc.ServerStream) error {
	s, ok := srv.(int)
	if !ok {
		return errors.New("failed")
	}
	if s != defaultSrv {
		return errors.New("wrong srv")
	}
	return nil
}

func unaryHandler(ctx context.Context, req interface{}) (interface{}, error) {
	s, ok := req.(int)
	if !ok {
		return nil, errors.New("failed")
	}
	if s != defaultSrv {
		return nil, errors.New("wrong srv")
	}
	return s, nil
}

func TestBasicAuthStream(t *testing.T) {
	user := "test"
	pass := "pass"
	ba := NewBasicAuth(user, pass)
	ctx := context.Background()

	// no context
	mockServerStream := &grpcmocks.ServerStream{}
	mockServerStream.On("Context").Return(ctx)
	err := ba.StreamInterceptor(defaultSrv, mockServerStream, nil, streamHandler)
	assert.Error(t, err)
	// wrong username
	mockServerStream = &grpcmocks.ServerStream{}
	incomingCtx := metadata.NewIncomingContext(ctx, metadata.MD{"what": []string{}})
	mockServerStream.On("Context").Return(incomingCtx)
	err = ba.StreamInterceptor(defaultSrv, mockServerStream, nil, streamHandler)
	assert.Error(t, err)
	// wrong password
	mockServerStream = &grpcmocks.ServerStream{}
	incomingCtx = metadata.NewIncomingContext(ctx, metadata.MD{user: []string{}})
	mockServerStream.On("Context").Return(incomingCtx)
	err = ba.StreamInterceptor(defaultSrv, mockServerStream, nil, streamHandler)
	assert.Error(t, err)
	// vaild
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
	ctx := context.Background()
	// no meta context
	_, err := ba.UnaryInterceptor(ctx, defaultSrv, nil, unaryHandler)
	assert.Error(t, err)
	// wrong username
	incomingCtx := metadata.NewIncomingContext(ctx, metadata.MD{"what": []string{}})
	r, err := ba.UnaryInterceptor(incomingCtx, defaultSrv, nil, unaryHandler)
	assert.Error(t, err)
	assert.Nil(t, r)
	// wrong password
	incomingCtx = metadata.NewIncomingContext(ctx, metadata.MD{user: []string{}})
	r, err = ba.UnaryInterceptor(incomingCtx, defaultSrv, nil, unaryHandler)
	assert.Error(t, err)
	assert.Nil(t, r)
	// vaild
	incomingCtx = metadata.NewIncomingContext(ctx, metadata.MD{user: []string{pass}})
	r, err = ba.UnaryInterceptor(incomingCtx, defaultSrv, nil, unaryHandler)
	assert.NoError(t, err)
	s, ok := r.(int)
	assert.True(t, ok)
	assert.Equal(t, s, defaultSrv)
}
