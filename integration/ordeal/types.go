package ordeal

import (
	"github.com/golang/protobuf/proto"
	"github.com/jhump/protoreflect/desc"
)

//rpc represents method of grpc
type rpc struct {
	Method         *desc.MethodDescriptor
	RequestFactory func(map[string]interface{}) proto.Message
}

type TestCase struct {
	Method   string                   `yaml:"method"`
	Requests map[string][]interface{} `yaml:"requests"`
}
