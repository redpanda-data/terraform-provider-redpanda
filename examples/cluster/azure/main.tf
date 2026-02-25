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
      "schema_registry_enable_authorization" = true
    })
  }
  tags = {
    "key" = "value"
  }
  #   azure_private_link = {
  #     enabled         = true
  #     connect_console = true
  #     allowed_subscriptions = ["12345678-1234-1234-1234-123456789012"]
  #   }

  timeouts = {
    create = "90m"
  }
}

resource "redpanda_user" "test" {
  name            = var.user_name
  password        = var.user_pw
  mechanism       = var.mechanism
  cluster_api_url = redpanda_cluster.test.cluster_api_url
  allow_deletion  = var.user_allow_deletion
}

resource "redpanda_topic" "test" {
  name               = var.topic_name
  partition_count    = var.partition_count
  replication_factor = var.replication_factor
  cluster_api_url    = redpanda_cluster.test.cluster_api_url
  allow_deletion     = true
}


resource "redpanda_schema" "user_schema" {
  cluster_id     = redpanda_cluster.test.id
  subject        = "${var.topic_name}-value"
  schema_type    = var.schema_type
  schema         = var.user_schema_definition
  username       = redpanda_user.test.name
  password       = var.user_pw
  allow_deletion = true

  depends_on = [
    redpanda_acl.cluster_admin,
    redpanda_acl.schema_registry_admin,
    redpanda_acl.cluster_action,
    redpanda_acl.topic_access,
    redpanda_schema_registry_acl.all_test_topic,
    redpanda_schema_registry_acl.describe_registry,
    redpanda_schema_registry_acl.alter_configs_registry,
    redpanda_schema_registry_acl.read_registry,
    redpanda_schema_registry_acl.write_registry
  ]
}

resource "redpanda_schema" "user_event_schema" {
  cluster_id     = redpanda_cluster.test.id
  subject        = "${var.topic_name}-events-value"
  schema_type    = var.schema_type
  schema         = var.user_event_schema_definition
  username       = redpanda_user.test.name
  password       = var.user_pw
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
    redpanda_schema_registry_acl.all_test_topic,
    redpanda_schema_registry_acl.describe_registry,
    redpanda_schema_registry_acl.alter_configs_registry,
    redpanda_schema_registry_acl.read_registry,
    redpanda_schema_registry_acl.write_registry
  ]
}

resource "redpanda_schema" "product_schema" {
  cluster_id     = redpanda_cluster.test.id
  subject        = "${var.topic_name}-product-value"
  schema_type    = var.schema_type
  schema         = var.product_schema_definition
  compatibility  = var.compatibility_level
  username       = redpanda_user.test.name
  password       = var.user_pw
  allow_deletion = true

  depends_on = [
    redpanda_acl.cluster_admin,
    redpanda_acl.schema_registry_admin,
    redpanda_acl.cluster_action,
    redpanda_acl.topic_access,
    redpanda_schema_registry_acl.all_test_topic,
    redpanda_schema_registry_acl.describe_registry,
    redpanda_schema_registry_acl.alter_configs_registry,
    redpanda_schema_registry_acl.read_registry,
    redpanda_schema_registry_acl.write_registry
  ]
}


resource "redpanda_acl" "cluster_admin" {
  resource_type         = "CLUSTER"
  resource_name         = "kafka-cluster"
  resource_pattern_type = "LITERAL"
  principal             = "User:${redpanda_user.test.name}"
  host                  = "*"
  operation             = "ALL"
  permission_type       = "ALLOW"
  cluster_api_url       = redpanda_cluster.test.cluster_api_url
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
  cluster_api_url       = redpanda_cluster.test.cluster_api_url
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
  cluster_api_url       = redpanda_cluster.test.cluster_api_url
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
  cluster_api_url       = redpanda_cluster.test.cluster_api_url
  allow_deletion        = var.acl_allow_deletion
}

resource "redpanda_schema_registry_acl" "read_product" {
  cluster_id     = redpanda_cluster.test.id
  principal      = "User:${redpanda_user.test.name}"
  resource_type  = "SUBJECT"
  resource_name  = "product-"
  pattern_type   = "PREFIXED"
  host           = "*"
  operation      = "READ"
  permission     = "ALLOW"
  username       = redpanda_user.test.name
  password       = var.user_pw
  allow_deletion = var.sr_acl_allow_deletion
  depends_on     = [redpanda_acl.schema_registry_admin]
}

resource "redpanda_schema_registry_acl" "write_orders" {
  cluster_id     = redpanda_cluster.test.id
  principal      = "User:${redpanda_user.test.name}"
  resource_type  = "SUBJECT"
  resource_name  = "orders-value"
  pattern_type   = "LITERAL"
  host           = "*"
  operation      = "WRITE"
  permission     = "ALLOW"
  username       = redpanda_user.test.name
  password       = var.user_pw
  allow_deletion = true

  depends_on = [redpanda_acl.schema_registry_admin]
}

resource "redpanda_schema_registry_acl" "all_test_topic" {
  cluster_id     = redpanda_cluster.test.id
  principal      = "User:${redpanda_user.test.name}"
  resource_type  = "SUBJECT"
  resource_name  = "${var.topic_name}-"
  pattern_type   = "PREFIXED"
  host           = "*"
  operation      = "ALL"
  permission     = "ALLOW"
  username       = redpanda_user.test.name
  password       = var.user_pw
  allow_deletion = true

  depends_on = [redpanda_acl.schema_registry_admin]
}

