.PHONY: all proto doc int unit test lint buf_deps proto_clean generate_proto tfplugindocs_install generate_docs integration_tests unit_tests install_gofumpt install_lint

GOBIN := $(PWD)/tools
TFPLUGINDOCSCMD := $(GOBIN)/tfplugindocs
GOCMD=go
BUFCMD=buf
GOFUMPTCMD=gofumpt
GOLANGCILINTCMD=golangci-lint

all: proto doc lint test

proto: proto_clean buf_deps generate_proto

doc: tfplugindocs_install generate_docs

int: integration_tests

unit: unit_tests

test: unit_tests integration_tests

lint: install_gofumpt install_lint lint

buf_deps:
	@echo "installing dependencies..."
	@$(GOCMD) install github.com/bufbuild/buf/cmd/buf@latest
	@$(GOCMD) install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@$(GOCMD) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

proto_clean:
	@echo "cleaning proto/gen directory..."
	@rm -rf proto/gen

generate_proto: proto_clean buf_deps
	@echo "generating protobuf files..."
	@$(BUFCMD) generate buf.build/redpandadata/cloud
	@$(BUFCMD) generate buf.build/redpandadata/dataplane
	@$(BUFCMD) generate buf.build/redpandadata/common

tfplugindocs_install:
	@echo "installing tfplugindocs..."
	@GOBIN=$(GOBIN) $(GOCMD) install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@latest

generate_docs: tfplugindocs_install
	@echo "generating provider_documentation..."
	@$(TFPLUGINDOCSCMD)

integration_tests:
	@echo "running integration tests..."
	@DEBUG=true \
	REDPANDA_CLIENT_ID=$${REDPANDA_CLIENT_ID} \
	REDPANDA_CLIENT_SECRET=$${REDPANDA_CLIENT_SECRET} \
	RUN_CLUSTER_TESTS=true \
	TF_ACC=true \
	VERSION=ign \
	$(GOCMD) test -v -parallel=5 ./redpanda/tests

unit_tests:
	@echo "running unit tests..."
	@DEBUG=true \
	REDPANDA_CLIENT_ID="dummy_id" \
	REDPANDA_CLIENT_SECRET="dummy_secret" \
	$(GOCMD) test -v -short ./...

install_gofumpt:
	@echo "installing gofumpt..."
	@$(GOCMD) install mvdan.cc/gofumpt@latest

install_lint:
	@echo "installing linter..."
	@$(GOCMD) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

lint:
	@echo "running gofumpt..."
	@$(GOFUMPTCMD) -w -d .
	@echo "running linter..."
	@$(GOLANGCILINTCMD) run

