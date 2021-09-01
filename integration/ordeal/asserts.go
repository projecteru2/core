package ordeal

import (
	"reflect"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
)

type Assertion struct {
	// maybe put etcd and redis client here
}

func (a Assertion) Assert(method string, t *testing.T, args map[string]interface{}, resp proto.Message, err error) {
	meth := reflect.ValueOf(a).MethodByName("Assert" + method)
	values := []reflect.Value{
		reflect.ValueOf(t),
		reflect.ValueOf(args),
		reflect.ValueOf(resp),
		reflect.ValueOf(err),
	}
	if err == nil {
		values[3] = reflect.New(reflect.TypeOf((*error)(nil)).Elem()).Elem()
	}
	meth.Call(values)
}

func (a Assertion) AssertAddPod(t *testing.T, args map[string]interface{}, resp proto.Message, err error) {
	assert.NoError(t, err)
}
