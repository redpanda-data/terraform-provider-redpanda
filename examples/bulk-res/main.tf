terraform {
  required_providers {
    redpanda = {
      source = "redpanda-data/redpanda"
    }
  }
}

provider "redpanda" {
  # Configuration options for the redpanda provider
}

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
    // not actually used as API does not consume it yet but we keep it in state for when it does
    "key" = "value"
  }
}
resource "redpanda_user" "test" {
  name            = var.user_name
  password        = var.user_pw
  mechanism       = var.mechanism
  cluster_api_url = redpanda_cluster.test.cluster_api_url
}

resource "redpanda_topic" "test" {
  count              = 100
  name               = "${var.topic_name}_${count.index + 1}"
  partition_count    = var.partition_count
  replication_factor = var.replication_factor
  cluster_api_url    = redpanda_cluster.test.cluster_api_url
  allow_deletion     = true
}

resource "redpanda_acl" "test" {
  count                 = 100
  resource_type         = "TOPIC"
  resource_name         = redpanda_topic.test[count.index].name
  resource_pattern_type = "LITERAL"
  principal             = "User:${redpanda_user.test.name}"
  host                  = "*"
  operation             = "READ"
  permission_type       = "ALLOW"
  cluster_api_url       = redpanda_cluster.test.cluster_api_url
}

variable "cluster_id" {
  default = "testname"
}
variable "resource_group_name" {
  default = "tfrp-acc-testbulk-aKtA"
}

variable "network_name" {
  default = "public-network-rate-limit-test"
}

variable "cluster_name" {
  default = "rate-limit-test"
}

variable "region" {
  default = "us-east-2"
}

variable "zones" {
  default = ["use2-az1"]
}

variable "cloud_provider" {
  default = "aws"
}

variable "user_name" {
  default = "test-username"
}

variable "user_pw" {
  default = "password"
}

variable "mechanism" {
  default = "scram-sha-256"
}

variable "topic_name" {
  default = "test-topic"
}

variable "partition_count" {
  default = 3
}

variable "replication_factor" {
  default = 3
}

variable "throughput_tier" {
  default = "tier-1-aws-v2-arm"
}