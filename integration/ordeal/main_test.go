package ordeal

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
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
		log.Fatalf("failed to get working directory: %+v", err)
	}

	importPath := filepath.Join(filepath.Dir(filepath.Dir(pwd)), "rpc/gen/")
	fileNames, err := protoparse.ResolveFilenames([]string{importPath}, "core.proto")
	if err != nil {
		log.Fatalf("failed to resolve import paths: %+v", err)
	}
	p := protoparse.Parser{
		ImportPaths:           []string{importPath},
		InferImportPaths:      false,
		IncludeSourceCodeInfo: true,
	}
	parsedFiles, err := p.ParseFiles(fileNames...)
	if err != nil {
		log.Fatalf("failed to parse proto file: %+v", err)
	}

	if len(parsedFiles) < 1 {
		log.Fatalf("proto file not found")
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
					ResponseMarshal: func(method *desc.MethodDescriptor) func(proto.Message) (string, error) {
						return func(m proto.Message) (json string, err error) {
							dym, err := dynamic.AsDynamicMessage(m)
							if err != nil {
								return
							}
							marshaler := jsonpb.Marshaler{
								OrigName:     true,
								EnumsAsInts:  true,
								EmitDefaults: true,
							}
							return marshaler.MarshalToString(dym)
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
		log.Fatalf("failed to new eru client: %+v", err)
	}

	stub := grpcdynamic.NewStub(client.GetConn())
	assertion := NewAssertion()
	f, err := os.Open("/tmp/_core_int_cases")
	if err != nil {
		log.Fatalf("testcase file not found: /tmp/_core_int_cases")
	}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		method := scanner.Text()
		if !scanner.Scan() {
			log.Fatalf("request stream broken")
		}

		request := scanner.Bytes()
		if !scanner.Scan() {
			log.Fatalf("assert stream broken")
		}

		assertsBuf := scanner.Bytes()
		asserts := []Assert{}
		if err = json.Unmarshal(assertsBuf, &asserts); err != nil {
			log.Fatalf("failed to parse asserts json: %+v, %s", err, string(assertsBuf))
		}

		rpc, ok := rpcs[method]
		if !ok {
			log.Fatalf("method not found: %s", method)
		}

		req, err := rpc.RequestFactory(request)
		if err != nil {
			log.Fatalf("invalid request: %+v, %s", err, string(request))
		}

		if rpc.Method.IsServerStreaming() {
			stream, err := stub.InvokeRpcServerStream(context.TODO(), rpc.Method, req)
			respJSONCh, errString := make(chan string), ""
			if err == nil {
				go func() {
					defer close(respJSONCh)
					for {
						msg, err := stream.RecvMsg()
						if err == io.EOF {
							return
						} else if err != nil {
							log.Fatalf("receive error from stream: %+v", err)
						}
						respJSON, err := rpc.ResponseMarshal(msg)
						if err != nil {
							log.Fatalf("invalid resp: %+v", err)
						}
						respJSONCh <- respJSON
					}
				}()
			} else {
				close(respJSONCh)
				errString = err.Error()
			}
			assertion.AssertStream(t, string(request), respJSONCh, errString, asserts)

		} else {
			resp, err := stub.InvokeRpc(context.TODO(), rpc.Method, req)
			var respJSON, errString string
			if err == nil {
				if respJSON, err = rpc.ResponseMarshal(resp); err != nil {
					log.Fatalf("invalid resp: %+v", err)
				}
			} else {
				errString = err.Error()
			}
			assertion.AssertUnary(t, string(request), respJSON, errString, asserts)
		}
	}
}
