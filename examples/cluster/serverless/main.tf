provider "redpanda" {
}

resource "redpanda_resource_group" "test" {
  name = var.resource_group_name
}

resource "redpanda_serverless_private_link" "test" {
  count = var.private_networking == "STATE_ENABLED" ? 1 : 0
  name               = "${var.cluster_name}-private-link"
  resource_group_id  = redpanda_resource_group.test.id
  cloud_provider     = "aws"
  serverless_region  = var.region
  allow_deletion = var.allow_private_link_deletion

  cloud_provider_config = {
    aws = {
      allowed_principals = var.allowed_principals
    }
  }
}

resource "redpanda_serverless_cluster" "test" {
  name              = var.cluster_name
  resource_group_id = redpanda_resource_group.test.id
  serverless_region = var.region

  private_link_id = var.private_networking == "STATE_ENABLED" ? redpanda_serverless_private_link.test[0].id : null

  networking_config = {
    public = var.public_networking
    private  = var.private_networking
  }
}

resource "redpanda_topic" "test" {
  name               = var.topic_name
  partition_count    = var.partition_count
  replication_factor = var.replication_factor
  cluster_api_url    = redpanda_serverless_cluster.test.cluster_api_url
  allow_deletion     = true
}

resource "redpanda_user" "test" {
  name            = var.user_name
  password        = var.user_pw
  mechanism       = var.mechanism
  cluster_api_url = redpanda_serverless_cluster.test.cluster_api_url
  allow_deletion  = var.user_allow_deletion
}
