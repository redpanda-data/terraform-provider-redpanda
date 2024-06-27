---
page_title: "redpanda_resource_group Resource - terraform-provider-redpanda"
subcategory: ""
description: |-
  A Redpanda Cloud resource group
---

# redpanda_resource_group (Resource)

A Redpanda Cloud resource group

Creates a Resource Group in the Redpanda Cloud.

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `name` (String) Name of the resource group. Changing the name of a resource group will result in a new resource group being created and the old one being destroyed

### Read-Only

- `id` (String) UUID of the resource group

## Usage

```terraform
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = var.resource_group_name
}

variable "resource_group_name" {
  default = "testname"
}
```

## Import

```shell
terraform import resource.redpanda_resource_group.example resourcegroupId
```