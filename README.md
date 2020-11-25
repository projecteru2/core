Eru
====
![](https://github.com/projecteru2/core/workflows/test/badge.svg)
![](https://github.com/projecteru2/core/workflows/golangci-lint/badge.svg)
[![Codacy Badge](https://app.codacy.com/project/badge/Grade/69918e0a02ae45c5ae7dfc42bad5cfe5)](https://www.codacy.com/gh/projecteru2/core?utm_source=github.com&amp;utm_medium=referral&amp;utm_content=projecteru2/core&amp;utm_campaign=Badge_Grade)

Eru is a stateless, flexible, production-ready resource scheduler designed to easily integrate into existing systems. 

Eru can use multiple engines to run anything for the long or short term. 

This project is Eru Core. The Core use for resource allocation and manage resource's lifetime.

### Testing

Run ` make test `

### Compile

* Run ` make build ` if you want binary.
* Run `./make-rpm ` if you want RPM for el7. However we use [FPM](https://github.com/jordansissel/fpm) for packing, so you have to prepare it first.

### Developing

Run `make deps` for generating vendor dir.

You can use our [footstone](https://hub.docker.com/r/projecteru2/footstone/) image for testing and compiling.

#### GRPC

Generate golang grpc definitions.

```shell
go get -u github.com/golang/protobuf/{proto,protoc-gen-go}
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

After we implemented bootstrap in eru, now you can build and deploy eru with [cli](https://github.com/projecteru2/cli) tool.

1. Test source code and build image

```shell
<cli_execute_path> --name <image_name> http://bit.ly/EruCore
```

Make sure you can clone code. After the fresh image was named and tagged, it will be auto pushed to the remote registry which was defined in config file.

2. Deploy core itself

```shell
<cli_execute_path> workloads deploy --pod <pod_name> [--node <node_name>] --entry core --network <network_name> --image <projecteru2/core>|<your_own_image> --file <core_config_yaml>:/core.yaml [--count <count_num>] [--cpu 0.3 | --mem 1024000000] http://bit.ly/EruCore
```

Now you will find core was started in nodes.
