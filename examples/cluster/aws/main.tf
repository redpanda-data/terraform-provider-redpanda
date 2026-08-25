provider "redpanda" {}

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

  timeouts = {
    create = "20m"
    delete = "20m"
  }
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
  allow_deletion    = var.cluster_allow_deletion
  cluster_configuration = {
    custom_properties_json = jsonencode({
      "enable_shadow_linking"                = var.cluster_enable_shadow_linking
      "schema_registry_enable_authorization" = true
    })
  }
  maintenance_window_config = {
    day_hour = {
      day_of_week = var.maintenance_day_of_week
      hour_of_day = var.maintenance_hour_of_day
    }
  }
  tags = var.cluster_tags
  timeouts = {
    create = "90m"
  }
}

resource "redpanda_user" "test" {
  name                = var.user_name
  password            = var.user_password_wo != null ? null : var.user_pw
  password_wo         = var.user_password_wo
  password_wo_version = var.user_password_wo_version
  mechanism           = var.mechanism
  cluster_api_url     = redpanda_cluster.test.cluster_api_url
  allow_deletion      = var.user_allow_deletion

  depends_on = [redpanda_topic.test]
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

# Bootstrap SR ACL grants for the provider's own Bearer-token principal.
# Two grants are required: the SUBJECT-scope grant authorizes
# POST /subjects/<subj>/versions; the REGISTRY-scope grant authorizes the
# follow-up GET /schemas/ids/<id>/versions that the SR client (franz-go) makes
# to fetch the full schema metadata after create. Without the REGISTRY grant,
# the schema is created but the follow-up GET 403s and the resource fails.
# User:* is broader than necessary; tighten once the exact principal is
# documented as discoverable.

resource "redpanda_acl" "role_topic_read" {
  resource_type         = "TOPIC"
  resource_name         = redpanda_topic.test.name
  resource_pattern_type = "LITERAL"
  principal             = "RedpandaRole:${redpanda_role.developer.name}"
  host                  = "*"
  operation             = "READ"
  permission_type       = "ALLOW"
  cluster_api_url       = redpanda_cluster.test.cluster_api_url
  allow_deletion        = var.acl_allow_deletion
}

# Console-endpoint canary. The topic canary proves the dataplane; the console API
# is a separate endpoint with separate readiness, so it needs its own. Role is the
# simplest console resource, and the console resources below wait on it.
resource "redpanda_role" "console_canary" {
  name            = "${var.role_name}-console-canary"
  cluster_api_url = redpanda_cluster.test.cluster_api_url
  allow_deletion  = true
}

resource "redpanda_role" "developer" {
  name            = var.role_name
  cluster_api_url = redpanda_cluster.test.cluster_api_url
  allow_deletion  = var.role_allow_deletion

  depends_on = [redpanda_topic.test,
    redpanda_role.console_canary,
  ]
}

resource "redpanda_role_assignment" "developer_assignment" {
  role_name       = redpanda_role.developer.name
  principal       = "User:${redpanda_user.test.name}"
  cluster_api_url = redpanda_cluster.test.cluster_api_url

  depends_on = [redpanda_user.test,
    redpanda_role.console_canary,
  ]
}

