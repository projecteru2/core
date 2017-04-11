Core
====

## DEV

开发测试的时候，修改好了版本号，直接推到 gitlab 吧，build 完成了以后会自动发布到 mirrors.ricebook.net ，然后用部署脚本更新即可（见下方示范）。

## setup dev environment

`make deps` 可能非常耗时间, 建议开代理, 或者直接从 hub.ricebook.net/base/centos:onbuild-eru-core-2017.03.04 这个镜像 copy.

```shell
git config --global url."git@gitlab.ricebook.net:".insteadOf "https://gitlab.ricebook.net/"
go get gitlab.ricebook.net/platform/core.git
mv $GOPATH/src/gitlab.ricebook.net/platform/core.git $GOPATH/src/gitlab.ricebook.net/platform/core
cd $GOPATH/src/gitlab.ricebook.net/platform/core
make deps
make build
```

## Upgrade core on test/production server

```shell
make build
# test server
devtools/upgrade-eru-core.sh test
# prod server
devtools/upgrade-eru-core.sh prod
```

### GRPC

Generate golang & python code

```shell
$ go get -u github.com/golang/protobuf/{proto,protoc-gen-go}
$ make golang
$ make python
```

Current version of dependencies are:

* google.golang.org/grpc: v1.0.1-GA
* github.com/golang/protobuf: f592bd283e

do not forget first command...

### deploy core on local environment

* start eru core

```shell
$ core --config core.yaml.sample --log-level debug
```

or

```shell
$ export ERU_CONFIG_PATH=/path/to/core.yaml
$ export ERU_LOG_LEVEL=DEBUG
$ core
```


### Use client.py

```
$ devtools/client.py --grpc-host core-grpc.intra.ricebook.net node:get intra c2-docker-7
```
