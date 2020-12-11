.PHONY: grpc deps binary build test mock cloc unit-test

REPO_PATH := github.com/projecteru2/core
REVISION := $(shell git rev-parse HEAD || unknown)
BUILTAT := $(shell date +%Y-%m-%dT%H:%M:%S)
VERSION := $(shell git describe --tags $(shell git rev-list --tags --max-count=1))
GO_LDFLAGS ?= -s -X $(REPO_PATH)/version.REVISION=$(REVISION) \
			  -X $(REPO_PATH)/version.BUILTAT=$(BUILTAT) \
			  -X $(REPO_PATH)/version.VERSION=$(VERSION)

grpc:
	cd ./rpc/gen/; protoc --go_out=plugins=grpc:. core.proto

deps:
	env GO111MODULE=on go mod download
	env GO111MODULE=on go mod vendor

binary:
	CGO_ENABLED=0 go build -ldflags "$(GO_LDFLAGS)" -o eru-core

build: deps binary

test: deps unit-test

mock: deps
	mockery -dir ./vendor/google.golang.org/grpc -name ServerStream -output 3rdmocks
	mockery -dir vendor/github.com/docker/docker/client -name APIClient -output engine/docker/mocks

.ONESHELL:

cloc:
	cloc --exclude-dir=vendor,3rdmocks,mocks,tools --not-match-f=test .

unit-test:
	go vet `go list ./... | grep -v '/vendor/' | grep -v '/tools'`
	go test -timeout 120s -count=1 -cover ./utils/... \
	./types/... \
	./store/etcdv3/... \
	./source/common/... \
	./strategy/... \
	./scheduler/complex/... \
	./rpc/. \
	./lock/etcdlock/... \
	./auth/simple/... \
	./cluster/calcium/... \
	./discovery/helium... \
	./resources/types/. \
	./resources/storage/... \
	./resources/volume/... \
	./resources/cpumem/...

lint:
	golangci-lint run
