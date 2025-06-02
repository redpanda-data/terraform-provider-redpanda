GOBIN := $(PWD)/tools
TFPLUGINDOCSCMD := $(GOBIN)/tfplugindocs
GOCMD=go
BUFCMD=buf
GOFUMPTCMD=gofumpt
GOLANGCILINTCMD=golangci-lint

.PHONY: doc
doc: tfplugindocs_install generate_docs

.PHONY: int
int: all_integration_tests

.PHONY: unit
unit: unit_tests

.PHONY: lint
lint: install_gofumpt install_lint linter

.PHONY: ready
ready: doc lint tidy

# Task to both clean and generate mocks
.PHONY: mock
mock: clean-mocks generate-mocks

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
REDPANDA_CLOUD_ENVIRONMENT ?= "pre"
.PHONY: all_integration_tests
all_integration_tests:
	@echo "running integration tests..."
	@DEBUG=true \
	REDPANDA_CLIENT_ID=$(REDPANDA_CLIENT_ID) \
	REDPANDA_CLIENT_SECRET=$(REDPANDA_CLIENT_SECRET) \
	RUN_CLUSTER_TESTS=true \
	RUN_BYOC_TESTS=false \
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
	RUN_BYOC_TESTS=false \
	$(GOCMD) test -short ./...

.PHONY: install_gofumpt
install_gofumpt:
	@echo "installing gofumpt..."
	@$(GOCMD) install mvdan.cc/gofumpt@v0.7.0

.PHONY: install_lint
install_lint:
	@echo "installing linter..."
	@$(GOCMD) install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.61

.PHONY: linter
linter:
	@if [ "$$BUILDKITE" = "true" ]; then \
		GOFLAGS="-buildvcs=false" $(GOLANGCILINTCMD) run --timeout=5m; \
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
PROVIDER_NAMESPACE ?= hashicorp
PROVIDER_NAME ?= redpanda
CONTENT_ROOT ?= $(PWD)
CLOUD_PROVIDER ?= aws
TEST_TYPE ?= cluster
DATASOURCE_TEST_DIR ?= standard
TF_CONFIG_DIR ?= examples/$(TEST_TYPE)/$(CLOUD_PROVIDER)
PROVIDER_DIR := .terraform.d/plugins/registry.terraform.io/$(PROVIDER_NAMESPACE)/$(PROVIDER_NAME)/$(PROVIDER_VERSION)/$(OS)_$(ARCH)
TF_LOG ?= WARN
# path to the built binary
PROVIDER_BINARY := $(PWD)/terraform-provider-$(PROVIDER_NAME)

.PHONY: build-provider
build-provider:
	@echo "building terraform provider..."
	@$(GOCMD) build -o $(PROVIDER_BINARY)

.PHONY: move-provider
move-provider:
	@echo "moving provider binary to content root..."
	@echo "PROVIDER_DIR: $(PROVIDER_DIR)"
	@mkdir -p $(call GET_TF_CONFIG_DIR)/$(PROVIDER_DIR)
	@cp $(PROVIDER_BINARY) $(call GET_TF_CONFIG_DIR)/$(PROVIDER_DIR)/terraform-provider-$(PROVIDER_NAME)_v$(PROVIDER_VERSION)

.PHONY: apply
apply: build-provider move-provider test-create

.PHONY: teardown
teardown: test-destroy
PREFIX ?= tfrp-local
CLOUD_PROVIDER ?= aws
CLUSTER_INFO_FILE := .cluster_info_$(CLOUD_PROVIDER).json

define GET_OR_CREATE_CLUSTER_INFO
$(shell \
    if [ -f $(CLUSTER_INFO_FILE) ]; then \
      cat $(CLUSTER_INFO_FILE); \
    else \
      CLUSTER_NAME="$(PREFIX)-$$(LC_ALL=C tr -dc 'A-Za-z0-9' < /dev/urandom | head -c 4)"; \
      echo '{"name":"'$$CLUSTER_NAME'","id":""}' | tee $(CLUSTER_INFO_FILE); \
    fi \
)
endef

# Function to determine TF_CONFIG_DIR
define GET_TF_CONFIG_DIR
$(shell \
    if [ "$(TEST_TYPE)" = "cluster" ] || [ "$(TEST_TYPE)" = "byoc" ] || [ "$(TEST_TYPE)" = "byovpc" ]; then \
        echo "examples/$(TEST_TYPE)/$(CLOUD_PROVIDER)"; \
    elif [ "$(TEST_TYPE)" = "datasource" ]; then \
        echo "examples/$(TEST_TYPE)/$(DATASOURCE_TEST_DIR)"; \
    else \
        echo "Error: Invalid TEST_TYPE" >&2; \
        exit 1; \
    fi \
)
endef

