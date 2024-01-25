provider "redpanda" {}

data "redpanda_namespace" "test" {
  id = var.namespace_id
}

data "redpanda_network" "test" {
  id = var.network_id
}

data "redpanda_cluster" "test" {
  id = var.cluster_id
}

resource "redpanda_user" "test" {
  name            = var.user_name
  password        = var.user_pw
  mechanism       = var.mechanism
  cluster_api_url = data.redpanda_cluster.test.cluster_api_url
}

resource "redpanda_topic" "test" {
  name               = var.topic_name
  partition_count    = var.partition_count
  replication_factor = var.replication_factor
  cluster_api_url    = data.redpanda_cluster.test.cluster_api_url
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
  cluster_api_url       = data.redpanda_cluster.test.cluster_api_url
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

variable "namespace_id" {
  default = ""
}

variable "network_id" {
  default = ""
}

variable "cluster_id" {
  default = ""
}