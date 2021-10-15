package pbreflect

import (
	"context"
	"io"
	"log"
	"strings"

	"github.com/golang/protobuf/jsonpb" // nolint:staticcheck // protoreflect relies on this
	"github.com/golang/protobuf/proto"  // nolint:staticcheck // protoreflect relies on this
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"github.com/projecteru2/core/client"
	coretypes "github.com/projecteru2/core/types"
)

// Service represents a grpc service
type Service struct {
	rpcs        map[string]*desc.MethodDescriptor
	pbMarshaler *jsonpb.Marshaler
	stub        grpcdynamic.Stub
}

// MustNewService .
func MustNewService() *Service {
	client, err := client.NewClient(context.TODO(), "127.0.0.1:5001", coretypes.AuthConfig{})
	if err != nil {
		log.Fatalf("failed to new eru client: %+v", err)
	}
	return &Service{
		rpcs: make(map[string]*desc.MethodDescriptor),
		pbMarshaler: &jsonpb.Marshaler{
			OrigName:     true,
			EnumsAsInts:  true,
			EmitDefaults: true,
		},
		stub: grpcdynamic.NewStub(client.GetConn()),
	}
}

type protoMessage struct {
	message proto.Message
	err     error
}

// Response represents a grpc response in json format including error
type Response struct {
	Content string
	Err     string
}

// Send a grpc request in json, get a channel with responses in json
func (s Service) Send(ctx context.Context, method, request string) (_ <-chan Response, err error) {
	meth, req, err := s.encodeResquest(method, request)
	if err != nil {
		return
	}

	var send func(context.Context, *desc.MethodDescriptor, proto.Message) <-chan protoMessage
	switch {
	case meth.IsServerStreaming():
		send = s.sendServerStream
	default:
		send = s.sendUnary
	}
	ch := send(context.TODO(), meth, req)

	respCh := make(chan Response)
	go func() {
		defer close(respCh)
		for msg := range ch {
			if msg.err != nil {
				respCh <- Response{Err: msg.err.Error()}
				continue
			}

			content, err := s.decodeResponse(msg.message)
			if err != nil {
				respCh <- Response{Err: err.Error()}
				continue
			}

			respCh <- Response{Content: content}
		}
	}()
	return respCh, nil
}

func (s Service) encodeResquest(method, request string) (*desc.MethodDescriptor, proto.Message, error) {
	meth := s.rpcs[method]
	req := dynamic.NewMessageFactoryWithDefaults().NewDynamicMessage(meth.GetInputType())
	return meth, req, jsonpb.Unmarshal(strings.NewReader(request), req)
}

func (s Service) decodeResponse(msg proto.Message) (content string, err error) {
	dym, err := dynamic.AsDynamicMessage(msg)
	if err != nil {
		return
	}
	return s.pbMarshaler.MarshalToString(dym)
}

func (s Service) sendServerStream(ctx context.Context, method *desc.MethodDescriptor, request proto.Message) <-chan protoMessage {
	ch := make(chan protoMessage)
	go func() {
		defer close(ch)
		stream, err := s.stub.InvokeRpcServerStream(ctx, method, request)
		if err != nil {
			ch <- protoMessage{err: err}
			return
		}

		for {
			msg, err := stream.RecvMsg()
			if err != nil {
				if err != io.EOF {
					ch <- protoMessage{err: err}
				}
				return
			}
			ch <- protoMessage{message: msg}

		}
	}()
	return ch
}

func (s Service) sendUnary(ctx context.Context, method *desc.MethodDescriptor, request proto.Message) <-chan protoMessage {
	ch := make(chan protoMessage)
	go func() {
		defer close(ch)
		resp, err := s.stub.InvokeRpc(ctx, method, request)
		ch <- protoMessage{message: resp, err: err}
	}()
	return ch
}
