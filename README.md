# Redpanda Terraform Provider

The Redpanda Terraform Provider is a [Terraform](https://www.terraform.io/) plugin that allows you to create and manage
resources on [Redpanda Cloud](https://redpanda.com/redpanda-cloud).

## Table of Contents

- [Getting Started](#getting-started)
- [Contributing](#contributing)
    - [Pull Request Process](#pull-request-process)
- [Makefile Commands - Developer Guide](#makefile-commands---developer-guide)
    - [Prerequisites](#prerequisites)
    - [Cluster Management Commands](#cluster-management-commands)
    - [Development Commands](#development-commands)
    - [Best Practices](#best-practices)
- [Contributing](#contributing)
- [Releasing a Version](#releasing-a-version)
- [Support](#support)

## Getting Started

User documentation on the Terraform provider is available at
the [Terraform Registry](https://registry.terraform.io/providers/redpanda-data/redpanda/latest/docs).

## Contributing

When contributing to this project, please ensure you've run `make ready` and all tests pass before submitting a pull
request. If you've added new functionality, consider adding appropriate unit and integration tests.

### Pull Request Process

* (optional) Use the label docs to generate documentation
* Use the label ci-ready to run integration tests

## Makefile Commands - Developer Guide

This guide provides an overview of the key Makefile commands used in the development and testing of the Redpanda
Terraform Provider. These commands help streamline the development process, manage Redpanda clusters for testing, and
ensure code quality.

### Prerequisites

Before using these commands, ensure you have the following:

- Go installed on your system
- Terraform CLI installed
- Access to a Redpanda Cloud account with appropriate permissions
- Required environment variables set (REDPANDA_CLIENT_ID, REDPANDA_CLIENT_SECRET)

### Cluster Management Commands

These commands are used to create and manage Redpanda clusters for testing purposes.

#### apply

Creates and sets up a Redpanda cluster using Terraform. This is intended for use in manual testing and development. It
should not be active when running the integration tests or you will lose the cluster ID and name.

Here's an example usage

```shell
# Test type defaults to cluster
# Cloud provider defaults to aws
make apply 
# make changes to examples/cluster/aws/main.tf
# rerun apply to review
make apply

# switch to datasource to test accessing the cluster with datasource and creating resources
# this is convenient for manual testing of changes to dataplane resources
# make changes in examples/datasource/standard/main.tf
export TEST_TYPE=datasource
make apply

# switch to GCP to validate cluster against GCP. 
# Note that you won't lose your AWS state or cluster when doing this
export TEST_TYPE=cluster
export CLOUD_PROVIDER=gcp

make apply

# clean up by tearing down the GCP cluster
make teardown

# switch back to AWS and cleanup
export CLOUD_PROVIDER=aws
make teardown
```

Command: `make apply`

**Key Variables:**

- `REDPANDA_CLIENT_ID`: Redpanda Cloud client ID
- `REDPANDA_CLIENT_SECRET`: Redpanda Cloud client secret
- `REDPANDA_CLOUD_ENVIRONMENT`: Redpanda Cloud environment (ign or prod)
- `TF_CONFIG_DIR`: Terraform configuration directory (auto-generated)
- `CLOUD_PROVIDER`: Cloud provider (e.g., aws, azure, gcp)
- `TEST_TYPE`: Type of test (e.g., byoc, cluster, datasource)

The `TF_CONFIG_DIR` is dynamically constructed based on the `TEST_TYPE` and `CLOUD_PROVIDER`:

For byoc and cluster tests: `TF_CONFIG_DIR := examples/$(TEST_TYPE)/$(CLOUD_PROVIDER)`
For datasource tests: `TF_CONFIG_DIR := examples/$(TEST_TYPE)/$(DATASOURCE_TEST_DIR)`

This is done to enable persisting the name and id of a cluster created by apply while still allowing for name
randomization. Names and IDs are persisted by cloud provider, so you can switch between providers without losing them.
You can also switch from the cluster test to the datasource test and the correct cluster will be reused depending on the
cloud provider you have set.

#### teardown

Destroys the current Redpanda cluster and associated infrastructure managed by Terraform.

Command: `make teardown`

This command uses the same `TF_CONFIG_DIR` as the `apply` command to ensure it targets the correct resources.

### Development Commands

These commands assist in code development, testing, and maintenance.

#### ready

Prepares the project by generating documentation, running linters, and tidying dependencies.

Command: `make ready`

This command is useful to run before committing changes to ensure code quality and up-to-date documentation.

#### unit

Runs unit tests for the project.

Command: `make unit`

**Note:** This command uses dummy credentials and does not run cluster tests.

#### int

Runs integration tests for the project.

Command: `make int`

**Important:** This command requires valid Redpanda credentials and will create actual resources in your Redpanda Cloud
account.

#### mock

Cleans and regenerates mock files used in testing.

Command: `make mock`

Mocks are generated using mockgen from specific interfaces as defined in redpanda/mocks/mocks.go. Once you have tagged
them with go generate, you can run this command to generate the mocks.

### Best Practices

1. Always run `make ready` before committing changes to ensure code quality and documentation accuracy.
2. Use `make unit` for quick, local testing that doesn't require Redpanda credentials.
3. Use `apply` and `teardown` for more complex manual testing during development
4. Run the integration tests by tagging your PR with `ci-ready` to ensure all tests pass before merging.

## Releasing a Version

Do not change the Terraform Registry Manifest version! This is the version of the protocol, not the provider. To release
a version cut a release in GitHub. Goreleaser will handle things from there.

## Support

To raise issues, questions, or interact with the community:

- [Github Issues](https://github.com/redpanda-data/terraform-provider-redpanda/issues)
- [Slack](https://redpanda.com/slack)