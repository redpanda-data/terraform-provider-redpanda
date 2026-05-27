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

# A user on the source cluster that the shadow link will authenticate as.
resource "redpanda_user" "shadow_link_user" {
  name            = var.user_name
  password        = var.user_password
  mechanism       = "scram-sha-256"
  cluster_api_url = redpanda_cluster.source.cluster_api_url
  allow_deletion  = true
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
}

# Stored in the shadow cluster's secret store so the link can reference it via $${secrets.<NAME>}.
resource "redpanda_secret" "source_password" {
  name                = var.secret_name
  secret_data         = var.user_password
  secret_data_version = 1
  scopes              = ["SCOPE_REDPANDA_CLUSTER"]
  cluster_api_url     = redpanda_cluster.shadow.cluster_api_url
  allow_deletion      = true
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

  allow_deletion = true
}
