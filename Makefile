.PHONY: all doc int unit test lint linter tfplugindocs_install generate_docs integration_tests unit_tests install_gofumpt install_lint build

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
	TF_LOG=DEBUG \
	VERSION=ign \
	$(GOCMD) test -v -parallel=5 -timeout=0 ./redpanda/tests

bulk_tests_data:
	@echo "running bulk tests..."
	@DEBUG=true \
	REDPANDA_CLIENT_ID=$${REDPANDA_CLIENT_ID} \
	REDPANDA_CLIENT_SECRET=$${REDPANDA_CLIENT_SECRET} \
	BULK_CLUSTER_ID=$${BULK_CLUSTER_ID} \
	RUN_BULK_TESTS=true \
	TF_ACC=true \
	VERSION=ign \
	$(GOCMD) test -v -parallel=5 -timeout=0 -run TestAccResourcesBulkData ./redpanda/tests

bulk_tests_res:
	@echo "running bulk tests..."
	@DEBUG=true \
	REDPANDA_CLIENT_ID=$${REDPANDA_CLIENT_ID} \
	REDPANDA_CLIENT_SECRET=$${REDPANDA_CLIENT_SECRET} \
	RUN_BULK_TESTS=true \
	TF_ACC=true \
	VERSION=ign \
	$(GOCMD) test -v -parallel=5 -timeout=0 -run TestAccResourcesBulkRes ./redpanda/tests

unit_tests:
	@echo "running unit tests..."
	@DEBUG=true \
	REDPANDA_CLIENT_ID="dummy_id" \
	REDPANDA_CLIENT_SECRET="dummy_secret" \
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
	@$(GOLANGCILINTCMD) run --config=.golangci.yml


# Allow overriding these variables from the environment
OS ?= $(shell go env GOOS)
ARCH ?= $(shell go env GOARCH)
PROVIDER_VERSION ?= 0.5.2
PROVIDER_NAME ?= redpanda
PROVIDER_NAMESPACE ?= redpanda-data
CONTENT_ROOT ?= $(PWD)
PROVIDER_DIR := $(CONTENT_ROOT)/.terraform.d/plugins/registry.terraform.io/$(PROVIDER_NAMESPACE)/$(PROVIDER_NAME)/$(PROVIDER_VERSION)/$(OS)_$(ARCH)

# Path to the built provider binary
PROVIDER_BINARY := $(PWD)/terraform-provider-$(PROVIDER_NAME)

build:
	@echo "building terraform provider..."
	@$(GOCMD) build -o $(PROVIDER_BINARY)

.PHONY: move-provider
move-provider: build
	@echo "moving provider binary to content root..."
	@mkdir -p $(PROVIDER_DIR)
	@cp $(PROVIDER_BINARY) $(PROVIDER_DIR)/terraform-provider-$(PROVIDER_NAME)_v$(PROVIDER_VERSION)

.PHONY: test-actual
test-actual: build test-create test-destroy

TF_CONFIG_DIR ?= examples/bulk-res
.PHONY: test-create
test-create:
	@echo "Applying Terraform configuration..."
	@cd $(TF_CONFIG_DIR) && \
	REDPANDA_CLIENT_ID="$${REDPANDA_CLIENT_ID}" \
	REDPANDA_CLIENT_SECRET="$${REDPANDA_CLIENT_SECRET}" \
	REDPANDA_CLOUD_ENVIRONMENT="$${REDPANDA_CLOUD_ENVIRONMENT}" \
	TF_LOG=DEBUG \
	TF_INSECURE_SKIP_PROVIDER_VERIFICATION=true \
	TF_PLUGIN_DIR=$(PROVIDER_DIR)
	terraform init && \
	terraform apply -parallelism 10 -auto-approve

.PHONY: test-destroy
test-destroy:
	@echo "Destroying Terraform configuration..."
	@cd $(TF_CONFIG_DIR) && \
	REDPANDA_CLIENT_ID="$${REDPANDA_CLIENT_ID}" && \
    TF_LOG=DEBUG && \
	terraform init && \
	terraform destroy -auto-approve