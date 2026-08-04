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
  cluster_api_url    = redpanda_serverless_cluster.test.dataplane_api.url
  allow_deletion     = true
  configuration = var.topic_configuration != null ? var.topic_configuration : {
    "cleanup.policy" = "delete"
    "retention.ms"   = var.topic_retention_ms
  }
}

resource "redpanda_user" "test" {
  name                = var.user_name
  password            = var.user_password_wo != null ? null : var.user_pw
  password_wo         = var.user_password_wo
  password_wo_version = var.user_password_wo_version
  mechanism           = var.mechanism
  cluster_api_url     = redpanda_serverless_cluster.test.dataplane_api.url
  allow_deletion      = var.user_allow_deletion

  depends_on = [redpanda_topic.test]
}

# Secret store entry. Name must match ^[A-Z][A-Z0-9_]*$, so it is derived from
# the resource group name rather than taking a separate variable.
resource "redpanda_secret" "test" {
  name                = upper(replace(var.resource_group_name, "-", "_"))
  secret_data         = var.secret_value
  secret_data_version = 1
  scopes              = ["SCOPE_REDPANDA_CLUSTER"]
  cluster_api_url     = redpanda_serverless_cluster.test.dataplane_api.url
  allow_deletion      = var.secret_allow_deletion

  depends_on = [redpanda_topic.test]
}

# Redpanda Connect pipeline. display_name is asserted by the shared acceptance
# step builder; keep it in sync if you change it.
resource "redpanda_pipeline" "test" {
  display_name    = "test-pipeline"
  state           = var.pipeline_state
  cluster_api_url = redpanda_serverless_cluster.test.dataplane_api.url
  allow_deletion  = var.pipeline_allow_deletion
  config_yaml     = <<-YAML
    input:
      generate:
        interval: 60s
        mapping: 'root.id = uuid_v4()'
    output:
      drop: {}
  YAML

  depends_on = [redpanda_topic.test]
}

resource "redpanda_acl" "cluster_admin" {
  resource_type         = "CLUSTER"
  resource_name         = "kafka-cluster"
  resource_pattern_type = "LITERAL"
  principal             = "User:${redpanda_user.test.name}"
  host                  = "*"
  operation             = "ALL"
  permission_type       = "ALLOW"
  cluster_api_url       = redpanda_serverless_cluster.test.dataplane_api.url
  allow_deletion        = var.acl_allow_deletion
}

resource "redpanda_acl" "schema_registry_admin" {
  resource_type         = "CLUSTER"
  resource_name         = "kafka-cluster"
  resource_pattern_type = "LITERAL"
  principal             = "User:${redpanda_user.test.name}"
  host                  = "*"
  operation             = "ALTER"
  permission_type       = "ALLOW"
  cluster_api_url       = redpanda_serverless_cluster.test.dataplane_api.url
  allow_deletion        = var.acl_allow_deletion
}

resource "redpanda_acl" "cluster_action" {
  resource_type         = "CLUSTER"
  resource_name         = "kafka-cluster"
  resource_pattern_type = "LITERAL"
  principal             = "User:${redpanda_user.test.name}"
  host                  = "*"
  operation             = "CLUSTER_ACTION"
  permission_type       = "ALLOW"
  cluster_api_url       = redpanda_serverless_cluster.test.dataplane_api.url
  allow_deletion        = var.acl_allow_deletion
}

resource "redpanda_acl" "topic_access" {
  resource_type         = "TOPIC"
  resource_name         = redpanda_topic.test.name
  resource_pattern_type = "LITERAL"
  principal             = "User:${redpanda_user.test.name}"
  host                  = "*"
  operation             = "ALL"
  permission_type       = "ALLOW"
  cluster_api_url       = redpanda_serverless_cluster.test.dataplane_api.url
  allow_deletion        = var.acl_allow_deletion
}

resource "redpanda_schema_registry_acl" "provider_bootstrap_subject" {
  cluster_id     = redpanda_serverless_cluster.test.id
  principal      = "User:*"
  resource_type  = "SUBJECT"
  resource_name  = var.topic_name
  pattern_type   = "PREFIXED"
  host           = "*"
  operation      = "ALL"
  permission     = "ALLOW"
  allow_deletion = true

  depends_on = [redpanda_topic.test]
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

  depends_on = [redpanda_topic.test]
}

resource "redpanda_schema_registry_acl" "read_product" {
  cluster_id     = redpanda_serverless_cluster.test.id
  principal      = "User:${redpanda_user.test.name}"
  resource_type  = "SUBJECT"
  resource_name  = "product-"
  pattern_type   = "PREFIXED"
  host           = "*"
  operation      = "READ"
  permission     = "ALLOW"
  allow_deletion = var.sr_acl_allow_deletion
  depends_on     = [redpanda_acl.cluster_admin, redpanda_acl.schema_registry_admin]
}

resource "redpanda_schema_registry_acl" "write_orders" {
  cluster_id     = redpanda_serverless_cluster.test.id
  principal      = "User:${redpanda_user.test.name}"
  resource_type  = "SUBJECT"
  resource_name  = "orders-value"
  pattern_type   = "LITERAL"
  host           = "*"
  operation      = "WRITE"
  permission     = "ALLOW"
  allow_deletion = true

  depends_on = [redpanda_acl.cluster_admin, redpanda_acl.schema_registry_admin]
}

