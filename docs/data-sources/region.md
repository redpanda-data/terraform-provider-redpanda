---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "redpanda_region Data Source - terraform-provider-redpanda"
subcategory: ""
description: |-
  Data source for a Redpanda Cloud region
---

# redpanda_region (Data Source)

Data source for a Redpanda Cloud region



<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `cloud_provider` (String) Cloud provider where the region exists
- `name` (String) Name of the region

### Read-Only

- `zones` (List of String) Zones available in the region
