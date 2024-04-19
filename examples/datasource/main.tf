provider "redpanda" {
}

variable "cluster_id" {
  default = ""
}

data "redpanda_cluster" "test" {
  id = var.cluster_id
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

resource "redpanda_topic" "test" {
  name               = var.topic_name
  partition_count    = var.partition_count
  replication_factor = var.replication_factor
  cluster_api_url    = data.redpanda_cluster.test.cluster_api_url
  allow_deletion     = true
}

variable "users" {
  description = "List of users to create"
  default     = [
    "user1",
    "user2",
    "user3",
    "user4",
    "user5",
    "user6",
    "user7",
    "user8",
    "user9",
    "user10",
    "user11",
    "user12",
    "user13",
    "user14",
    "user15",
    "user16",
    "user17",
    "user18",
    "user19",
    "user20",
    "user21",
    "user22",
    "user23",
    "user24",
    "user25",
    "user26",
    "user27",
    "user28",
    "user29",
    "user30",
  ]
}

resource "redpanda_user" "users" {
  count           = length(var.users)
  name            = var.users[count.index]
  password        = "password_${count.index + 1}"
  mechanism       = "scram-sha-256"
  cluster_api_url = data.redpanda_cluster.test.cluster_api_url
}

variable "operations" {
  description = "List of allowed ACL operations"
  default     = [
    "READ",
    "WRITE",
    "CREATE",
    "DELETE",
    "ALTER",
    "DESCRIBE",
    "DESCRIBE_CONFIGS",
    "ALTER_CONFIGS",
  ]
}

resource "redpanda_acl" "acls" {
  count                 = length(var.users) * length(var.operations)
  resource_type         = "TOPIC"
  resource_name         = redpanda_topic.test.name
  resource_pattern_type = "LITERAL"
  principal             = "User:${var.users[floor(count.index / length(var.operations))]}"
  host                  = "*"
  operation             = var.operations[count.index % length(var.operations)]
  permission_type       = "ALLOW"
  cluster_api_url       = data.redpanda_cluster.test.cluster_api_url
}