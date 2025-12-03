# Redpanda Terraform Provider

The Redpanda Terraform Provider is a [Terraform](https://www.terraform.io/) plugin that allows you to create and manage
resources on [Redpanda Cloud](https://redpanda.com/redpanda-cloud).

## Table of Contents

- [Getting Started](#getting-started)
- [Contributing](#contributing)
    - [Pull Request Process](#pull-request-process)
- [Development Guide](#development-guide)
    - [Prerequisites](#prerequisites)
    - [Task Commands Overview](#task-commands-overview)
    - [Local Development & Testing](#local-development--testing)
    - [Cluster Management Commands](#cluster-management-commands)
    - [Development Commands](#development-commands)
    - [Release Commands](#release-commands)
    - [Best Practices](#best-practices)
- [Contributing](#contributing)
- [Releasing a Version](#releasing-a-version)
- [Support](#support)

## Getting Started

User documentation on the Terraform provider is available at
the [Terraform Registry](https://registry.terraform.io/providers/redpanda-data/redpanda/latest/docs).

## Contributing

When contributing to this project, please ensure you've run `task ready` and all tests pass before submitting a pull
request. If you've added new functionality, consider adding appropriate unit and integration tests.

### Pull Request Process

* (optional) Use the label docs to generate documentation
* Use the label ci-ready to run integration tests

## Development Guide

This guide provides an overview of the development workflow using [Task](https://taskfile.dev/) for building, testing, and managing the Redpanda Terraform Provider. Task replaces our previous Makefile-based workflow with a more modern, cross-platform solution.

### Prerequisites

Before using these commands, ensure you have the following:

- [Task](https://taskfile.dev/) installed on your system
- Go installed on your system
- Terraform CLI installed
- Access to a Redpanda Cloud account with appropriate permissions
- Required environment variables set (see [Environment Configuration](#environment-configuration))

#### Environment Configuration

Create a `.env` file in the project root with your configuration:

```bash
# Copy the example file and customize
cp .env.example .env
```

Required variables:
- `REDPANDA_CLIENT_ID`: Redpanda Cloud client ID
- `REDPANDA_CLIENT_SECRET`: Redpanda Cloud client secret
- `REDPANDA_CLOUD_ENVIRONMENT`: Redpanda Cloud environment (pre/prod)

### Task Commands Overview

View all available commands:

```bash
task --list-all
```

The task system is organized into domains:

- **build**: Provider compilation and installation
- **test**: Unit and integration testing
- **docs**: Documentation generation
- **lint**: Code quality and formatting
- **local**: Local development cluster management
- **release**: Release preparation and publishing

### Local Development & Testing

The local task domain provides commands for testing your locally-built provider against live Redpanda Cloud clusters. Note that "local" refers to using a locally-built version of the provider rather than the published version from the Terraform Registry - the clusters themselves are still created in Redpanda Cloud.

#### Local Provider Testing

Test your provider changes against real cloud infrastructure:

```bash
# Create an AWS cluster using your local provider build
task local:cluster:aws:apply

# Create an Azure cluster using your local provider build
task local:cluster:azure:apply

# Create a GCP cluster using your local provider build
task local:cluster:gcp:apply

# Destroy clusters when done
task local:cluster:aws:destroy
task local:cluster:azure:destroy
task local:cluster:gcp:destroy
```

These commands:
- Build the provider locally from your current code
- Install the local build to the specific example directory (not globally)
- Run `terraform init` and `terraform apply/destroy` using your local provider build
- Create real clusters in Redpanda Cloud for testing your changes
- Handle different cluster types automatically (cluster, serverless, data sources)

Example workflow:

```bash
# Build and test AWS cluster with your local provider changes
task local:cluster:aws:apply

# Make changes to your provider code or examples/cluster/aws/main.tf
# Test your changes (rebuilds provider automatically)
task local:cluster:aws:apply

# Clean up the cloud resources when done
task local:cluster:aws:destroy
```

**Important:** These commands create real resources in Redpanda Cloud and may incur costs. Always clean up test clusters when finished.

### Cluster Management Commands

For CI/CD and integration testing, use the test domain commands:

#### Integration Tests

```bash
# Run unit tests (no credentials needed)
task test:unit

# Run network tests (requires credentials)
task test:network

# Run data source tests (requires existing cluster)
task test:datasource

# Run full cluster tests (creates real resources)
task test:cluster:aws
task test:cluster:azure
task test:cluster:gcp

# Run BYOC tests
task test:byoc:aws
task test:byoc:azure
task test:byoc:gcp

# Run serverless tests
task test:serverless
task test:serverless:regions
```

**Important:** Integration tests create real cloud resources and require valid credentials.

### Development Commands

These commands assist in code development, testing, and maintenance.

#### Build Commands

```bash
# Build the provider binary
task build

# Install provider to local Terraform cache
task build:install

# Clean up Go modules
task build:tidy
```

#### Code Quality

```bash
# Prepare code for commit (docs, lint, tidy)
task ready

# Run linting
task lint

# Run linting with auto-fix
task lint:fix

# Install linting tools
task lint:install
```

#### Documentation

```bash
# Generate provider documentation
task docs

# Install documentation tools
task docs:install
```

#### Testing & Mocks

```bash
# Run unit tests (no credentials needed)
task test:unit

# Generate and clean mocks
task mock

# Clean existing mocks
task mock:clean

# Generate mocks from interfaces
task mock:generate
```

The `task ready` command is especially useful before committing changes as it ensures code quality and up-to-date documentation.

### Release Commands

These commands are used for creating and managing releases:

```bash
# Check GoReleaser configuration
task release:check

# Import GPG key for signing
task release:import-gpg

# Create a full release (requires GPG setup)
task release

# Build release artifacts locally
task release:build

# Create a snapshot release for testing
task release:snapshot
```

**Environment Variables for Releases:**
- `GPG_PRIVATE_KEY`: Base64-encoded GPG private key
- `PASSPHRASE`: GPG key passphrase
- `GPG_FINGERPRINT`: GPG key fingerprint
- `GITHUB_TOKEN`: GitHub token for release publishing

### Best Practices

1. Always run `task ready` before committing changes to ensure code quality and documentation accuracy.
2. Use `task test:unit` for quick, local testing that doesn't require Redpanda credentials.
3. Use `task local:cluster:*:apply` and `task local:cluster:*:destroy` for manual testing during development.
4. Run the integration tests by tagging your PR with `ci-ready` to ensure all tests pass before merging.
5. Use `task release:check` to validate GoReleaser configuration before creating releases.
6. Set up your `.env` file with appropriate credentials for your development environment.

## Releasing a Version

Do not change the Terraform Registry Manifest version! This is the version of the protocol, not the provider. To release
a version cut a release in GitHub. Goreleaser will handle things from there.

## Support

To raise issues, questions, or interact with the community:

- [Github Issues](https://github.com/redpanda-data/terraform-provider-redpanda/issues)
- [Slack](https://redpanda.com/slack)
- 