provider "redpanda" {}

resource "redpanda_namespace" "test" {
  name = var.namespace_name
}

resource "redpanda_network" "test" {
  name           = var.network_name
  namespace_id   = redpanda_namespace.test.id
  cloud_provider = var.cloud_provider
  region         = var.region
  cluster_type   = "dedicated"
  cidr_block     = "10.0.0.0/20"
}

resource "redpanda_cluster" "test" {
  name            = var.cluster_name
  namespace_id    = redpanda_namespace.test.id
  network_id      = redpanda_network.test.id
  cloud_provider  = var.cloud_provider
  region          = var.region
  cluster_type    = "dedicated"
  connection_type = "public"
  throughput_tier = var.throughput_tier
  zones           = var.zones
  allow_deletion  = true
  tags            = {
    "key" = "value"
  }
}
variable "cluster_name" {
  default = ""
}
variable "namespace_name" {
  default = ""
}
variable "network_name" {
  default = ""
}

variable "region" {
  default = "us-central1"
}

variable "zones" {
  default = ["us-central1-a", "us-central1-b", "us-central1-c"]
}

variable "cloud_provider" {
  default = "gcp"
}

variable "throughput_tier" {
  default = "tier-1-gcp-um4g"
}

resource "redpanda_user" "test" {
  name            = var.user_name
  password        = var.user_pw
  mechanism       = var.mechanism
  cluster_api_url = redpanda_cluster.test.cluster_api_url
}

resource "redpanda_topic" "test" {
  name               = var.topic_name
  partition_count    = var.partition_count
  replication_factor = var.replication_factor
  cluster_api_url    = redpanda_cluster.test.cluster_api_url
  allow_deletion     = true
}


#ACL_Operation_value = map[string]int32{
#  "OPERATION_UNSPECIFIED":      0,
#  "OPERATION_ANY":              1,
#  "OPERATION_ALL":              2,
#  "OPERATION_READ":             3,
#  "OPERATION_WRITE":            4,
#  "OPERATION_CREATE":           5,
#  "OPERATION_DELETE":           6,
#  "OPERATION_ALTER":            7,
#  "OPERATION_DESCRIBE":         8,
#  "OPERATION_CLUSTER_ACTION":   9,
#  "OPERATION_DESCRIBE_CONFIGS": 10,
#  "OPERATION_ALTER_CONFIGS":    11,
#  "OPERATION_IDEMPOTENT_WRITE": 12,
#  "OPERATION_CREATE_TOKENS":    13,
#  "OPERATION_DESCRIBE_TOKENS":  14,
#}

resource "redpanda_acl" "test" {
  resource_type         = "TOPIC"
  resource_name         = redpanda_topic.test.name
  resource_pattern_type = "LITERAL"
  principal             = "User:${redpanda_user.test.name}"
  host                  = "*"
  operation             = "READ"
  permission_type       = "ALLOW"
  cluster_api_url       = redpanda_cluster.test.cluster_api_url
}

resource "redpanda_acl" "test1" {
  resource_type         = "TOPIC"
  resource_name         = redpanda_topic.test.name
  resource_pattern_type = "LITERAL"
  principal             = "User:${redpanda_user.test.name}"
  host                  = "*"
  operation             = "WRITE"
  permission_type       = "ALLOW"
  cluster_api_url       = redpanda_cluster.test.cluster_api_url
}

resource "redpanda_acl" "test2" {
  resource_type         = "TOPIC"
  resource_name         = redpanda_topic.test.name
  resource_pattern_type = "LITERAL"
  principal             = "User:${redpanda_user.test.name}"
  host                  = "*"
  operation             = "CREATE"
  permission_type       = "ALLOW"
  cluster_api_url       = redpanda_cluster.test.cluster_api_url
}
resource "redpanda_acl" "test3" {
  resource_type         = "TOPIC"
  resource_name         = redpanda_topic.test.name
  resource_pattern_type = "LITERAL"
  principal             = "User:${redpanda_user.test.name}"
  host                  = "*"
  operation             = "DELETE"
  permission_type       = "ALLOW"
  cluster_api_url       = redpanda_cluster.test.cluster_api_url
}
resource "redpanda_acl" "test4" {
  resource_type         = "TOPIC"
  resource_name         = redpanda_topic.test.name
  resource_pattern_type = "LITERAL"
  principal             = "User:${redpanda_user.test.name}"
  host                  = "*"
  operation             = "ALTER"
  permission_type       = "ALLOW"
  cluster_api_url       = redpanda_cluster.test.cluster_api_url
}

resource "redpanda_acl" "test5" {
  resource_type         = "TOPIC"
  resource_name         = redpanda_topic.test.name
  resource_pattern_type = "LITERAL"
  principal             = "User:${redpanda_user.test.name}"
  host                  = "*"
  operation             = "DESCRIBE"
  permission_type       = "ALLOW"
  cluster_api_url       = redpanda_cluster.test.cluster_api_url
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
