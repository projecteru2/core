package ordeal

import (
	"log"
	"os"
	"reflect"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"github.com/stretchr/testify/assert"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type Assertion struct {
	// maybe put etcd and redis client here
	Etcd *clientv3.Client
}

func NewAssertion() *Assertion {
	etcd, err := clientv3.New(clientv3.Config{
		Endpoints: []string{"127.0.0.1:2379"},
	})
	if err != nil {
		log.Fatalf("failed to connect etcd: %+v", err)
	}
	return &Assertion{
		Etcd: etcd,
	}
}

func (a Assertion) AssertUnary(method string, t *testing.T, req proto.Message, reqjson string, resp proto.Message, respErr error, asserts []Assert) {
	dyReq, err := dynamic.AsDynamicMessage(req)
	if err != nil {
		log.Fatalf("failed to convert proto.Message: %+v", err)
	}

	meth := reflect.ValueOf(a).MethodByName("Assert" + method)
	emptyValue := reflect.Value{}
	if meth == emptyValue {
		a.AssertNoError(t, method, dyReq, respErr)
		return
	}

	var dyResp *dynamic.Message
	if resp != nil {
		dyResp, err = dynamic.AsDynamicMessage(resp)
		if err != nil {
			log.Fatalf("fail to convert proto.Message: %+v", err)
		}
	}
	values := []reflect.Value{
		reflect.ValueOf(t),
		reflect.ValueOf(dyReq),
		reflect.ValueOf(dyResp),
		reflect.ValueOf(respErr),
	}
	if resp == nil {
		values[2] = reflect.New(reflect.TypeOf((*dynamic.Message)(nil))).Elem()
	}
	if respErr == nil {
		values[3] = reflect.New(reflect.TypeOf((*error)(nil)).Elem()).Elem()
	}
	meth.Call(values)
	if respErr == nil {
		a.AssertCustomed(t, reqjson, asserts)
	}
}

func (a Assertion) AssertStream(method string, t *testing.T, req proto.Message, reqjson string, stream *grpcdynamic.ServerStream, respErr error, asserts []Assert) {
	dyReq, err := dynamic.AsDynamicMessage(req)
	if err != nil {
		log.Fatalf("failed to convert proto.Message: %+v", err)
	}

	meth := reflect.ValueOf(a).MethodByName("Assert" + method)
	emptyValue := reflect.Value{}
	if meth == emptyValue {
		a.AssertNoError(t, method, dyReq, respErr)
		return
	}

	values := []reflect.Value{
		reflect.ValueOf(t),
		reflect.ValueOf(dyReq),
		reflect.ValueOf(stream),
		reflect.ValueOf(respErr),
	}
	if respErr == nil {
		values[3] = reflect.New(reflect.TypeOf((*error)(nil)).Elem()).Elem()
	}
	meth.Call(values)
	if respErr == nil {
		a.AssertCustomed(t, reqjson, asserts)
	}
}

func (a Assertion) AssertNoError(t *testing.T, method string, req *dynamic.Message, err error) {
	assert.NoError(t, err, method)
}

func (a Assertion) AssertCustomed(t *testing.T, req string, asserts []Assert) {
	envs := append(os.Environ(),
		"req="+req,
	)
	for _, ass := range asserts {
		actualOut, err := bash(ass.Equal.Actual, envs)
		if err != nil {
			log.Fatalf("failed to exec bash command: %+v, %s", err, ass.Equal.Actual)
		}
		expectedOut, err := bash(ass.Equal.Expected, envs)
		if err != nil {
			log.Fatalf("failed to exec bash command: %+v, %s", err, ass.Equal.Expected)
		}
		assert.EqualValues(t, actualOut, expectedOut)
	}
}

func (a Assertion) AssertAddPod(t *testing.T, req *dynamic.Message, resp *dynamic.Message, err error) {
	name := req.GetFieldByName("name").(string)
	if name == "" {
		assert.EqualError(t, err, "rpc error: code = Code(1021) desc = pod name is empty")
		return
	}
	assert.NoError(t, err)
}

func (a Assertion) AssertCreateWorkload(t *testing.T, req *dynamic.Message, stream *grpcdynamic.ServerStream, err error) {
	assert.NoError(t, err)
}
