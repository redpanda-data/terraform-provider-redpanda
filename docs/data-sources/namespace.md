---
page_title: "redpanda_namespace Data Source - terraform-provider-redpanda"
subcategory: ""
description: |-
  Data source for a Redpanda Cloud namespace
---

# redpanda_namespace (Data Source)

Data source for a Redpanda Cloud namespace

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `id` (String) UUID of the namespace

### Read-Only

- `name` (String) Name of the namespace

## Usage

```hcl
data "redpanda_namespace" "example" {
    id = "namespace_id"
}
```