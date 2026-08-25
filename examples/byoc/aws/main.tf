provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = var.resource_group_name
}

resource "redpanda_network" "test" {
  name              = var.network_name
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = var.cloud_provider
  region            = var.region
  cluster_type      = "byoc"
  cidr_block        = "10.0.0.0/20"
}

resource "redpanda_cluster" "test" {
  name              = var.cluster_name
  resource_group_id = redpanda_resource_group.test.id
  network_id        = redpanda_network.test.id
  cloud_provider    = redpanda_network.test.cloud_provider
  region            = redpanda_network.test.region
  cluster_type      = redpanda_network.test.cluster_type
  # Dual listener mode: feature-gated, AWS BYOC only. A private-only cluster
  # cannot gain public listeners in place — the provider rejects that at plan
  # time. See the cluster docs "Limitations" section.
  connection_type   = var.dual_listener_connections == null ? "public" : null
  kafka_api = var.dual_listener_connections == null ? null : {
    connections = var.dual_listener_connections
  }
  http_proxy = var.dual_listener_connections == null ? null : {
    connections = var.dual_listener_connections
  }
  schema_registry = var.dual_listener_connections == null ? null : {
    connections = var.dual_listener_connections
  }
  throughput_tier = var.throughput_tier
  zones           = var.zones
  allow_deletion  = var.cluster_allow_deletion
  tags            = var.cluster_tags
  # aws_private_link = {
  #   enabled         = true
  #   connect_console = true
  #   allowed_principals = ["arn:aws:iam::123456789024:root"]
  # }

  timeouts = {
    create = var.cluster_create_timeout
  }
}

resource "redpanda_topic" "test" {
  name               = var.topic_name
  partition_count    = var.partition_count
  replication_factor = var.replication_factor
  cluster_api_url    = redpanda_cluster.test.cluster_api_url
  allow_deletion     = true
  configuration = var.topic_configuration != null ? var.topic_configuration : {
    "cleanup.policy" = "delete"
    "retention.ms"   = var.topic_retention_ms
  }
}

