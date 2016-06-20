Core
====

## INSTALL
```
to generate golang code

$ cd rpc/gen
$ protoc --go_out=plugins=grpc:. core.proto

to generate python code

$ cd rpc/gen
$ protoc -I . --python_out=. --grpc_out=. --plugin=protoc-gen-grpc=`which grpc_python_plugin` core.proto
```
