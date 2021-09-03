package ordeal

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"github.com/projecteru2/core/client"
	coretypes "github.com/projecteru2/core/types"
)

var (
	rpcs = make(map[string]*rpc)
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
					RequestFactory: func(method *desc.MethodDescriptor) func([]byte) (proto.Message, error) {
						return func(jsonbuf []byte) (proto.Message, error) {

							msg := dynamic.NewMessageFactoryWithDefaults().NewDynamicMessage(method.GetInputType())
							return msg, jsonpb.Unmarshal(bytes.NewReader(jsonbuf), msg)
						}
					}(method),
				}
			}
		}
	}

	m.Run()
}

func TestCases(t *testing.T) {
	client, err := client.NewClient(context.TODO(), "127.0.0.1:5001", coretypes.AuthConfig{})
	if err != nil {
		panic(fmt.Sprintf("failed to new eru client: %+v", err))
	}

	stub := grpcdynamic.NewStub(client.GetConn())
	assertion := Assertion{}
	f, err := os.Open("/tmp/_core_int_cases")
	if err != nil {
		panic("testcase file not found: /tmp/_core_int_cases")
	}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		method := scanner.Text()
		if !scanner.Scan() {
			panic("testcase stream broken")
		}

		testcase := scanner.Bytes()
		rpc, ok := rpcs[method]
		if !ok {
			panic(fmt.Errorf("method not found: %s", method))
		}

		req, err := rpc.RequestFactory(testcase)
		if err != nil {
			panic(fmt.Errorf("invalid request: %+v, %s", err, string(testcase)))
		}

		if rpc.Method.IsServerStreaming() {
			stream, err := stub.InvokeRpcServerStream(context.TODO(), rpc.Method, req)
			assertion.AssertStream(method, t, req, stream, err)

		} else {
			resp, err := stub.InvokeRpc(context.TODO(), rpc.Method, req)
			assertion.AssertUnary(method, t, req, resp, err)
		}
	}
}