resource "redpanda_schema_registry_acl" "all_test_topic" {
  cluster_id     = redpanda_serverless_cluster.test.id
  principal      = "User:${redpanda_user.test.name}"
  resource_type  = "SUBJECT"
  resource_name  = "${var.topic_name}-"
  pattern_type   = "PREFIXED"
  host           = "*"
  operation      = "ALL"
  permission     = "ALLOW"
  allow_deletion = true

  depends_on = [redpanda_acl.cluster_admin, redpanda_acl.schema_registry_admin]
}

resource "redpanda_schema_registry_acl" "describe_registry" {
  cluster_id     = redpanda_serverless_cluster.test.id
  principal      = "User:${redpanda_user.test.name}"
  resource_type  = "REGISTRY"
  resource_name  = "*"
  pattern_type   = "LITERAL"
  host           = "*"
  operation      = "DESCRIBE"
  permission     = "ALLOW"
  allow_deletion = true

  depends_on = [redpanda_acl.cluster_admin, redpanda_acl.schema_registry_admin]
}

resource "redpanda_schema_registry_acl" "alter_configs_registry" {
  cluster_id     = redpanda_serverless_cluster.test.id
  principal      = "User:${redpanda_user.test.name}"
  resource_type  = "REGISTRY"
  resource_name  = "*"
  pattern_type   = "LITERAL"
  host           = "*"
  operation      = "ALTER_CONFIGS"
  permission     = "ALLOW"
  allow_deletion = true

  depends_on = [redpanda_acl.cluster_admin, redpanda_acl.schema_registry_admin]
}

resource "redpanda_schema_registry_acl" "read_registry" {
  cluster_id     = redpanda_serverless_cluster.test.id
  principal      = "User:${redpanda_user.test.name}"
  resource_type  = "REGISTRY"
  resource_name  = "*"
  pattern_type   = "LITERAL"
  host           = "*"
  operation      = "READ"
  permission     = "ALLOW"
  allow_deletion = true

  depends_on = [redpanda_acl.cluster_admin, redpanda_acl.schema_registry_admin]
}

resource "redpanda_schema" "user_schema" {
  cluster_id  = redpanda_serverless_cluster.test.id
  subject     = "${var.topic_name}-value"
  schema_type = var.schema_type
  schema      = var.user_schema_definition
  # Explicit: serverless Schema Registry defaults compatibility to NONE, while
  # dedicated defaults to BACKWARD.
  compatibility  = "BACKWARD"
  allow_deletion = true

  depends_on = [
    redpanda_acl.cluster_admin,
    redpanda_acl.schema_registry_admin,
    redpanda_acl.cluster_action,
    redpanda_acl.topic_access,
    redpanda_schema_registry_acl.provider_bootstrap_subject,
    redpanda_schema_registry_acl.provider_bootstrap_registry,
    redpanda_schema_registry_acl.all_test_topic,
    redpanda_schema_registry_acl.describe_registry,
    redpanda_schema_registry_acl.alter_configs_registry,
    redpanda_schema_registry_acl.read_registry,
  ]
}

resource "redpanda_schema" "user_event_schema" {
  cluster_id     = redpanda_serverless_cluster.test.id
  subject        = "${var.topic_name}-events-value"
  schema_type    = var.schema_type
  schema         = var.user_event_schema_definition
  allow_deletion = true

  references = [
    {
      name    = "User"
      subject = redpanda_schema.user_schema.subject
      version = redpanda_schema.user_schema.version
    }
  ]

  depends_on = [
    redpanda_acl.cluster_admin,
    redpanda_acl.schema_registry_admin,
    redpanda_acl.cluster_action,
    redpanda_acl.topic_access,
    redpanda_schema_registry_acl.provider_bootstrap_subject,
    redpanda_schema_registry_acl.provider_bootstrap_registry,
    redpanda_schema_registry_acl.all_test_topic,
    redpanda_schema_registry_acl.describe_registry,
    redpanda_schema_registry_acl.alter_configs_registry,
    redpanda_schema_registry_acl.read_registry,
  ]
}

resource "redpanda_schema" "product_schema" {
  cluster_id     = redpanda_serverless_cluster.test.id
  subject        = "${var.topic_name}-product-value"
  schema_type    = var.schema_type
  schema         = var.product_schema_definition
  compatibility  = var.compatibility_level
  allow_deletion = true

  depends_on = [
    redpanda_acl.cluster_admin,
    redpanda_acl.schema_registry_admin,
    redpanda_acl.cluster_action,
    redpanda_acl.topic_access,
    redpanda_schema_registry_acl.provider_bootstrap_subject,
    redpanda_schema_registry_acl.provider_bootstrap_registry,
    redpanda_schema_registry_acl.all_test_topic,
    redpanda_schema_registry_acl.describe_registry,
    redpanda_schema_registry_acl.alter_configs_registry,
    redpanda_schema_registry_acl.read_registry,
  ]
}

resource "redpanda_schema" "protobuf_roundtrip" {
  cluster_id     = redpanda_serverless_cluster.test.id
  subject        = "${var.topic_name}-protobuf-value"
  schema_type    = "PROTOBUF"
  schema         = var.protobuf_schema_definition
  references     = []
  allow_deletion = true

  depends_on = [
    redpanda_acl.cluster_admin,
    redpanda_acl.schema_registry_admin,
    redpanda_acl.cluster_action,
    redpanda_acl.topic_access,
    redpanda_schema_registry_acl.provider_bootstrap_subject,
    redpanda_schema_registry_acl.provider_bootstrap_registry,
    redpanda_schema_registry_acl.all_test_topic,
    redpanda_schema_registry_acl.describe_registry,
    redpanda_schema_registry_acl.alter_configs_registry,
    redpanda_schema_registry_acl.read_registry,
  ]
}
