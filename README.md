Core
====

## setup dev environment

```shell
git config --global url."git@gitlab.ricebook.net:".insteadOf "https://gitlab.ricebook.net/"
go get gitlab.ricebook.net/platform/core.git
mv $GOPATH/src/gitlab.ricebook.net/platform/core.git $GOPATH/src/gitlab.ricebook.net/platform/core
cd $GOPATH/src/gitlab.ricebook.net/platform/core && go install
ln -s $GOPATH/src/gitlab.ricebook.net/platform/core $MY_WORK_SPACE/eru-core2
```

### GRPC

Generate golang & python code

```shell
cd rpc/gen
protoc --go_out=plugins=grpc:. core.proto
protoc -I . --python_out=. --grpc_out=. --plugin=protoc-gen-grpc=`which grpc_python_plugin` core.proto
```

### deploy core on local environment

* create `core.yaml` like this

```
bind: ":5000"
agent_port: "12345"
permdir: "/mnt/mfs/permdirs"
etcd:
    - "http://127.0.0.1:2379"

git:
    public_key: "[path_to_pub_key]"
    private_key: "[path_to_pri_key]"

docker:
    log_driver: "json-file"
    network_mode: "bridge"
    cert_path: "[cert_file_dir]"
    hub: "hub.ricebook.net"
```

* start eru core

```
core --config core.yaml
```