define UPDATE_CLUSTER_ID
$(shell \
    CLUSTER_INFO='$(1)' \
    CLUSTER_ID='$(2)' \
    NEW_INFO=$$(echo $$CLUSTER_INFO | jq --arg id "$$CLUSTER_ID" '.id = $$id') \
    echo $$NEW_INFO > $(CLUSTER_INFO_FILE) \
)
endef

define GET_CLUSTER_NAME
$(shell \
    CLUSTER_INFO='$(call GET_OR_CREATE_CLUSTER_INFO)' \
    echo $$CLUSTER_INFO | jq -r '.name' \
)
endef

define GET_CLUSTER_ID
$(shell \
    CLUSTER_INFO='$(call GET_OR_CREATE_CLUSTER_INFO)' \
    echo $$CLUSTER_INFO | jq -r '.id' \
)
endef

.PHONY: test-create
test-create: tf-init tf-apply update-cluster-info

.PHONY: tf-init
tf-init:
	@echo "Initializing Terraform..."
	@cd $(call GET_TF_CONFIG_DIR) && \
	ls -ltrah && \
	rm -f .terraform.lock.hcl && \
	REDPANDA_CLIENT_ID="$(REDPANDA_CLIENT_ID)" \
	REDPANDA_CLIENT_SECRET="$(REDPANDA_CLIENT_SECRET)" \
	REDPANDA_CLOUD_ENVIRONMENT="$(REDPANDA_CLOUD_ENVIRONMENT)" \
	TF_LOG=$(TF_LOG) \
	TF_INSECURE_SKIP_PROVIDER_VERIFICATION=true \
	TF_PLUGIN_CACHE_DIR=.terraform.d/plugins_cache \
	terraform init -plugin-dir=.terraform.d/plugins

.PHONY: tf-apply
tf-apply:
	@echo "Constructing Terraform apply command..."
	@(cd $(call GET_TF_CONFIG_DIR) && \
	CLUSTER_INFO='$(GET_OR_CREATE_CLUSTER_INFO)' \
	CLUSTER_NAME=$$(echo '$(GET_OR_CREATE_CLUSTER_INFO)' | jq -r '.name') \
	REDPANDA_CLIENT_ID="$(REDPANDA_CLIENT_ID)" \
	REDPANDA_CLIENT_SECRET="$(REDPANDA_CLIENT_SECRET)" \
	REDPANDA_CLOUD_ENVIRONMENT="$(REDPANDA_CLOUD_ENVIRONMENT)" \
	GOOGLE_CREDENTIALS_BASE64="$(GOOGLE_CREDENTIALS_BASE64)" \
	CLOUD_PROVIDER="$(CLOUD_PROVIDER)" \
	TEST_TYPE="$(TEST_TYPE)" \
	TF_LOG=$(TF_LOG) \
	TF_INSECURE_SKIP_PROVIDER_VERIFICATION=true \
	TF_PLUGIN_CACHE_DIR=.terraform.d/plugins_cache \
	bash -c 'if grep -q "resource \"redpanda_cluster\"" *.tf; then \
    	if [ "$$CLOUD_PROVIDER" = "gcp" ] && [ "$$TEST_TYPE" = "byovpc" ]; then \
			terraform apply -auto-approve \
				-var="resource_group_name=$$CLUSTER_NAME" \
				-var="network_name=$$CLUSTER_NAME" \
				-var="cluster_name=$$CLUSTER_NAME" \
				-var="google_credentials_base64=$$GOOGLE_CREDENTIALS_BASE64"; \
    	else \
        	terraform apply -auto-approve \
				-var="resource_group_name=$$CLUSTER_NAME" \
				-var="network_name=$$CLUSTER_NAME" \
				-var="cluster_name=$$CLUSTER_NAME"; \
   		 fi; \
	elif grep -q "resource \"redpanda_serverless_cluster\"" *.tf; then \
		terraform apply -auto-approve \
			-var="resource_group_name=$$CLUSTER_NAME" \
			-var="cluster_name=$$CLUSTER_NAME"; \
	elif grep -q "data \"redpanda_cluster\"" *.tf; then \
		CLUSTER_ID=$$(echo "$$CLUSTER_INFO" | jq -r ".id"); \
		terraform apply -auto-approve -var="cluster_id=$$CLUSTER_ID"; \
	elif grep -q "data \"redpanda_network\"" *.tf; then \
		CLUSTER_ID=$$(echo "$$CLUSTER_INFO" | jq -r ".id"); \
		terraform apply -auto-approve -var="network_id=$$NETWORK_ID"; \
	else \
		echo "Error: No supported Redpanda cluster configuration found in Terraform files."; \
		exit 1; \
	fi')

