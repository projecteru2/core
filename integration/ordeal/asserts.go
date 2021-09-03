package ordeal

import (
	"reflect"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"github.com/stretchr/testify/assert"
)

type Assertion struct {
	// maybe put etcd and redis client here
}

func (a Assertion) AssertUnary(method string, t *testing.T, req proto.Message, resp proto.Message, err error) {
	meth := reflect.ValueOf(a).MethodByName("Assert" + method)
	values := []reflect.Value{
		reflect.ValueOf(t),
		reflect.ValueOf(req),
		reflect.ValueOf(resp),
		reflect.ValueOf(err),
	}
	if err == nil {
		values[3] = reflect.New(reflect.TypeOf((*error)(nil)).Elem()).Elem()
	}
	meth.Call(values)
}

func (a Assertion) AssertStream(method string, t *testing.T, req proto.Message, stream *grpcdynamic.ServerStream, err error) {
	meth := reflect.ValueOf(a).MethodByName("Assert" + method)
	values := []reflect.Value{
		reflect.ValueOf(t),
		reflect.ValueOf(req),
		reflect.ValueOf(stream),
		reflect.ValueOf(err),
	}
	if err == nil {
		values[3] = reflect.New(reflect.TypeOf((*error)(nil)).Elem()).Elem()
	}
	meth.Call(values)
}

func (a Assertion) AssertAddPod(t *testing.T, req proto.Message, resp proto.Message, err error) {
	assert.NoError(t, err)
}

func (a Assertion) AssertCreateWorkload(t *testing.T, req proto.Message, stream *grpcdynamic.ServerStream, err error) {
	assert.NoError(t, err)
}
