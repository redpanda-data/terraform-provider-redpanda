provider "redpanda" {}

variable "source_password" {
  type        = string
  sensitive   = true
  description = "SCRAM password for the source cluster (will be stored as a dataplane secret)"
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

  allow_deletion = true
}
