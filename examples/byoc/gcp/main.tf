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
  connection_type   = "public"
  throughput_tier   = var.throughput_tier
  zones             = var.zones
  allow_deletion    = var.cluster_allow_deletion
  tags              = var.cluster_tags
  ## This is a reference for GCP Private Service Connect
  #   gcp_private_service_connect = {
  #     enabled               = true
  #     global_access_enabled = true
  #     consumer_accept_list = [
  #       {
  #         source = "123456789012"
  #       }
  #     ]
  #   }
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

