Core
====

## DEV

生产服部署在 c2-eru-1，测试服部署在 c1-eru-1，如果需要测试，先更新 core，然后这样子部署：

```shell
devtools/upgrade-eru-core.sh test
devtools/upgrade-eru-core.sh prod
```


## setup dev environment

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
devtools/upgrade-eru-core.sh test eru-core
# prod server
devtools/upgrade-eru-core.sh
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

* create `core.yaml` like this

```yaml
bind: ":5000" # gRPC server 监听地址
agent_port: "12345" # agent 的 HTTP API 端口, 暂时没有用到
permdir: "/mnt/mfs/permdirs" # 宿主机的 permdir 的路径
etcd: # etcd 集群的地址
    - "http://127.0.0.1:2379"
etcd_lock_prefix: "/eru-core/_lock" # etcd 分布式锁的前缀, 一般会用隐藏文件夹
zone: "c1" # 机房区域

git:
    public_key: "[path_to_pub_key]" # git clone 使用的公钥
    private_key: "[path_to_pri_key]" # git clone 使用的私钥
    gitlab_token: "[token]" # gitlab API token

docker:
    log_driver: "json-file" # 日志驱动, 线上会需要用 none
    network_mode: "bridge" # 默认网络模式, 用 bridge
    cert_path: "[cert_file_dir]" # docker tls 证书目录
    hub: "hub.ricebook.net" # docker hub 地址
    hub_prefix: "namespace/test" # 存放镜像的命名空间, 两边的/会被去掉, 中间的会保留. 镜像名字会是$hub/$hub_prefix/appname:version

scheduler:
    lock_key: "_scheduler_lock" # scheduler 用的锁的 key, 会在 etcd_lock_prefix 里面
    lock_ttl: 10 # scheduler 超时时间
    type: "complex" # scheduler 类型, complex 或者 simple
```

* start eru core

```shell
$ core --config /path/to/core.yaml --log-level debug
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
