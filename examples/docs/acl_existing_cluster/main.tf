provider "redpanda" {}

variable "cluster_api_url" {
  type        = string
  description = "Data plane URL of an existing cluster, shown on the cluster's overview page in Redpanda Cloud"
}

variable "user_password" {
  type        = string
  sensitive   = true
  description = "Password for the Redpanda user"
}

locals {
  principal = "User:${redpanda_user.consumer.name}"

  # The minimum a consumer needs: read and describe its topics, join its groups.
  topic_ops = ["READ", "DESCRIBE"]
  group_ops = ["READ", "DESCRIBE"]
}

resource "redpanda_user" "consumer" {
  name                = "example-consumer"
  password_wo         = var.user_password
  password_wo_version = 1
  mechanism           = "scram-sha-256"
  cluster_api_url     = var.cluster_api_url
  allow_deletion      = true
}

resource "redpanda_acl" "topic" {
  for_each = toset(local.topic_ops)

  resource_type         = "TOPIC"
  resource_name         = "orders-"
  resource_pattern_type = "PREFIXED"
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
  resource_name         = "orders-consumers"
  resource_pattern_type = "LITERAL"
  principal             = local.principal
  host                  = "*"
  operation             = each.value
  permission_type       = "ALLOW"
  cluster_api_url       = var.cluster_api_url
  allow_deletion        = true
}
