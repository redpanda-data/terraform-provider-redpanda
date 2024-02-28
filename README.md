# Redpanda Terraform Provider

!!! THIS IS AN ALPHA RELEASE !!!

Please be aware that all features are subject to change and may not be fully supported at this time.

The Redpanda terraform provider is a [Terraform](https://www.terraform.io/) plugin that allows you to create
and manage resources on [Redpanda Cloud.](https://redpanda.com/redpanda-cloud)

## Getting started

To add the Redpanda provider:

```hcl
terraform {
  required_providers {
    redpanda = {
      source = "redpanda-data/redpanda"
    }
  }
}
```

### Authentication

Client credentials for authentication can be provided as:

**Static credentials:**

```terraform
provider "redpanda" {
  client_id     = "<CLIENT_ID>"
  client_secret = "<CLIENT_SECRET>"
}
```

or

**Environment variables:**

```
REDPANDA_CLIENT_ID=<CLIENT_ID>
REDPANDA_CLIENT_SECRET=<CLIENT_SECRET>
```

## Developing the provider

### Requirements

- [Go](https://go.dev/)
- [Terraform](https://www.terraform.io/)

### Building the provider

After building the provider (`go build`), you may override the plugin with your
locally built provider.

Follow [Terraform documentation](https://developer.hashicorp.com/terraform/cli/config/config-file#development-overrides-for-provider-developers)
on dev overrides for provider developers.

### Running Acceptance Test

The following environment variables are required to run the acceptance tests:

```yaml
# For acceptance test
TF_ACC=true

  # For long-running cluster tests
RUN_CLUSTER_TESTS=true

  # For datasource tests, against existing cluster
TEST_AGAINST_EXISTING_CLUSTER=true
CLUSTER_ID=<CLUSTER_ID>
```

### Releasing a Version

Do not change the Terraform Registry Manifest version! This is the version of the protocol, not the provider

## Generating Docs

To generate docs run the following commands

```shell
go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@latest
tfplugindocs
```

## Support

To raise issues, questions, or interact with the community:

- [Github Issues ](https://github.com/redpanda-data/terraform-provider-redpanda/issues)
- [Slack](https://redpanda.com/slack) 
