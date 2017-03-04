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
	go get -u -v -d github.com/opencontainers/runc/libcontainer/system
	go get -u -v -d github.com/docker/libtrust
	go get -u -v -d github.com/docker/distribution
	go get -u -v -d github.com/docker/docker/pkg/archive
	go get -u -v -d github.com/CMGS/statsd
	go get -u -v -d github.com/docker/go-units
	go get -u -v -d github.com/docker/go-connections
	go get -u -v -d github.com/Sirupsen/logrus
	go get -u -v -d github.com/stretchr/testify
	go get -u -v -d github.com/golang/protobuf/{proto,protoc-gen-go}
	go get -u -v -d github.com/codegangsta/cli
	go get -u -v -d gopkg.in/yaml.v2
	go get -u -v -d gopkg.in/libgit2/git2go.v25
	go get -u -v -d golang.org/x/net/context
	go get -u -v -d google.golang.org/grpc
	go get -u -v -d github.com/coreos/etcd
	go get -u -v -d github.com/docker/docker/api/types || echo oops
	go get -u -v -d github.com/docker/docker/api/types/container

build:
	go build -ldflags "$(GO_LDFLAGS)" -a -tags netgo -installsuffix netgo -o eru-core

test:
	go test ./...
