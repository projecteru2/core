package ordeal

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"github.com/jinzhu/configor"
	"github.com/projecteru2/core/client"
	coretypes "github.com/projecteru2/core/types"
)

var (
	rpcs      = make(map[string]*rpc)
	testcases = []TestCase{}
	asserts   = []func(*testing.T, proto.Message, error){}
)

func TestMain(m *testing.M) {
	pwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	importPath := filepath.Join(filepath.Dir(filepath.Dir(pwd)), "rpc/gen/")
	fileNames, err := protoparse.ResolveFilenames([]string{importPath}, "core.proto")
	if err != nil {
		panic(err)
	}
	p := protoparse.Parser{
		ImportPaths:           []string{importPath},
		InferImportPaths:      false,
		IncludeSourceCodeInfo: true,
	}
	parsedFiles, err := p.ParseFiles(fileNames...)
	if err != nil {
		panic(err)
	}

	if len(parsedFiles) < 1 {
		panic("proto file not found")
	}

	for _, parsedFile := range parsedFiles {
		for _, service := range parsedFile.GetServices() {
			for _, method := range service.GetMethods() {
				rpcs[method.GetName()] = &rpc{
					Method: method,
					RequestFactory: func(method *desc.MethodDescriptor) func(map[string]interface{}) proto.Message {
						return func(args map[string]interface{}) proto.Message {
							dmsg := dynamic.NewMessage(method.GetInputType())
							for field, val := range args {
								dmsg.SetFieldByName(field, val)
							}
							msg := dynamic.NewMessageFactoryWithDefaults().NewDynamicMessage(method.GetInputType())
							dmsg.ConvertTo(msg)
							return msg
						}
					}(method),
				}
			}
		}
	}

	configor.Load(&testcases, filepath.Join(pwd, "testcases.yaml"))
	m.Run()
}

func TestCases(t *testing.T) {
	client, err := client.NewClient(context.TODO(), "127.0.0.1:5001", coretypes.AuthConfig{})
	if err != nil {
		panic(fmt.Sprintf("failed to new eru client: %+v", err))
	}

	stub := grpcdynamic.NewStub(client.GetConn())
	assertion := Assertion{}
	for _, testcase := range testcases {
		rpc, ok := rpcs[testcase.Method]
		if !ok {
			panic(fmt.Errorf("method not found: %s", testcase.Method))
		}

		for args := range requestCombinations(testcase.Requests) {
			resp, err := stub.InvokeRpc(context.TODO(), rpc.Method, rpc.RequestFactory(args))

			assertion.Assert(testcase.Method, t, args, resp, err)
		}

	}
}