resource "redpanda_schema_registry_acl" "describe_test_topic" {
  cluster_id     = redpanda_cluster.test.id
  principal      = "User:${redpanda_user.test.name}"
  resource_type  = "SUBJECT"
  resource_name  = "${var.topic_name}-"
  pattern_type   = "PREFIXED"
  host           = "*"
  operation      = "DESCRIBE"
  permission     = "ALLOW"
  username       = redpanda_user.test.name
  password       = var.user_pw
  allow_deletion = true

  depends_on = [redpanda_acl.schema_registry_admin]
}

resource "redpanda_schema_registry_acl" "describe_registry" {
  cluster_id     = redpanda_cluster.test.id
  principal      = "User:${redpanda_user.test.name}"
  resource_type  = "REGISTRY"
  resource_name  = "*"
  pattern_type   = "LITERAL"
  host           = "*"
  operation      = "DESCRIBE"
  permission     = "ALLOW"
  username       = redpanda_user.test.name
  password       = var.user_pw
  allow_deletion = true

  depends_on = [redpanda_acl.schema_registry_admin]
}

resource "redpanda_schema_registry_acl" "alter_configs_registry" {
  cluster_id     = redpanda_cluster.test.id
  principal      = "User:${redpanda_user.test.name}"
  resource_type  = "REGISTRY"
  resource_name  = "*"
  pattern_type   = "LITERAL"
  host           = "*"
  operation      = "ALTER_CONFIGS"
  permission     = "ALLOW"
  username       = redpanda_user.test.name
  password       = var.user_pw
  allow_deletion = true

  depends_on = [redpanda_acl.schema_registry_admin]
}

resource "redpanda_schema_registry_acl" "read_registry" {
  cluster_id     = redpanda_cluster.test.id
  principal      = "User:${redpanda_user.test.name}"
  resource_type  = "REGISTRY"
  resource_name  = "*"
  pattern_type   = "LITERAL"
  host           = "*"
  operation      = "READ"
  permission     = "ALLOW"
  username       = redpanda_user.test.name
  password       = var.user_pw
  allow_deletion = true

  depends_on = [redpanda_acl.schema_registry_admin]
}

resource "redpanda_schema_registry_acl" "write_registry" {
  cluster_id     = redpanda_cluster.test.id
  principal      = "User:${redpanda_user.test.name}"
  resource_type  = "REGISTRY"
  resource_name  = "*"
  pattern_type   = "LITERAL"
  host           = "*"
  operation      = "WRITE"
  permission     = "ALLOW"
  username       = redpanda_user.test.name
  password       = var.user_pw
  allow_deletion = true

  depends_on = [redpanda_acl.schema_registry_admin]
}

resource "redpanda_role" "developer" {
  name            = var.role_name
  cluster_api_url = redpanda_cluster.test.cluster_api_url
  allow_deletion  = var.role_allow_deletion
}

resource "redpanda_role_assignment" "developer_assignment" {
  role_name       = redpanda_role.developer.name
  principal       = redpanda_user.test.name
  cluster_api_url = redpanda_cluster.test.cluster_api_url

  depends_on = [redpanda_user.test]
}

resource "redpanda_pipeline" "test" {
  cluster_api_url = redpanda_cluster.test.cluster_api_url
  display_name    = var.pipeline_name
  description     = var.pipeline_description
  config_yaml     = var.pipeline_config_yaml
  state           = var.pipeline_state
  allow_deletion  = var.pipeline_allow_deletion

  resources = {
    memory_shares = var.pipeline_memory_shares
    cpu_shares    = var.pipeline_cpu_shares
  }

  tags = {
    "environment" = "test"
    "managed-by"  = "terraform"
  }
}

output "pipeline_info" {
  description = "Information about the created pipeline"
  value = {
    id    = redpanda_pipeline.test.id
    name  = redpanda_pipeline.test.display_name
    state = redpanda_pipeline.test.state
    url   = redpanda_pipeline.test.url
  }
}

output "user_schema_info" {
  description = "Information about the created user schema"
  value = {
    id      = redpanda_schema.user_schema.id
    subject = redpanda_schema.user_schema.subject
    version = redpanda_schema.user_schema.version
    type    = redpanda_schema.user_schema.schema_type
  }
}

output "user_event_schema_info" {
  description = "Information about the created user event schema with references"
  value = {
    id         = redpanda_schema.user_event_schema.id
    subject    = redpanda_schema.user_event_schema.subject
    version    = redpanda_schema.user_event_schema.version
    type       = redpanda_schema.user_event_schema.schema_type
    references = redpanda_schema.user_event_schema.references
  }
}

output "product_schema_info" {
  description = "Information about the created product schema with compatibility settings"
  value = {
    id            = redpanda_schema.product_schema.id
    subject       = redpanda_schema.product_schema.subject
    version       = redpanda_schema.product_schema.version
    type          = redpanda_schema.product_schema.schema_type
    compatibility = redpanda_schema.product_schema.compatibility
  }
}
