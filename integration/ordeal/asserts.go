package ordeal

import (
	"log"
	"os"
	"testing"

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

func (a Assertion) AssertUnary(t *testing.T, reqjson, respjson, errString string, asserts []Assert) {
	a.AssertCustomed(t, reqjson, respjson, errString, asserts)
}

func (a Assertion) AssertStream(t *testing.T, reqjson string, respjsonCh chan string, errString string, asserts []Assert) {
	for respjson := range respjsonCh {
		log.Println(respjson)
		a.AssertCustomed(t, reqjson, respjson, errString, asserts)
	}
	return
}

func (a Assertion) AssertCustomed(t *testing.T, req, resp, err string, asserts []Assert) {
	envs := append(os.Environ(),
		"req="+req,
		"resp="+resp,
		"err="+err,
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
