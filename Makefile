golang:
	cd ./rpc/gen/; protoc --go_out=plugins=grpc:. core.proto

python:
	cd ./rpc/gen/; protoc -I . --python_out=. --grpc_out=. --plugin=protoc-gen-grpc=`which grpc_python_plugin` core.proto; mv core_pb2.py ../../devtools/
