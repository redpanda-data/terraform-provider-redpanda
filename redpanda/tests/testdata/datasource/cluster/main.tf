provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = var.name_prefix
}

resource "redpanda_network" "test" {
  name              = var.name_prefix
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = var.cloud_provider
  region            = var.region
  cluster_type      = "dedicated"
  cidr_block        = "10.0.0.0/20"

  timeouts = {
    create = "20m"
    delete = "20m"
  }
}

resource "redpanda_cluster" "test" {
  name              = var.name_prefix
  resource_group_id = redpanda_resource_group.test.id
  network_id        = redpanda_network.test.id
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
  id = redpanda_cluster.test.id
}

resource "redpanda_user" "test" {
  name            = "${var.name_prefix}-user"
  password        = var.user_pw
  mechanism       = var.mechanism
  cluster_api_url = redpanda_cluster.test.cluster_api_url
  allow_deletion  = true
}

resource "redpanda_topic" "test" {
  name               = "${var.name_prefix}-topic"
  partition_count    = var.partition_count
  replication_factor = var.replication_factor
  cluster_api_url    = redpanda_cluster.test.cluster_api_url
  allow_deletion     = true
}

resource "redpanda_acl" "test" {
  resource_type         = "TOPIC"
  resource_name         = redpanda_topic.test.name
  resource_pattern_type = "LITERAL"
  principal             = "User:${redpanda_user.test.name}"
  host                  = "*"
  operation             = "READ"
  permission_type       = "ALLOW"
  cluster_api_url       = redpanda_cluster.test.cluster_api_url
  allow_deletion        = true
}

resource "redpanda_role" "test" {
  name            = "${var.name_prefix}-role"
  cluster_api_url = redpanda_cluster.test.cluster_api_url
  allow_deletion  = true
}

resource "redpanda_role_assignment" "test" {
  role_name       = redpanda_role.test.name
  principal       = "User:${redpanda_user.test.name}"
  cluster_api_url = redpanda_cluster.test.cluster_api_url
}

output "datasource_cluster_id" {
  value = data.redpanda_cluster.test.id
}

output "datasource_cluster_name" {
  value = data.redpanda_cluster.test.name
}
