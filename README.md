Core
====
[![CircleCI](https://circleci.com/gh/projecteru2/core/tree/master.svg?style=shield)](https://circleci.com/gh/projecteru2/core/tree/master)
[![Codacy Badge](https://api.codacy.com/project/badge/Grade/e26ca3ee697d406caa9e49b0c491ff13)](https://www.codacy.com/app/CMGS/core?utm_source=github.com&amp;utm_medium=referral&amp;utm_content=projecteru2/core&amp;utm_campaign=Badge_Grade)

Eru 体系的核心组件，无状态，采用悲观锁实现来分配资源。


## 测试

执行 ``` make test ``` 即可

## 编译

执行 ``` make build ```，如果需要打包出 RPM 需要预先安装好 [FPM](https://github.com/jordansissel/fpm)，然后执行 ```./make-rpm ```

## 开发

`make deps` 可能非常耗时间, 建议开代理

在 macOS 下需要先安装 `libgit2` 假定已经安装了 [Homebrew](https://brew.sh/) 的前提下，执行：
```shell
# libgit2 锁定在 0.25.1
cd /usr/local/Homebrew/Library/Taps/homebrew/homebrew-core/Formula
gco 9c527911c8c630355d92df001575cacbb4a8b8b4 libgit2.rb
HOMEBREW_NO_AUTO_UPDATE=1 brew install libgit2
make deps
```

在 Linux 下可以参考这个用于 CI 测试的 [Dockerfile](https://github.com/projecteru2/core/blob/master/.circleci/Dockerfile)
我们是基于 CentOS 的体系，因此在 Ubuntu 下会略有不同

同时，你也可以使用 [footstone](https://hub.docker.com/r/projecteru2/footstone/) 来测试编译打包 core.

### GRPC

Generate golang & python code

```shell
go get -u github.com/golang/protobuf/{proto,protoc-gen-go}
make grpc
```

Current version of dependencies are:

* google.golang.org/grpc: v1.0.1-GA
* github.com/golang/protobuf: f592bd283e

do not forget first command...

### 本地部署

```shell
$ eru-core --config core.yaml.sample
```

或者

```shell
$ export ERU_CONFIG_PATH=/path/to/core.yaml
$ eru-core
```

### 使用 client.py 执行

```
$ devtools/client.py --grpc-host core-grpc.intra.ricebook.net node:get intra c2-docker-7
```

## Dockerized Core

Image: [projecteru2/core](https://hub.docker.com/r/projecteru2/core/)

```shell
docker run -d -e IN_DOCKER=1 \
  --name eru-core --net host \
  --restart always \
  -v <HOST_CONFIG_DIR_PATH>:/etc/eru \
  -v <HOST_BACKUP_DIR_PATH>:/data/backup \
  projecteru2/core \
  /usr/bin/eru-core
```
