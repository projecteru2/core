.PHONY: grpc deps binary build test mock cloc unit-test

REPO_PATH := github.com/projecteru2/core
REVISION := $(shell git rev-parse HEAD || unknown)
BUILTAT := $(shell date +%Y-%m-%dT%H:%M:%S)
VERSION := $(shell cat VERSION)
GO_LDFLAGS ?= -s -X $(REPO_PATH)/versioninfo.REVISION=$(REVISION) \
			  -X $(REPO_PATH)/versioninfo.BUILTAT=$(BUILTAT) \
			  -X $(REPO_PATH)/versioninfo.VERSION=$(VERSION)

grpc:
	cd ./rpc/gen/; protoc --go_out=plugins=grpc:. core.proto

deps:
	env GO111MODULE=on go mod download
	env GO111MODULE=on go mod vendor
	# fix mock docker client bug, see https://github.com/moby/moby/pull/34383 [docker 17.05.0-ce]
	sed -i.bak "143s/\*http.Transport/http.RoundTripper/" ./vendor/github.com/docker/docker/client/client.go

binary:
	go build -ldflags "$(GO_LDFLAGS)" -a -tags "netgo osusergo" -installsuffix netgo -o eru-core

build: deps binary

test: deps unit-test

mock: deps
	mockery -dir ./vendor/google.golang.org/grpc -name ServerStream -output 3rdmocks
	mockery -dir vendor/github.com/docker/docker/client -name APIClient -output engine/docker/mocks

.ONESHELL:
libgit2:
	cd /tmp
	rm -fr libgit2-0.28.5.tar.gz libgit2-0.28.5
	curl -Lv -O https://github.com/libgit2/libgit2/releases/download/v0.28.5/libgit2-0.28.5.tar.gz
	tar xvfz libgit2-0.28.5.tar.gz
	mkdir -p libgit2-0.28.5/build
	cd libgit2-0.28.5/build
	cmake ..
	cmake --build .
	sudo cp libgit2.pc /usr/lib/pkgconfig/
	sudo cp libgit2.so.0.28.5 /usr/lib
	sudo ln -s /usr/lib/libgit2.so.0.28.5 /usr/lib/libgit2.so.28
	sudo ln -s /usr/lib/libgit2.so.28 /usr/lib/libgit2.so
	sudo cp -aR ../include/* /usr/local/include/

cloc:
	cloc --exclude-dir=vendor,3rdmocks,mocks,tools --not-match-f=test .

unit-test:
	go vet `go list ./... | grep -v '/vendor/' | grep -v '/tools'`
	go test -timeout 120s -count=1 -cover ./utils/... ./types/... ./store/etcdv3/... ./source/common/... ./scheduler/complex/... ./rpc/. ./lock/etcdlock/... ./auth/simple/... ./cluster/calcium/...
