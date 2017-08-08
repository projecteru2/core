Core
====

Eru 体系的核心组件，无状态，采用悲观锁实现来分配资源。

## Develop

开发测试的时候，修改好了版本号，直接推到 gitlab 吧，build 完成了以后会自动发布到 mirrors.ricebook.net ，然后用部署脚本更新即可（见下方示范）。

## setup dev environment

`make deps` 可能非常耗时间, 建议开代理, 或者直接使用 `hub.ricebook.net/base/centos:onbuild-eru-core-2017.08.04` 这个镜像来操作。

在 OSX 下需要先安装 `libgit2` 假定已经安装了 [Homebrew](https://brew.sh/) 的前提下，执行：
```shell
# libgit2 锁定在 0.25.1
cd /usr/local/Homebrew/Library/Taps/homebrew/homebrew-core/Formula
gco 9c527911c8c630355d92df001575cacbb4a8b8b4 libgit2.rb
HOMEBREW_NO_AUTO_UPDATE=1 brew install libgit2
make deps
```

## Upgrade core on test/production server

```shell
make build
# test server
devtools/deploy.sh test
# prod server
devtools/deploy.sh prod
```

### GRPC

Generate golang & python code

```shell
go get -u github.com/golang/protobuf/{proto,protoc-gen-go}
make golang
make python
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
