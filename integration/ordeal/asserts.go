package ordeal

import (
	"log"
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

func (a Assertion) AssertUnary(method string, t *testing.T, req *dynamic.Message, resp proto.Message, err error) {
	meth := reflect.ValueOf(a).MethodByName("Assert" + method)
	emptyValue := reflect.Value{}
	if meth == emptyValue {
		a.AssertNoError(t, method, req, err)
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
		reflect.ValueOf(req),
		reflect.ValueOf(dyResp),
		reflect.ValueOf(err),
	}
	if resp == nil {
		values[2] = reflect.New(reflect.TypeOf((*dynamic.Message)(nil))).Elem()
	}
	if err == nil {
		values[3] = reflect.New(reflect.TypeOf((*error)(nil)).Elem()).Elem()
	}
	meth.Call(values)
}

func (a Assertion) AssertStream(method string, t *testing.T, req *dynamic.Message, stream *grpcdynamic.ServerStream, err error) {
	meth := reflect.ValueOf(a).MethodByName("Assert" + method)
	emptyValue := reflect.Value{}
	if meth == emptyValue {
		a.AssertNoError(t, method, req, err)
		return
	}

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

func (a Assertion) AssertNoError(t *testing.T, method string, req *dynamic.Message, err error) {
	assert.NoError(t, err, method)
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
