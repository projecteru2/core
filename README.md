Core
====

## setup dev environment

```shell
$ git config --global url."git@gitlab.ricebook.net:".insteadOf "https://gitlab.ricebook.net/"
$ go get gitlab.ricebook.net/platform/core.git
$ mv $GOPATH/src/gitlab.ricebook.net/platform/core.git $GOPATH/src/gitlab.ricebook.net/platform/core
$ cd $GOPATH/src/gitlab.ricebook.net/platform/core && go install
$ ln -s $GOPATH/src/gitlab.ricebook.net/platform/core $MY_WORK_SPACE/eru-core2
$ make deps
```

### GRPC

Generate golang & python code

```shell
$ make golang
$ make python
```

### deploy core on local environment

* create `core.yaml` like this

```yaml
bind: ":5000" # gRPC server 监听地址
agent_port: "12345" # agent 的 HTTP API 端口, 暂时没有用到
permdir: "/mnt/mfs/permdirs" # 宿主机的 permdir 的路径
etcd: # etcd 集群的地址
    - "http://127.0.0.1:2379"
etcd_lock_prefix: "/eru-core/_lock" # etcd 分布式锁的前缀, 一般会用隐藏文件夹

git:
    public_key: "[path_to_pub_key]" # git clone 使用的公钥
    private_key: "[path_to_pri_key]" # git clone 使用的私钥
    gitlab_token: "[token]" # gitlab API token

docker:
    log_driver: "json-file" # 日志驱动, 线上会需要用 none
    network_mode: "bridge" # 默认网络模式, 用 bridge
    cert_path: "[cert_file_dir]" # docker tls 证书目录
    hub: "hub.ricebook.net" # docker hub 地址

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

## TODO

- [ ] more complicated scheduler
- [ ] networks, either use eru-agent or use docker plugin, the latter one is preferred
