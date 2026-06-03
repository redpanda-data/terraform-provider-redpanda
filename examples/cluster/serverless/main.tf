provider "redpanda" {
}

resource "redpanda_resource_group" "test" {
  name = var.resource_group_name
}

resource "redpanda_serverless_private_link" "test" {
  count             = var.private_networking == "STATE_ENABLED" ? 1 : 0
  name              = "${var.cluster_name}-private-link"
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = "aws"
  serverless_region = var.region
  allow_deletion    = var.allow_private_link_deletion

  aws_config = {
    allowed_principals = var.allowed_principals
  }
}

resource "redpanda_serverless_cluster" "test" {
  name              = var.cluster_name
  resource_group_id = redpanda_resource_group.test.id
  serverless_region = var.region
  allow_deletion    = var.cluster_allow_deletion

  private_link_id = var.private_networking == "STATE_ENABLED" ? redpanda_serverless_private_link.test[0].id : null

  networking_config = {
    public  = var.public_networking
    private = var.private_networking
  }
}

resource "redpanda_topic" "test" {
  name               = var.topic_name
  partition_count    = var.partition_count
  replication_factor = var.replication_factor
  cluster_api_url    = redpanda_serverless_cluster.test.cluster_api_url
  allow_deletion     = true
}

resource "redpanda_user" "test" {
  name            = var.user_name
  password        = var.user_pw
  mechanism       = var.mechanism
  cluster_api_url = redpanda_serverless_cluster.test.cluster_api_url
  allow_deletion  = var.user_allow_deletion
}

# Schema Registry against a serverless cluster keys off cluster_id (control
# plane), not cluster_api_url; cluster_id resolves the SR endpoint via the
# ServerlessClusterService fallback in GetSchemaRegistryClientForCluster.
# Both redpanda_schema_registry_acl and redpanda_schema route through it.
resource "redpanda_schema_registry_acl" "provider_bootstrap_subject" {
  cluster_id     = redpanda_serverless_cluster.test.id
  principal      = "User:*"
  resource_type  = "SUBJECT"
  resource_name  = var.resource_group_name
  pattern_type   = "PREFIXED"
  host           = "*"
  operation      = "ALL"
  permission     = "ALLOW"
  allow_deletion = true
}

resource "redpanda_schema_registry_acl" "provider_bootstrap_registry" {
  cluster_id     = redpanda_serverless_cluster.test.id
  principal      = "User:*"
  resource_type  = "REGISTRY"
  resource_name  = "*"
  pattern_type   = "LITERAL"
  host           = "*"
  operation      = "ALL"
  permission     = "ALLOW"
  allow_deletion = true
}

resource "redpanda_schema" "user_schema" {
  cluster_id     = redpanda_serverless_cluster.test.id
  subject        = "${var.resource_group_name}-value"
  schema_type    = "AVRO"
  compatibility  = "BACKWARD"
  allow_deletion = true
  schema = jsonencode({
    type = "record"
    name = "User"
    fields = [
      { name = "id", type = "long" },
      { name = "name", type = "string" }
    ]
  })

  depends_on = [
    redpanda_schema_registry_acl.provider_bootstrap_subject,
    redpanda_schema_registry_acl.provider_bootstrap_registry,
  ]
}
