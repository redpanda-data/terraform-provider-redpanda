GOBIN := $(PWD)/tools
TFPLUGINDOCSCMD := $(GOBIN)/tfplugindocs
GOCMD=go
BUFCMD=buf
GOFUMPTCMD=gofumpt
GOLANGCILINTCMD=golangci-lint

.PHONY: all
all: doc lint test

.PHONY: doc
doc: tfplugindocs_install generate_docs

.PHONY: int
int: all_integration_tests

.PHONY: unit
unit: unit_tests

.PHONY: test
test: unit_tests all_integration_tests

.PHONY: lint
lint: install_gofumpt install_lint linter

.PHONY: ready
ready: doc lint tidy

.PHONY: tidy
tidy:
	@echo "running go mod tidy..."
	@$(GOCMD) mod tidy

.PHONY: tfplugindocs_install
tfplugindocs_install:
	@echo "installing tfplugindocs..."
	@GOBIN=$(GOBIN) $(GOCMD) install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@latest

.PHONY: generate_docs
generate_docs: tfplugindocs_install
	@echo "generating provider_documentation..."
	@$(TFPLUGINDOCSCMD)

REDPANDA_CLIENT_ID ?= $(or $(INTEGRATION_PROVIDER_SECRET_REDPANDA_CLIENT_ID),$(REDPANDA_CLIENT_ID))
REDPANDA_CLIENT_SECRET ?= $(or $(INTEGRATION_PROVIDER_SECRET_REDPANDA_CLIENT_SECRET),$(REDPANDA_CLIENT_SECRET))

.PHONY: all_integration_tests
all_integration_tests:
	@echo "running integration tests..."
	@DEBUG=true \
	REDPANDA_CLIENT_ID=$(REDPANDA_CLIENT_ID) \
	REDPANDA_CLIENT_SECRET=$(REDPANDA_CLIENT_SECRET) \
	RUN_CLUSTER_TESTS=true \
	TF_ACC=true \
	TF_LOG=DEBUG \
	VERSION=ign \
	$(GOCMD) test -v -parallel=5 -timeout=0 ./redpanda/tests

.PHONY: unit_tests
unit_tests:
	@echo "running unit tests..."
	@DEBUG=true \
	REDPANDA_CLIENT_ID="dummy_id" \
	REDPANDA_CLIENT_SECRET="dummy_secret" \
	RUN_CLUSTER_TESTS=false \
	$(GOCMD) test -short ./...

.PHONY: install_gofumpt
install_gofumpt:
	@echo "installing gofumpt..."
	@$(GOCMD) install mvdan.cc/gofumpt@v0.6.0

.PHONY: install_lint
install_lint:
	@echo "installing linter..."
	@$(GOCMD) install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.55.2

.PHONY: linter
linter:
	@if [ "$$BUILDKITE" = "true" ]; then \
		GOFLAGS="-buildvcs=false" $(GOLANGCILINTCMD) run; \
	else \
		$(GOLANGCILINTCMD) run; \
	fi

.PHONY: fix-lint
fix-lint:
	@echo "running gofumpt..."
	@$(GOLANGCILINTCMD) run --fix

OS ?= $(shell go env GOOS)
ARCH ?= $(shell go env GOARCH)
PROVIDER_VERSION ?= 0.7.1
PROVIDER_NAMESPACE ?= redpanda_data
PROVIDER_NAME ?= redpanda
CONTENT_ROOT ?= $(PWD)
CLOUD_PROVIDER ?= aws
TEST_TYPE ?= cluster
TF_CONFIG_DIR ?= examples/$(TEST_TYPE)/$(CLOUD_PROVIDER)
PROVIDER_DIR := .terraform.d/plugins/registry.terraform.io/$(PROVIDER_NAMESPACE)/$(PROVIDER_NAME)/$(PROVIDER_VERSION)/$(OS)_$(ARCH)

# path to the built binary
PROVIDER_BINARY := $(PWD)/terraform-provider-$(PROVIDER_NAME)

.PHONY: build-provider
build-provider:
	@echo "building terraform provider..."
	@$(GOCMD) build -o $(PROVIDER_BINARY)

BINARY_LOC :=  $(TF_CONFIG_DIR)/$(PROVIDER_DIR)/terraform-provider-$(PROVIDER_NAME)_v$(PROVIDER_VERSION)
.PHONY: move-provider
move-provider:
	@echo "moving provider binary to content root..."
	@echo "PROVIDER_DIR: $(PROVIDER_DIR)"
	@echo "BINARY_LOC: $(BINARY_LOC)"
	@mkdir -p $(TF_CONFIG_DIR)/$(PROVIDER_DIR)
	@cp $(PROVIDER_BINARY) $(BINARY_LOC)

.PHONY: standup
standup: build-provider move-provider test-create

.PHONY: teardown
teardown: test-destroy

PREFIX ?= tfrp-local
TEMP_FILE := .tmp_$(CLOUD_PROVIDER)
RANDOM_STRING := $(shell LC_ALL=C tr -dc 'A-Za-z0-9' < /dev/urandom | head -c 4)
define GET_OR_CREATE_RESOURCE_NAME
$(shell \
    if [ -f $(TEMP_FILE) ]; then \
        cat $(TEMP_FILE); \
    else \
        echo "$(PREFIX)-$(RANDOM_STRING)" | tee $(TEMP_FILE); \
    fi \
)
endef

