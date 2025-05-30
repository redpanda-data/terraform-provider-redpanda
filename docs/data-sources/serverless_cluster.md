---
page_title: "redpanda_serverless_cluster Data Source - terraform-provider-redpanda"
subcategory: ""
description: |-
  Data source for a Redpanda Cloud serverless cluster
---

# redpanda_serverless_cluster (Data Source)

Data source for a Redpanda Cloud serverless cluster

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `id` (String) The ID of the serverless cluster

### Read-Only

- `cluster_api_url` (String) The URL of the cluster API
- `name` (String) Name of the serverless cluster
- `resource_group_id` (String) The ID of the resource group in which to create the serverless cluster
- `serverless_region` (String) Redpanda specific region for the serverless cluster

## Usage

```hcl
data "redpanda_serverless_cluster" "example" {
    id = "serverless_cluster_id"
}
```

### Example Usage to create a serverless cluster

```terraform
provider "redpanda" {
}
resource "redpanda_resource_group" "test" {
  name = var.resource_group_name
}

resource "redpanda_serverless_cluster" "test" {
  name              = var.cluster_name
  resource_group_id = redpanda_resource_group.test.id
  serverless_region = var.region
}

variable "resource_group_name" {
  default = "testgroup"
}

variable "cluster_name" {
  default = "testname"
}

variable "region" {
  default = "eu-west-1"
}
```

## Limitations

Can only be used with Redpanda Cloud serverless clusters.