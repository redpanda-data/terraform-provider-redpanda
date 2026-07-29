provider "redpanda" {}

variable "source_password" {
  type        = string
  sensitive   = true
  description = "SCRAM password for the source cluster (will be stored as a dataplane secret)"
}

variable "source_schema_registry_password" {
  type        = string
  sensitive   = true
  description = "Schema Registry HTTP Basic auth password for the source cluster"
}

resource "redpanda_resource_group" "example" {
  name = "example-resource-group"
}

resource "redpanda_network" "shadow" {
  name              = "example-shadow-network"
  resource_group_id = redpanda_resource_group.example.id
  cloud_provider    = "aws"
  region            = "us-west-2"
  cluster_type      = "byoc"
  cidr_block        = "10.1.0.0/20"
}

# The shadow cluster runs the link. enable_shadow_linking must be set on this cluster.
resource "redpanda_cluster" "shadow" {
  name              = "example-shadow"
  resource_group_id = redpanda_resource_group.example.id
  network_id        = redpanda_network.shadow.id
  cloud_provider    = "aws"
  region            = "us-west-2"
  cluster_type      = "byoc"
  connection_type   = "public"
  throughput_tier   = "tier-1-aws-v2-arm"
  zones             = ["us-west-2a", "us-west-2b", "us-west-2c"]
  cluster_configuration = {
    custom_properties_json = jsonencode({
      enable_shadow_linking = true
    })
  }
}

# Store the SCRAM password used by the shadow link in the shadow cluster's secret store.
resource "redpanda_secret" "source_password" {
  name                = "SOURCE_PASSWORD"
  secret_data         = var.source_password
  secret_data_version = 1
  scopes              = ["SCOPE_REDPANDA_CLUSTER"]
  cluster_api_url     = redpanda_cluster.shadow.cluster_api_url
  allow_deletion      = true
}

# Schema Registry HTTP Basic auth password, stored alongside the SCRAM password.
resource "redpanda_secret" "source_schema_registry_password" {
  name                = "SOURCE_SR_PASSWORD"
  secret_data         = var.source_schema_registry_password
  secret_data_version = 1
  scopes              = ["SCOPE_REDPANDA_CLUSTER"]
  cluster_api_url     = redpanda_cluster.shadow.cluster_api_url
  allow_deletion      = true
}

resource "redpanda_shadow_link" "example" {
  name               = "example-link"
  shadow_redpanda_id = redpanda_cluster.shadow.id
  source_redpanda_id = "redpanda-id-of-source-cluster"

  client_options = {
    authentication_configuration = {
      scram_configuration = {
        scram_mechanism = "SCRAM_SHA_256"
        username        = "shadow-link-user"
        password        = "$${secrets.${redpanda_secret.source_password.name}}"
      }
    }
  }

  # Replicate Schema Registry over the HTTP API. This is one arm of the
  # schema_registry_shadowing_mode oneof — set shadow_schema_registry_topic
  # instead to shadow the `_schemas` topic byte-for-byte.
  schema_registry_sync_options = {
    shadow_schema_registry_api = {
      source_url = "https://source-schema-registry.example.com:8081"

      auth_options = {
        basic = {
          username = "schema-registry-user"
          password = "$${secrets.${redpanda_secret.source_schema_registry_password.name}}"
        }
      }

      # Poll for new subjects every 10s; full reconcile every 5 minutes.
      tail_interval                  = "10s"
      full_sync_interval             = "5m0s"
      max_source_requests_per_second = 30

      # Replicate only these contexts; omit source_filter to replicate everything.
      source_filter = {
        contexts = [".", ".prod"]
      }

      # Preserve source context names on the shadow cluster. Use
      # destination.exact instead to remap them.
      destination = {
        identity = true
      }

      unsupported_schema_feature_policy = "FAIL"
    }
  }

  allow_deletion = true
}
