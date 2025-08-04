---
page_title: "redpanda_role_assignment Resource - terraform-provider-redpanda"
subcategory: ""
description: |-
  A role assignment is used for attaching one user to a role.
---

# redpanda_role_assignment (Resource)

Assigns an existing Redpanda role to a principal. Resource ID format: `{role_name}:{principal}`

## Example Usage

```terraform
provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = var.resource_group_name
}

resource "redpanda_network" "test" {
  name              = var.network_name
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = var.cloud_provider
  region            = var.region
  cluster_type      = "dedicated"
  cidr_block        = "10.0.0.0/20"
}

resource "redpanda_cluster" "test" {
  name              = var.cluster_name
  network_id        = redpanda_network.test.id
  cloud_provider    = var.cloud_provider
  region            = var.region
  cluster_type      = "dedicated"
  connection_type   = "public"
  throughput_tier   = var.throughput_tier
  zones             = var.zones
  allow_deletion    = true
  tags = {
    "key" = "value"
  }
}

# Create a user
resource "redpanda_user" "test_user" {
  name            = "test-user"
  password        = "test-password"
  mechanism       = "scram-sha-256"
  cluster_api_url = redpanda_cluster.test.cluster_api_url
}

# Create a role (note: this would need to be created via rpk CLI separately)
# rpk security role create test-role

# Assign the role to the user
resource "redpanda_role_assignment" "test" {
  role_name       = "test-role"
  principal       = redpanda_user.test_user.id
  cluster_api_url = redpanda_cluster.test.cluster_api_url
}

variable "resource_group_name" {
  default = "testname"
}

variable "network_name" {
  default = "testname"
}

variable "cluster_name" {
  default = "testname"
}

variable "region" {
  default = "us-east-2"
}

variable "zones" {
  default = ["use2-az1", "use2-az2", "use2-az3"]
}

variable "cloud_provider" {
  default = "aws"
}

variable "throughput_tier" {
  default = "tier-1-aws-v2-arm"
}
```

## Schema

### Required

- `cluster_api_url` (String) The cluster API URL. Changing this will prevent deletion of the resource on the existing cluster
- `principal` (String) The principal to assign the role to. Specify just the username (e.g., `"john.doe"`)
- `role_name` (String) The name of the role to assign

### Read-Only

- `id` (String) The ID of this resource. Format: `{role_name}:{principal}`

## Import

Role assignments can be imported using the format `role_name:principal`:

```shell
terraform import redpanda_role_assignment.test "test-role:test-user"
```

Note: The `cluster_api_url` must be specified in your Terraform configuration. The import will validate the role assignment exists during the next `terraform plan` or `terraform apply`.

## Notes

- The role must already exist before it can be assigned. Roles are typically created using `rpk security role create` or through the Redpanda Console.
- The principal should be specified as just the username (e.g., `"john.doe"`). The `User:` prefix is not needed and will be automatically stripped if provided.
- Role assignments are atomic operations - you cannot update an existing assignment. To change a role assignment, delete and recreate the resource.
- The resource uses the Redpanda gRPC SecurityService (via console endpoint) for role management operations.