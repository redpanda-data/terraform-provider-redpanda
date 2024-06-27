provider "redpanda" {
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
  count              = 100
  name               = "${var.topic_name}_${count.index + 1}"
  partition_count    = var.partition_count
  replication_factor = var.replication_factor
  cluster_api_url    = data.redpanda_cluster.test.cluster_api_url
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
  cluster_api_url       = data.redpanda_cluster.test.cluster_api_url
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
  default = ["use1-az2", "use1-az4", "use1-az6"]
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
