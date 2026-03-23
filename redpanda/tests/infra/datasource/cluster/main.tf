provider "redpanda" {}

locals {
  cluster_count = 3
  n             = var.resources_per_cluster
  total         = var.resources_per_cluster * local.cluster_count
  # Maps each dataplane resource index to its cluster index (0, 1, or 2)
  cluster_idx = { for i in range(local.total) : i => floor(i / local.n) }
}

resource "redpanda_resource_group" "test" {
  count = local.cluster_count
  name  = "${var.name_prefix}-${count.index + 1}"
}

resource "redpanda_network" "test" {
  count             = local.cluster_count
  name              = "${var.name_prefix}-${count.index + 1}"
  resource_group_id = redpanda_resource_group.test[count.index].id
  cloud_provider    = var.cloud_provider
  region            = var.region
  cluster_type      = "dedicated"
  cidr_block        = "10.${count.index + 1}.0.0/20"

  timeouts = {
    create = "20m"
    delete = "20m"
  }
}

resource "redpanda_cluster" "test" {
  count             = local.cluster_count
  name              = "${var.name_prefix}-${count.index + 1}"
  resource_group_id = redpanda_resource_group.test[count.index].id
  network_id        = redpanda_network.test[count.index].id
  cloud_provider    = var.cloud_provider
  region            = var.region
  cluster_type      = "dedicated"
  connection_type   = "public"
  throughput_tier   = var.throughput_tier
  zones             = var.zones
  allow_deletion    = true
  tags = {
    "purpose" = "datasource-cluster-test"
  }
  timeouts = {
    create = "90m"
  }
}

data "redpanda_cluster" "test" {
  count = local.cluster_count
  id    = redpanda_cluster.test[count.index].id
}

resource "redpanda_user" "test" {
  count           = local.total
  name            = "${var.name_prefix}-user-c${local.cluster_idx[count.index] + 1}_${count.index % local.n + 1}"
  password        = var.user_pw
  mechanism       = var.mechanism
  cluster_api_url = redpanda_cluster.test[local.cluster_idx[count.index]].cluster_api_url
  allow_deletion  = true
}

resource "redpanda_topic" "test" {
  count              = local.total
  name               = "${var.name_prefix}-topic-c${local.cluster_idx[count.index] + 1}_${count.index % local.n + 1}"
  partition_count    = var.partition_count
  replication_factor = var.replication_factor
  cluster_api_url    = redpanda_cluster.test[local.cluster_idx[count.index]].cluster_api_url
  allow_deletion     = true
}

resource "redpanda_acl" "test" {
  count                 = local.total
  resource_type         = "TOPIC"
  resource_name         = redpanda_topic.test[count.index].name
  resource_pattern_type = "LITERAL"
  principal             = "User:${redpanda_user.test[count.index].name}"
  host                  = "*"
  operation             = "READ"
  permission_type       = "ALLOW"
  cluster_api_url       = redpanda_cluster.test[local.cluster_idx[count.index]].cluster_api_url
  allow_deletion        = true
}

# --- Roles: 200 per cluster ---

resource "redpanda_role" "test" {
  count           = local.total
  name            = "${var.name_prefix}-role-c${local.cluster_idx[count.index] + 1}_${count.index % local.n + 1}"
  cluster_api_url = redpanda_cluster.test[local.cluster_idx[count.index]].cluster_api_url
  allow_deletion  = true
}

resource "redpanda_role_assignment" "test" {
  count           = local.total
  role_name       = redpanda_role.test[count.index].name
  principal       = redpanda_user.test[count.index].name
  cluster_api_url = redpanda_cluster.test[local.cluster_idx[count.index]].cluster_api_url
}

output "datasource_cluster_1_id" {
  value = data.redpanda_cluster.test[0].id
}

output "datasource_cluster_1_name" {
  value = data.redpanda_cluster.test[0].name
}

output "datasource_cluster_2_id" {
  value = data.redpanda_cluster.test[1].id
}

output "datasource_cluster_2_name" {
  value = data.redpanda_cluster.test[1].name
}

output "datasource_cluster_3_id" {
  value = data.redpanda_cluster.test[2].id
}

output "datasource_cluster_3_name" {
  value = data.redpanda_cluster.test[2].name
}
