provider "redpanda" {}

locals {
  # Derived principal for role-based ACLs
  role_principal = "RedpandaRole:${redpanda_role.developer.name}"
}

# --- Infrastructure ---

resource "redpanda_resource_group" "example" {
  name = var.resource_group_name
}

resource "redpanda_network" "example" {
  name              = var.network_name
  resource_group_id = redpanda_resource_group.example.id
  cloud_provider    = var.cloud_provider
  region            = var.region
  cluster_type      = "dedicated"
  cidr_block        = "10.0.0.0/20"

  timeouts = {
    create = "20m"
    delete = "20m"
  }
}

resource "redpanda_cluster" "example" {
  name              = var.cluster_name
  resource_group_id = redpanda_resource_group.example.id
  network_id        = redpanda_network.example.id
  cloud_provider    = var.cloud_provider
  region            = var.region
  cluster_type      = "dedicated"
  connection_type   = "public"
  throughput_tier   = var.throughput_tier
  zones             = var.zones
  allow_deletion    = true

  timeouts = {
    create = "90m"
  }
}

# --- User & Topic ---

resource "random_password" "user" {
  length  = 24
  special = false
}

resource "redpanda_user" "example" {
  name                = var.user_name
  password_wo         = random_password.user.result
  password_wo_version = 1
  mechanism           = "scram-sha-256"
  cluster_api_url     = redpanda_cluster.example.cluster_api_url
  allow_deletion      = true
}

resource "redpanda_topic" "example" {
  name               = var.topic_name
  partition_count    = 3
  replication_factor = 3
  cluster_api_url    = redpanda_cluster.example.cluster_api_url
  allow_deletion     = true

  depends_on = [redpanda_user.example]
}

# --- Role ---

resource "redpanda_role" "developer" {
  name            = var.role_name
  cluster_api_url = redpanda_cluster.example.cluster_api_url
  allow_deletion  = true
}

# --- Role Assignments ---

# Assign the role to the Terraform-managed user
resource "redpanda_role_assignment" "created_user" {
  role_name       = redpanda_role.developer.name
  principal       = redpanda_user.example.name
  cluster_api_url = redpanda_cluster.example.cluster_api_url

  depends_on = [redpanda_user.example]
}

# Assign the role to an existing OIDC user (not managed by Terraform).
# This principal comes from your identity provider (e.g. SSO email).
resource "redpanda_role_assignment" "existing_user" {
  role_name       = redpanda_role.developer.name
  principal       = var.existing_user_principal
  cluster_api_url = redpanda_cluster.example.cluster_api_url
}

# --- ACLs for the Role ---

# Grant READ on the topic to the role
resource "redpanda_acl" "role_topic_read" {
  resource_type         = "TOPIC"
  resource_name         = redpanda_topic.example.name
  resource_pattern_type = "LITERAL"
  principal             = local.role_principal
  host                  = "*"
  operation             = "READ"
  permission_type       = "ALLOW"
  cluster_api_url       = redpanda_cluster.example.cluster_api_url
  allow_deletion        = true
}

# Grant READ on consumer group to the role (so it can actually consume)
resource "redpanda_acl" "role_group_read" {
  resource_type         = "GROUP"
  resource_name         = "*"
  resource_pattern_type = "LITERAL"
  principal             = local.role_principal
  host                  = "*"
  operation             = "READ"
  permission_type       = "ALLOW"
  cluster_api_url       = redpanda_cluster.example.cluster_api_url
  allow_deletion        = true
}

# --- Outputs ---

output "cluster_api_url" {
  description = "The cluster API URL"
  value       = redpanda_cluster.example.cluster_api_url
}

output "role_name" {
  description = "The name of the created role"
  value       = redpanda_role.developer.name
}

output "user_name" {
  description = "The name of the created user"
  value       = redpanda_user.example.name
}