.PHONY: test-create
test-create:
	@echo "Applying Terraform configuration..."
	@echo "TF_CONFIG_DIR: $(TF_CONFIG_DIR)"
	@cd $(TF_CONFIG_DIR) && \
	REDPANDA_CLIENT_ID="$(REDPANDA_CLIENT_ID)" \
	REDPANDA_CLIENT_SECRET="$(REDPANDA_CLIENT_SECRET)" \
	REDPANDA_CLOUD_ENVIRONMENT="$${REDPANDA_CLOUD_ENVIRONMENT}" \
	TF_LOG=DEBUG \
	TF_INSECURE_SKIP_PROVIDER_VERIFICATION=true \
	TF_PLUGIN_CACHE_DIR=.terraform.d/plugins_cache \
    terraform init -plugin-dir=.terraform.d/plugins && \
	terraform apply -auto-approve -var="resource_group_name=$(call GET_OR_CREATE_RESOURCE_NAME)" -var="network_name=$(call GET_OR_CREATE_RESOURCE_NAME)" -var="cluster_name=$(call GET_OR_CREATE_RESOURCE_NAME)"

.PHONY: test-destroy
test-destroy:
	@echo "Destroying Terraform configuration..."
	@cd $(TF_CONFIG_DIR) && \
	REDPANDA_CLIENT_ID="$${REDPANDA_CLIENT_ID}" \
	REDPANDA_CLIENT_SECRET="$${REDPANDA_CLIENT_SECRET}" \
	REDPANDA_CLOUD_ENVIRONMENT="$${REDPANDA_CLOUD_ENVIRONMENT}" \
    TF_LOG=DEBUG \
	TF_INSECURE_SKIP_PROVIDER_VERIFICATION=true \
	TF_PLUGIN_CACHE_DIR=.terraform.d/plugins_cache \
    terraform init -plugin-dir=.terraform.d/plugins && \
	terraform destroy -auto-approve -var="resource_group_name=$(call GET_OR_CREATE_RESOURCE_NAME)" -var="network_name=$(call GET_OR_CREATE_RESOURCE_NAME)" -var="cluster_name=$(call GET_OR_CREATE_RESOURCE_NAME)"


# Define the directory where the mocks are located
MOCKS_DIR := redpanda/mocks

# Task to generate all mocks
.PHONY: generate-mocks
generate-mocks:
	@echo "Generating mocks..."
	@cd $(MOCKS_DIR) && go generate
	@echo "Mocks generated successfully."

# Task to clean generated mocks
.PHONY: clean-mocks
clean-mocks:
	@echo "Cleaning generated mocks..."
	@rm -f $(MOCKS_DIR)/mock_*.go
	@echo "Mocks cleaned successfully."

# Task to both clean and regenerate mocks
.PHONY: refresh-mocks
refresh-mocks: clean-mocks generate-mocks

.PHONY: test_network
test_network:
	@echo "Running TestAccResourcesNetwork..."
	@DEBUG=true \
	REDPANDA_CLIENT_ID="$(REDPANDA_CLIENT_ID)" \
	REDPANDA_CLIENT_SECRET="$(REDPANDA_CLIENT_SECRET)" \
	TF_ACC=true \
	TF_LOG=DEBUG \
	VERSION=ign \
	$(GOCMD) test -v -timeout=4h ./redpanda/tests -run TestAccResourcesNetwork

.PHONY: test_cluster_aws
test_cluster_aws:
	@echo "Running TestAccResourcesClusterAWS..."
	@DEBUG=true \
	REDPANDA_CLIENT_ID="$(REDPANDA_CLIENT_ID)" \
	REDPANDA_CLIENT_SECRET="$(REDPANDA_CLIENT_SECRET)" \
	RUN_CLUSTER_TESTS=true \
	TF_ACC=true \
	TF_LOG=DEBUG \
	VERSION=ign \
	$(GOCMD) test -v -timeout=4h ./redpanda/tests -run TestAccResourcesClusterAWS

.PHONY: test_cluster_azure
test_cluster_azure:
	@echo "Running TestAccResourcesClusterAzure..."
	@DEBUG=true \
	REDPANDA_CLIENT_ID="$(REDPANDA_CLIENT_ID)" \
	REDPANDA_CLIENT_SECRET="$(REDPANDA_CLIENT_SECRET)" \
	RUN_CLUSTER_TESTS=true \
	TF_ACC=true \
	TF_LOG=DEBUG \
	VERSION=ign \
	$(GOCMD) test -v -timeout=4h ./redpanda/tests -run TestAccResourcesClusterAzure

.PHONY: test_cluster_gcp
test_cluster_gcp:
	@echo "Running TestAccResourcesClusterGCP..."
	@DEBUG=true \
	REDPANDA_CLIENT_ID="$(REDPANDA_CLIENT_ID)" \
	REDPANDA_CLIENT_SECRET="$(REDPANDA_CLIENT_SECRET)" \
	REDPANDA_VERSION="24.2.20240809182625" \
	RUN_CLUSTER_TESTS=true \
	TF_ACC=true \
	TF_LOG=DEBUG \
	VERSION=ign \
	$(GOCMD) test -v -timeout=4h ./redpanda/tests -run TestAccResourcesClusterGCP

.PHONY: test_serverless_cluster
test_serverless_cluster:
	@echo "Running TestAccResourcesStrippedDownServerlessCluster..."
	@DEBUG=true \
	REDPANDA_CLIENT_ID="$(REDPANDA_CLIENT_ID)" \
	REDPANDA_CLIENT_SECRET="$(REDPANDA_CLIENT_SECRET)" \
	RUN_SERVERLESS_TESTS=true \
	TF_ACC=true \
	TF_LOG=DEBUG \
	VERSION=ign \
	$(GOCMD) test -v -timeout=4h ./redpanda/tests -run TestAccResourcesStrippedDownServerlessCluster
