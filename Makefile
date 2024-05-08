.PHONY: all doc int unit test lint linter tfplugindocs_install generate_docs integration_tests unit_tests install_gofumpt install_lint

GOBIN := $(PWD)/tools
TFPLUGINDOCSCMD := $(GOBIN)/tfplugindocs
GOCMD=go
BUFCMD=buf
GOFUMPTCMD=gofumpt
GOLANGCILINTCMD=golangci-lint

all: doc lint test

doc: tfplugindocs_install generate_docs

int: integration_tests

unit: unit_tests

test: unit_tests integration_tests

lint: install_gofumpt install_lint linter

ready: doc lint tidy

tidy:
	@echo "running go mod tidy..."
	@$(GOCMD) mod tidy

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
	$(GOCMD) test -v -parallel=5 -timeout=0 ./redpanda/tests

unit_tests:
	@echo "running unit tests..."
	@DEBUG=true \
	REDPANDA_CLIENT_ID="dummy_id" \
	REDPANDA_CLIENT_SECRET="dummy_secret" \
	TF_ACC=false \
	RUN_CLUSTER_TESTS=false \
	$(GOCMD) test -short ./...

install_gofumpt:
	@echo "installing gofumpt..."
	@$(GOCMD) install mvdan.cc/gofumpt@v0.6.0

install_lint:
	@echo "installing linter..."
	@$(GOCMD) install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.55.2

linter:
	@echo "running gofumpt..."
	@$(GOFUMPTCMD) -w -d .
	@echo "running linter..."
	@$(GOLANGCILINTCMD) run

