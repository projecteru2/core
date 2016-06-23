Core
====

## setup dev environment

```
$ git config --global url."git@gitlab.ricebook.net:".insteadOf "https://gitlab.ricebook.net/"
$ go get gitlab.ricebook.net/platform/core.git
$ mv $GOPATH/src/gitlab.ricebook.net/platform/core.git $GOPATH/src/gitlab.ricebook.net/platform/core
$ cd $GOPATH/src/gitlab.ricebook.net/platform/core && go install
$ ln -s $GOPATH/src/gitlab.ricebook.net/platform/core $MY_WORK_SPACE/eru-core2
$ make deps
```

### GRPC

Generate golang & python code

```
$ make golang
$ make python
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
$ core --config /path/to/core.yaml --log-level debug
```

or

```
$ export ERU_CONFIG_PATH=/path/to/core.yaml
$ export ERU_LOG_LEVEL=DEBUG
$ core
```

## TODO

- [ ] more complicated scheduler
- [ ] networks, either use eru-agent or use docker plugin, the latter one is preferred
