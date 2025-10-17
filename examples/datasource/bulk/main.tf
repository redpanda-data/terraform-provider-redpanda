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
  allow_deletion  = true
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
