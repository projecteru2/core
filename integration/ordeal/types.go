package ordeal

import (
	"github.com/golang/protobuf/proto"
	"github.com/jhump/protoreflect/desc"
)

//rpc represents method of grpc
type rpc struct {
	Method          *desc.MethodDescriptor
	RequestFactory  func(jsonbuf []byte) (proto.Message, error)
	ResponseMarshal func(proto.Message) (string, error)
}

type Assert struct {
	Equal struct {
		Actual   string `json:"actual"`
		Expected string `json:"expected"`
	} `json:"equal"`
}
