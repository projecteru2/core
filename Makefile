.PHONY: golang python deps build test

REPO_PATH := gitlab.ricebook.net/platform/core
REVISION := $(shell git rev-parse HEAD || unknown)
BUILTAT := $(shell date +%Y-%m-%dT%H:%M:%S)
VERSION := $(shell cat VERSION)
GO_LDFLAGS ?= -s -X $(REPO_PATH)/versioninfo.REVISION=$(REVISION) \
			  -X $(REPO_PATH)/versioninfo.BUILTAT=$(BUILTAT) \
			  -X $(REPO_PATH)/versioninfo.VERSION=$(VERSION)

golang:
	cd ./rpc/gen/; protoc --go_out=plugins=grpc:. core.proto

python:
	cd ./rpc/gen/; python -m grpc.tools.protoc -I. --python_out=. --grpc_python_out=. core.proto; mv core_pb2.py ../../devtools/

deps:
	go get github.com/docker/docker
	go get github.com/docker/engine-api
	go get github.com/docker/go-units
	go get github.com/docker/go-connections
	go get github.com/coreos/etcd
	go get github.com/Sirupsen/logrus
	go get github.com/stretchr/testify
	go get github.com/golang/protobuf/proto
	go get github.com/codegangsta/cli
	go get gopkg.in/yaml.v2
	go get gopkg.in/libgit2/git2go.v23
	go get golang.org/x/net/context
	go get google.golang.org/grpc

build:
	go build -ldflags "$(GO_LDFLAGS)" -a -tags netgo -installsuffix netgo -o eru-core

test:
	go test ./...
