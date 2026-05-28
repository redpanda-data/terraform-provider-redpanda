provider "redpanda" {}

variable "user_password" {
  type        = string
  sensitive   = true
  description = "Password for the Redpanda SASL user (independent of Schema Registry auth)"
}

resource "redpanda_resource_group" "example" {
  name = "example-resource-group"
}

resource "redpanda_network" "example" {
  name              = "example-network"
  resource_group_id = redpanda_resource_group.example.id
  cloud_provider    = "aws"
  region            = "us-west-2"
  cluster_type      = "dedicated"
  cidr_block        = "10.0.0.0/20"
}

resource "redpanda_cluster" "example" {
  name              = "example-cluster"
  resource_group_id = redpanda_resource_group.example.id
  network_id        = redpanda_network.example.id
  cloud_provider    = "aws"
  region            = "us-west-2"
  cluster_type      = "dedicated"
  connection_type   = "public"
  throughput_tier   = "tier-1-aws-v2-arm"
  zones             = ["us-west-2a", "us-west-2b", "us-west-2c"]
}

resource "redpanda_user" "example" {
  name                = "schema-user"
  password_wo         = var.user_password
  password_wo_version = 1
  mechanism           = "scram-sha-256"
  cluster_api_url     = redpanda_cluster.example.cluster_api_url
  allow_deletion      = true
}

# The provider authenticates to Schema Registry using its cloud Bearer token.
# That token's effective SR principal needs explicit ACLs to manage schemas;
# grant them via redpanda_schema_registry_acl before the schema resource runs.
# Two grants are required: the SUBJECT grant authorizes
# POST /subjects/<subj>/versions; the REGISTRY grant authorizes the SR client's
# follow-up GET /schemas/ids/<id>/versions to fetch the full schema metadata.
resource "redpanda_schema_registry_acl" "provider_bootstrap_subject" {
  cluster_id     = redpanda_cluster.example.id
  principal      = "User:*"
  resource_type  = "SUBJECT"
  resource_name  = "user"
  pattern_type   = "PREFIXED"
  host           = "*"
  operation      = "ALL"
  permission     = "ALLOW"
  allow_deletion = true
}

resource "redpanda_schema_registry_acl" "provider_bootstrap_registry" {
  cluster_id     = redpanda_cluster.example.id
  principal      = "User:*"
  resource_type  = "REGISTRY"
  resource_name  = "*"
  pattern_type   = "LITERAL"
  host           = "*"
  operation      = "ALL"
  permission     = "ALLOW"
  allow_deletion = true
}

resource "redpanda_schema" "example" {
  cluster_id  = redpanda_cluster.example.id
  subject     = "user-value"
  schema_type = "AVRO"
  schema = jsonencode({
    type = "record"
    name = "User"
    fields = [
      {
        name = "id"
        type = "long"
      },
      {
        name = "username"
        type = "string"
      },
      {
        name = "email"
        type = "string"
      }
    ]
  })

  depends_on = [
    redpanda_schema_registry_acl.provider_bootstrap_subject,
    redpanda_schema_registry_acl.provider_bootstrap_registry,
  ]
}