.PHONY: update-cluster-info
update-cluster-info:
	@echo "Updating cluster info..."
	@cd $(call GET_TF_CONFIG_DIR) && \
	CLUSTER_INFO='$(GET_OR_CREATE_CLUSTER_INFO)' && \
	CLUSTER_ID=$$(terraform show -json | jq -r '.values.root_module.resources[] | select(.type == "redpanda_cluster" or .type == "redpanda_serverless_cluster") | .values.id') && \
	if [ -n "$$CLUSTER_ID" ]; then \
		NEW_CLUSTER_INFO=$$(echo "$$CLUSTER_INFO" | jq --arg id "$$CLUSTER_ID" '.id = $$id'); \
		echo "$$NEW_CLUSTER_INFO" > $(CURDIR)/$(CLUSTER_INFO_FILE); \
		echo "Updated cluster info: $$NEW_CLUSTER_INFO"; \
	else \
		echo "No cluster ID found. Cluster info not updated."; \
	fi

.PHONY: test-destroy
test-destroy:
	@echo "Destroying Terraform configuration..."
	@(cd $(TF_CONFIG_DIR) && \
	CLUSTER_INFO='$(call GET_OR_CREATE_CLUSTER_INFO)' \
	CLUSTER_NAME=$$(echo "$$CLUSTER_INFO" | jq -r '.name') \
	CLUSTER_ID=$$(echo "$$CLUSTER_INFO" | jq -r '.id') \
	REDPANDA_CLIENT_ID="$(REDPANDA_CLIENT_ID)" \
	REDPANDA_CLIENT_SECRET="$(REDPANDA_CLIENT_SECRET)" \
	REDPANDA_CLOUD_ENVIRONMENT=""$(REDPANDA_CLOUD_ENVIRONMENT)"" \
	TF_LOG=$(TF_LOG) \
	TF_INSECURE_SKIP_PROVIDER_VERIFICATION=true \
	TF_PLUGIN_CACHE_DIR=.terraform.d/plugins_cache \
	bash -c 'terraform init -plugin-dir=.terraform.d/plugins && \
	if grep -q "resource \"redpanda_cluster\"" *.tf; then \
		if [ "$$CLOUD_PROVIDER" = "gcp" ] && [ "$$TEST_TYPE" = "byovpc" ]; then \
			terraform destroy -auto-approve \
				-var="resource_group_name=$$CLUSTER_NAME" \
				-var="network_name=$$CLUSTER_NAME" \
				-var="cluster_name=$$CLUSTER_NAME" \
				-var="google_credentials_base64=$$GOOGLE_CREDENTIALS_BASE64"; \
		else \
			terraform destroy -auto-approve \
				-var="resource_group_name=$$CLUSTER_NAME" \
				-var="network_name=$$CLUSTER_NAME" \
				-var="cluster_name=$$CLUSTER_NAME"; \
		 fi; \
	elif grep -q "resource \"redpanda_serverless_cluster\"" *.tf; then \
		terraform destroy -auto-approve \
			-var="resource_group_name=$$CLUSTER_NAME" \
			-var="cluster_name=$$CLUSTER_NAME"; \
	elif grep -q "data \"redpanda_cluster\"" *.tf; then \
		terraform destroy -auto-approve \
			-var="cluster_id=$$CLUSTER_ID"; \
	elif grep -q "data \"redpanda_network\"" *.tf; then \
		CLUSTER_ID=$$(echo "$$CLUSTER_INFO" | jq -r ".id"); \
		terraform apply -auto-approve -var="network_id=$$NETWORK_ID"; \
	else \
		echo "Error: No supported Redpanda cluster configuration found in Terraform files."; \
		exit 1; \
	fi')
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

.PHONY: test_network
test_network:
	@echo "Running TestAccResourcesNetwork..."
	@DEBUG=true \
	REDPANDA_CLIENT_ID="$(REDPANDA_CLIENT_ID)" \
	REDPANDA_CLIENT_SECRET="$(REDPANDA_CLIENT_SECRET)" \
	TF_ACC=true \
	TF_LOG=$(TF_LOG) \
	VERSION=pre \
	$(GOCMD) test -v -timeout=1h ./redpanda/tests -run TestAccResourcesNetwork

.PHONY: test_datasource
test_datasource:
	@echo "Running TestAccResourcesWithDataSources..."
	@DEBUG=true \
	REDPANDA_CLIENT_ID="$(REDPANDA_CLIENT_ID)" \
  	REDPANDA_CLIENT_SECRET="$(REDPANDA_CLIENT_SECRET)" \
  	TEST_AGAINST_EXISTING_CLUSTER=true \
  	CLUSTER_ID="$(DATASOURCE_CLUSTER_ID)" \
  	TF_ACC=true \
  	TF_LOG=$(TF_LOG) \
  	VERSION=pre \
	$(GOCMD) test -v -timeout=1h ./redpanda/tests -run TestAccResourcesWithDataSources

