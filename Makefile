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
	glide i
	rm -rf ./vendor/github.com/docker/docker/vendor
	rm -rf ./vendor/github.com/docker/distribution/vendor

build: deps
	go build -ldflags "$(GO_LDFLAGS)" -a -tags netgo -installsuffix netgo -o eru-core

test: deps
	# fix mock docker client bug, see https://github.com/moby/moby/pull/34383 [docker 17.05.0-ce]
	sed -i "143s/\*http.Transport/http.RoundTripper/" ./vendor/github.com/docker/docker/client/client.go
	go vet `go list ./... | grep -v '/vendor/'`
	go test -v `glide nv`
