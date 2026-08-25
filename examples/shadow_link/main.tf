provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = var.resource_group_name
}

resource "redpanda_network" "source" {
  name              = var.source_network_name
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

resource "redpanda_network" "shadow" {
  name              = var.shadow_network_name
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = var.cloud_provider
  region            = var.region
  cluster_type      = "dedicated"
  cidr_block        = "10.1.0.0/20"

  timeouts = {
    create = "20m"
    delete = "20m"
  }
}

resource "redpanda_cluster" "source" {
  name              = var.source_cluster_name
  resource_group_id = redpanda_resource_group.test.id
  network_id        = redpanda_network.source.id
  cloud_provider    = var.cloud_provider
  region            = var.region
  cluster_type      = "dedicated"
  connection_type   = "public"
  throughput_tier   = var.throughput_tier
  zones             = var.zones
  allow_deletion    = var.cluster_allow_deletion

  timeouts = {
    create = "90m"
  }
}

# Shadow cluster needs enable_shadow_linking to accept inbound shadow links.
resource "redpanda_cluster" "shadow" {
  name              = var.shadow_cluster_name
  resource_group_id = redpanda_resource_group.test.id
  network_id        = redpanda_network.shadow.id
  cloud_provider    = var.cloud_provider
  region            = var.region
  cluster_type      = "dedicated"
  connection_type   = "public"
  throughput_tier   = var.throughput_tier
  zones             = var.zones
  allow_deletion    = var.cluster_allow_deletion
  cluster_configuration = {
    custom_properties_json = jsonencode({
      "enable_shadow_linking" = true
    })
  }

  timeouts = {
    create = "90m"
  }
}

# Dataplane canaries, one per cluster. Every dataplane resource below waits on
# the canary for its own cluster, so a dataplane that is not serving yet fails
# here — on a topic — instead of on whichever resource wins the race. Both
# clusters need one: the user and ACLs live on source, the secrets on shadow.
resource "redpanda_topic" "source_canary" {
  name               = "${var.link_name}-source-canary"
  partition_count    = 1
  replication_factor = 3
  cluster_api_url    = redpanda_cluster.source.cluster_api_url
  allow_deletion     = true
}

resource "redpanda_topic" "shadow_canary" {
  name               = "${var.link_name}-shadow-canary"
  partition_count    = 1
  replication_factor = 3
  cluster_api_url    = redpanda_cluster.shadow.cluster_api_url
  allow_deletion     = true
}

# A user on the source cluster that the shadow link will authenticate as.
resource "redpanda_user" "shadow_link_user" {
  name            = var.user_name
  password        = var.user_password
  mechanism       = "scram-sha-256"
  cluster_api_url = redpanda_cluster.source.cluster_api_url
  allow_deletion  = true

  depends_on = [redpanda_topic.source_canary]
}

resource "redpanda_acl" "shadow_link_user_read_topics" {
  resource_type         = "TOPIC"
  resource_name         = "*"
  resource_pattern_type = "LITERAL"
  principal             = "User:${redpanda_user.shadow_link_user.name}"
  host                  = "*"
  operation             = "READ"
  permission_type       = "ALLOW"
  cluster_api_url       = redpanda_cluster.source.cluster_api_url
  allow_deletion        = true

  depends_on = [redpanda_topic.source_canary]
}

resource "redpanda_acl" "shadow_link_user_describe_topics" {
  resource_type         = "TOPIC"
  resource_name         = "*"
  resource_pattern_type = "LITERAL"
  principal             = "User:${redpanda_user.shadow_link_user.name}"
  host                  = "*"
  operation             = "DESCRIBE"
  permission_type       = "ALLOW"
  cluster_api_url       = redpanda_cluster.source.cluster_api_url
  allow_deletion        = true

  depends_on = [redpanda_topic.source_canary]
}

resource "redpanda_acl" "shadow_link_user_describe_cluster" {
  resource_type         = "CLUSTER"
  resource_name         = "kafka-cluster"
  resource_pattern_type = "LITERAL"
  principal             = "User:${redpanda_user.shadow_link_user.name}"
  host                  = "*"
  operation             = "DESCRIBE"
  permission_type       = "ALLOW"
  cluster_api_url       = redpanda_cluster.source.cluster_api_url
  allow_deletion        = true

  depends_on = [redpanda_topic.source_canary]
}

# Stored in the shadow cluster's secret store so the link can reference it via $${secrets.<NAME>}.
resource "redpanda_secret" "source_password" {
  name                = var.secret_name
  secret_data         = var.user_password
  secret_data_version = 1
  scopes              = ["SCOPE_REDPANDA_CLUSTER"]
  cluster_api_url     = redpanda_cluster.shadow.cluster_api_url
  allow_deletion      = true

  depends_on = [redpanda_topic.shadow_canary]
}

# The Schema Registry credential gets its own secret rather than sharing the
# SCRAM one, so the test can tell the two references apart.
resource "redpanda_secret" "source_schema_registry_password" {
  name                = var.sr_secret_name
  secret_data         = var.user_password
  secret_data_version = 1
  scopes              = ["SCOPE_REDPANDA_CLUSTER"]
  cluster_api_url     = redpanda_cluster.shadow.cluster_api_url
  allow_deletion      = true

  depends_on = [redpanda_topic.shadow_canary]
}

resource "redpanda_shadow_link" "test" {
  name               = var.link_name
  shadow_redpanda_id = redpanda_cluster.shadow.id
  source_redpanda_id = redpanda_cluster.source.id

  client_options = {
    tls_settings = {
      enabled = true
    }
    authentication_configuration = {
      scram_configuration = {
        scram_mechanism = "SCRAM_SHA_256"
        username        = redpanda_user.shadow_link_user.name
        password        = "$${secrets.${redpanda_secret.source_password.name}}"
      }
    }
    metadata_max_age_ms = var.metadata_max_age_ms
  }

  # Replicates Schema Registry over the HTTP API. source_url points at the
  # source cluster's own registry so the endpoint is real and reachable rather
  # than a placeholder the control plane would accept but never resolve.
  schema_registry_sync_options = {
    shadow_schema_registry_api = {
      source_url = redpanda_cluster.source.schema_registry.url

      auth_options = {
        basic = {
          username = redpanda_user.shadow_link_user.name
          password = "$${secrets.${redpanda_secret.source_schema_registry_password.name}}"
        }
      }

      tail_interval                  = var.sr_tail_interval
      full_sync_interval             = "5m0s"
      max_source_requests_per_second = 30

      source_filter = {
        contexts = ["."]
      }

      # identity preserves source context names; the exact arm is the sibling.
      destination = {
        identity = true
      }

      unsupported_schema_feature_policy = "FAIL"
      paused                            = false
    }
  }

  # Role shadowing is deny-by-default: nothing syncs until at least one
  # include filter is present, and "sync everything" is exactly one LITERAL
  # INCLUDE filter named "*".
  role_sync_options = {
    interval = "45s"
    paused   = var.role_sync_paused
    role_name_filters = [
      {
        filter_type  = "INCLUDE"
        name         = "*"
        pattern_type = "LITERAL"
      }
    ]
  }

  allow_deletion = true
}
