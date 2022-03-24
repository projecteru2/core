.PHONY: grpc deps binary build test mock cloc unit-test

REPO_PATH := github.com/projecteru2/core
REVISION := $(shell git rev-parse HEAD || unknown)
BUILTAT := $(shell date +%Y-%m-%dT%H:%M:%S)
VERSION := $(shell git describe --tags $(shell git rev-list --tags --max-count=1))
GO_LDFLAGS ?= -X $(REPO_PATH)/version.REVISION=$(REVISION) \
			  -X $(REPO_PATH)/version.BUILTAT=$(BUILTAT) \
			  -X $(REPO_PATH)/version.VERSION=$(VERSION)
ifneq ($(KEEP_SYMBOL), 1)
	GO_LDFLAGS += -s
endif

grpc:
	protoc --go_out=. --go-grpc_out=. \
	--go_opt=paths=source_relative \
	--go-grpc_opt=require_unimplemented_servers=false,paths=source_relative \
	./rpc/gen/core.proto

deps:
	env GO111MODULE=on go mod download
	env GO111MODULE=on go mod vendor

binary:
	CGO_ENABLED=0 go build -ldflags "$(GO_LDFLAGS)" -o eru-core

build: deps binary

test: deps unit-test

mock: deps
	mockery --dir vendor/google.golang.org/grpc --output 3rdmocks --name ServerStream
	mockery --dir vendor/github.com/docker/docker/client --output engine/docker/mocks --name APIClient
	mockery --dir scheduler --output scheduler/mocks --name Scheduler
	mockery --dir source --output source/mocks --name Source
	mockery --dir store --output store/mocks --name Store
	mockery --dir engine --output engine/mocks --name API
	mockery --dir cluster --output cluster/mocks --name Cluster
	mockery --dir wal --output wal/mocks --name WAL
	mockery --dir lock --output lock/mocks --name DistributedLock
	mockery --dir store/etcdv3/meta --output store/etcdv3/meta/mocks --all
	mockery --dir vendor/go.etcd.io/etcd/client/v3 --output store/etcdv3/meta/mocks --name Txn
	mockery --dir rpc/gen/ --output rpc/mocks --name CoreRPC_RunAndWaitServer

.ONESHELL:

cloc:
	cloc --exclude-dir=vendor,3rdmocks,mocks,tools,gen --not-match-f=test .

unit-test:
	go vet `go list ./... | grep -v '/vendor/' | grep -v '/tools'` && \
	go test -race -timeout 240s -count=1 -cover ./utils/... \
	./types/... \
	./store/etcdv3/. \
	./store/etcdv3/embedded/. \
	./store/etcdv3/meta/. \
	./source/common/... \
	./strategy/... \
	./scheduler/complex/... \
	./rpc/. \
	./lock/etcdlock/... \
	./auth/simple/... \
	./discovery/helium... \
	./resources/types/. \
	./resources/storage/... \
	./resources/volume/... \
	./resources/cpumem/... \
	./wal/. \
	./wal/kv/. \
	./store/redis/... \
	./lock/redis/... && \
	go test -timeout 240s -count=1 -cover ./cluster/calcium/...

lint:
	golangci-lint run