TIMEOUT ?= 6h
.PHONY: test_cluster_aws
test_cluster_aws:
	@echo "Running TestAccResourcesClusterAWS..."
	@DEBUG=true \
	REDPANDA_CLIENT_ID="$(REDPANDA_CLIENT_ID)" \
	REDPANDA_CLIENT_SECRET="$(REDPANDA_CLIENT_SECRET)" \
	RUN_CLUSTER_TESTS=true \
	TF_ACC=true \
	TF_LOG=$(TF_LOG) \
	VERSION=pre \
	$(GOCMD) test -v -timeout=$(TIMEOUT) ./redpanda/tests -run TestAccResourcesClusterAWS

.PHONY: test_cluster_azure
test_cluster_azure:
	@echo "Running TestAccResourcesClusterAzure..."
	@DEBUG=true \
	REDPANDA_CLIENT_ID="$(REDPANDA_CLIENT_ID)" \
	REDPANDA_CLIENT_SECRET="$(REDPANDA_CLIENT_SECRET)" \
	RUN_CLUSTER_TESTS=true \
	TF_ACC=true \
	TF_LOG=$(TF_LOG) \
	VERSION=pre \
	$(GOCMD) test -v -timeout=$(TIMEOUT) ./redpanda/tests -run TestAccResourcesClusterAzure

.PHONY: test_cluster_gcp
test_cluster_gcp:
	@echo "Running TestAccResourcesClusterGCP..."
	@DEBUG=true \
	REDPANDA_CLIENT_ID="$(REDPANDA_CLIENT_ID)" \
	REDPANDA_CLIENT_SECRET="$(REDPANDA_CLIENT_SECRET)" \
	RUN_CLUSTER_TESTS=true \
	TF_ACC=true \
	TF_LOG=$(TF_LOG) \
	VERSION=pre \
	$(GOCMD) test -v -timeout=$(TIMEOUT) ./redpanda/tests -run TestAccResourcesClusterGCP

.PHONY: test_byoc_aws
test_byoc_aws:
	@echo "Running TestAccResourcesByocAWS..."
	@DEBUG=true \
	REDPANDA_CLIENT_ID="$(REDPANDA_CLIENT_ID)" \
	REDPANDA_CLIENT_SECRET="$(REDPANDA_CLIENT_SECRET)" \
	RUN_BYOC_TESTS=true \
	TF_ACC=true \
	TF_LOG=$(TF_LOG) \
	VERSION=pre \
	$(GOCMD) test -v -timeout=$(TIMEOUT) ./redpanda/tests -run TestAccResourcesByocAWS

.PHONY: test_byoc_azure
test_byoc_azure:
	@echo "Running TestAccResourcesByocAzure..."
	@DEBUG=true \
	REDPANDA_CLIENT_ID="$(REDPANDA_CLIENT_ID)" \
	REDPANDA_CLIENT_SECRET="$(REDPANDA_CLIENT_SECRET)" \
	RUN_BYOC_TESTS=true \
	TF_ACC=true \
	TF_LOG=$(TF_LOG) \
	VERSION=pre \
	$(GOCMD) test -v -timeout=$(TIMEOUT) ./redpanda/tests -run TestAccResourcesByocAzure

.PHONY: test_byoc_gcp
test_byoc_gcp:
	@echo "Running TestAccResourcesByocGCP..."
	@DEBUG=true \
	REDPANDA_CLIENT_ID="$(REDPANDA_CLIENT_ID)" \
	REDPANDA_CLIENT_SECRET="$(REDPANDA_CLIENT_SECRET)" \
	REDPANDA_VERSION="24.2.20240809182625" \
	RUN_BYOC_TESTS=true \
	TF_ACC=true \
	TF_LOG=$(TF_LOG) \
	VERSION=pre \
	$(GOCMD) test -v -timeout=$(TIMEOUT) ./redpanda/tests -run TestAccResourcesByocGCP

RUN_SERVERLESS_TESTS ?= true
.PHONY: test_serverless_cluster
test_serverless_cluster:
	@echo "Running TestAccResourcesStrippedDownServerlessCluster..."
	@DEBUG=true \
	REDPANDA_CLIENT_ID="$(REDPANDA_CLIENT_ID)" \
	REDPANDA_CLIENT_SECRET="$(REDPANDA_CLIENT_SECRET)" \
	RUN_SERVERLESS_TESTS="$(RUN_SERVERLESS_TESTS)" \
	TF_ACC=true \
	TF_LOG=$(TF_LOG) \
	VERSION=pre \
	$(GOCMD) test -v -timeout=$(TIMEOUT) ./redpanda/tests -run TestAccResourcesStrippedDownServerlessCluster

import-gpg:
	echo "$$GPG_PRIVATE_KEY" | base64 -d | gpg --batch --pinentry-mode loopback --passphrase "$$PASSPHRASE" --import

.PHONY: release
release: import-gpg
	goreleaser release --clean --verbose