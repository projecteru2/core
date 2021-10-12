package testsuite

import (
	"context"
	"log"
	"testing"

	"github.com/projecteru2/core/int/grpc_assert/pbreflect"
)

type Testsuite struct {
	Method  string
	Request string
	*Assertion
}

func New(method, request, assertion string) *Testsuite {
	return &Testsuite{
		Method:    method,
		Request:   request,
		Assertion: MustNewAssertion(assertion),
	}
}

func (c Testsuite) Run(t *testing.T, service pbreflect.Service) {
	responses, err := service.Send(context.TODO(), c.Method, c.Request)
	if err != nil {
		log.Fatalf("failed to send request: %+v", err)
	}

	contents, errs := []string{}, []string{}
	for response := range responses {
		contents = append(contents, response.Content)
		errs = append(errs, response.Err)
		c.assertEach(t, c.Request, response.Content, response.Err)
	}
	c.assertCompletion(t, c.Request, contents, errs)
	return
}
