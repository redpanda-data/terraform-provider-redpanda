---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "redpanda_regions Data Source - terraform-provider-redpanda"
subcategory: ""
description: |-
  Data source for a list of Redpanda Cloud regions
---

# redpanda_regions (Data Source)

Data source for a list of Redpanda Cloud regions



<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `cloud_provider` (String) Cloud provider where the regions exist

### Read-Only

- `regions` (Attributes List) Regions available for the cloud provider (see [below for nested schema](#nestedatt--regions))

<a id="nestedatt--regions"></a>
### Nested Schema for `regions`

Read-Only:

- `name` (String) Name of the region
- `zones` (List of String) Zones available in the region
