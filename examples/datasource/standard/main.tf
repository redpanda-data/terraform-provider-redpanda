provider "redpanda" {}

variable "cluster_id" {
  default = ""
}

data "redpanda_cluster" "test" {
  id = var.cluster_id
}

resource "redpanda_topic" "test" {
  name               = var.topic_name
  partition_count    = var.partition_count
  replication_factor = var.replication_factor
  cluster_api_url    = data.redpanda_cluster.test.cluster_api_url
  allow_deletion     = true
  configuration      = var.topic_config
}

resource "redpanda_user" "test" {
  name            = var.user_name
  password        = var.user_pw
  mechanism       = var.mechanism
  cluster_api_url = data.redpanda_cluster.test.cluster_api_url
}

resource "redpanda_acl" "test" {
  resource_type         = "CLUSTER"
  resource_name         = "kafka-cluster"
  resource_pattern_type = "LITERAL"
  principal             = "User:${redpanda_user.test.name}"
  host                  = "*"
  operation             = "ALTER"
  permission_type       = "ALLOW"
  cluster_api_url       = data.redpanda_cluster.test.cluster_api_url
}

variable "topic_config" {
  default = {
    "cleanup.policy"   = "compact"
    "flush.ms"         = 100
    "compression.type" = "snappy"
  }
}

variable "user_name" {
  default = "data-test-username"
}

variable "user_pw" {
  default = "password"
}

variable "mechanism" {
  default = "scram-sha-256"
}

variable "topic_name" {
  default = "data-test-topic"
}

variable "partition_count" {
  default = 3
}

variable "replication_factor" {
  default = 3
}
