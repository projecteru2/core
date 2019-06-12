Eru
====
[![CircleCI](https://circleci.com/gh/projecteru2/core/tree/master.svg?style=shield)](https://circleci.com/gh/projecteru2/core/tree/master)
[![Codacy Badge](https://api.codacy.com/project/badge/Grade/e26ca3ee697d406caa9e49b0c491ff13)](https://www.codacy.com/app/CMGS/core?utm_source=github.com&amp;utm_medium=referral&amp;utm_content=projecteru2/core&amp;utm_campaign=Badge_Grade)

Eru is a stateless, flexible, production-ready cluster scheduler designed to easily integrate into existing workflows. Eru can run any containerized things in long or short time. This project is Eru Core. The Core use for resource allocation and manage containers lifetime.

### Testing

Run ` make test `

### Compile

* Run ` make build ` if you want binary.
* Run `./make-rpm ` if you want RPM for el7. However we use [FPM](https://github.com/jordansissel/fpm) for packing, so you have to prepare it first.

### Developing

Run `make deps` for generating vendor dir.

Under macOS we have to install `libgit2` manually, if you using [Homebrew](https://brew.sh/) please install like this:

```shell
# need to specify libgit2 version 0.27.x because git2go not supported lastest 0.28: https://github.com/libgit2/git2go/issues/502
HOMEBREW_NO_AUTO_UPDATE=1 brew install https://raw.githubusercontent.com/Homebrew/homebrew-core/19dee2bd48ec0443e1f4a90bbdae0b9b8ef7da98/Formula/libgit2.rb
make deps
```

In linux you can reference our image's [Dockerfile](https://github.com/projecteru2/core/blob/master/Dockerfile). Our server were running under CentOS 7, so if your server was different, something will not same.

On other hand, you can use our [footstone](https://hub.docker.com/r/projecteru2/footstone/) image for testing and compiling.

#### GRPC

Generate golang & python 3 code

```shell
go get -u github.com/golang/protobuf/{proto,protoc-gen-go}
pip install -U grpcio-tools
make grpc
```

#### Run it

```shell
$ eru-core --config /etc/eru/core.yaml.sample
```

or

```shell
$ export ERU_CONFIG_PATH=/path/to/core.yaml
$ eru-core
```

### Dockerized Core manually

Image: [projecteru2/core](https://hub.docker.com/r/projecteru2/core/)

```shell
docker run -d \
  --name eru_core_$HOSTNAME \
  --net host \
  --restart always \
  -v <HOST_CONFIG_DIR_PATH>:/etc/eru \
  projecteru2/core \
  /usr/bin/eru-core
```

### Build and Deploy by Eru itself

After we implemented bootstrap in eru2, now you can build and deploy eru with [cli](https://github.com/projecteru2/cli) tool.

1. Test source code and build image

```shell
<cli_execute_path> --name <image_name> https://goo.gl/KTGJ9k
```

Make sure you can clone code. After the fresh image was named and tagged, it will be auto pushed to the remote registry which was defined in config file.

2. Deploy core itself

```shell
<cli_execute_path> container deploy --pod <pod_name> [--node <node_name>] --entry core --network <network_name> --image <projecteru2/core>|<your_own_image> --file <core_config_yaml>:/core.yaml [--count <count_num>] [--cpu 0.3 | --mem 1024000000] https://goo.gl/KTGJ9k
```

Now you will find core was started in nodes.
