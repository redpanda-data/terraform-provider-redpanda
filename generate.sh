#!/bin/bash
go install github.com/bufbuild/buf/cmd/buf@latest
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
rm -rf proto/gen
buf generate buf.build/redpandadata/cloud
buf generate buf.build/redpandadata/dataplane
buf generate buf.build/redpandadata/common
