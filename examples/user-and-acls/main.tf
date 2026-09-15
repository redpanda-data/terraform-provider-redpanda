provider "redpanda" {}

locals {
  principal = "User:${redpanda_user.example.name}"

  cluster_ops = ["READ", "DESCRIBE"]
  topic_ops   = ["DESCRIBE", "WRITE", "READ"]
  group_ops   = ["READ", "CREATE", "WRITE", "DESCRIBE"]
}

resource "random_password" "user" {
  length  = 24
  special = false
}

resource "redpanda_user" "example" {
  name                = var.user_name
  password_wo         = random_password.user.result
  password_wo_version = 1
  mechanism           = "scram-sha-256"
  cluster_api_url     = var.cluster_api_url
  allow_deletion      = true
}

# Apply a typical consumer-style permission set to the user. `for_each` over
# the operation lists in `locals` keeps the ACL bundle readable as you add or
# remove operations.

resource "redpanda_acl" "cluster" {
  for_each = toset(local.cluster_ops)

  resource_type         = "CLUSTER"
  resource_name         = "kafka-cluster"
  resource_pattern_type = "LITERAL"
  principal             = local.principal
  host                  = "*"
  operation             = each.value
  permission_type       = "ALLOW"
  cluster_api_url       = var.cluster_api_url
  allow_deletion        = true
}

resource "redpanda_acl" "topic" {
  for_each = toset(local.topic_ops)

  resource_type         = "TOPIC"
  resource_name         = "*"
  resource_pattern_type = "LITERAL"
  principal             = local.principal
  host                  = "*"
  operation             = each.value
  permission_type       = "ALLOW"
  cluster_api_url       = var.cluster_api_url
  allow_deletion        = true
}

resource "redpanda_acl" "group" {
  for_each = toset(local.group_ops)

  resource_type         = "GROUP"
  resource_name         = "*"
  resource_pattern_type = "LITERAL"
  principal             = local.principal
  host                  = "*"
  operation             = each.value
  permission_type       = "ALLOW"
  cluster_api_url       = var.cluster_api_url
  allow_deletion        = true
}

output "user_name" {
  description = "The name of the created user"
  value       = redpanda_user.example.name
}
