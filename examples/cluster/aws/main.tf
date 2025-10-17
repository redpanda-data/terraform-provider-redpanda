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
  allow_deletion    = true
  cluster_configuration = {
    custom_properties_json = jsonencode({
      "schema_registry_enable_authorization" = true
    })
  }
  tags = {
    "key" = "value"
  }

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
  allow_deletion     = var.topic_allow_deletion
}


resource "redpanda_acl" "cluster_admin" {
  resource_type         = "CLUSTER"
  resource_name         = "kafka-cluster"
  resource_pattern_type = "LITERAL"
  principal             = "User:${redpanda_user.test.name}"
  host                  = var.acl_host
  operation             = "ALL"
  permission_type       = var.acl_permission_type
  cluster_api_url       = redpanda_cluster.test.cluster_api_url
  allow_deletion        = var.acl_allow_deletion
}

resource "redpanda_acl" "schema_registry_admin" {
  resource_type         = "CLUSTER"
  resource_name         = "kafka-cluster"
  resource_pattern_type = "LITERAL"
  principal             = "User:${redpanda_user.test.name}"
  host                  = var.acl_host
  operation             = "ALTER"
  permission_type       = var.acl_permission_type
  cluster_api_url       = redpanda_cluster.test.cluster_api_url
  allow_deletion        = var.acl_allow_deletion
}

resource "redpanda_acl" "cluster_action" {
  resource_type         = "CLUSTER"
  resource_name         = "kafka-cluster"
  resource_pattern_type = "LITERAL"
  principal             = "User:${redpanda_user.test.name}"
  host                  = var.acl_host
  operation             = "CLUSTER_ACTION"
  permission_type       = var.acl_permission_type
  cluster_api_url       = redpanda_cluster.test.cluster_api_url
  allow_deletion        = var.acl_allow_deletion
}

resource "redpanda_acl" "topic_access" {
  resource_type         = "TOPIC"
  resource_name         = var.topic_name
  resource_pattern_type = "LITERAL"
  principal             = "User:${redpanda_user.test.name}"
  host                  = var.acl_host
  operation             = "ALL"
  permission_type       = var.acl_permission_type
  cluster_api_url       = redpanda_cluster.test.cluster_api_url
  allow_deletion        = var.acl_allow_deletion
}

# Schema Registry ACLs
resource "redpanda_schema_registry_acl" "all_test_topic" {
  cluster_id     = redpanda_cluster.test.id
  principal      = "User:${redpanda_user.test.name}"
  resource_type  = "SUBJECT"
  resource_name  = "${var.topic_name}-"
  pattern_type   = "PREFIXED"
  host           = var.sr_acl_host
  operation      = "ALL"
  permission     = var.sr_acl_permission
  username       = redpanda_user.test.name
  password       = var.user_pw
  allow_deletion = var.sr_acl_allow_deletion
  depends_on     = [redpanda_acl.schema_registry_admin]
}

resource "redpanda_schema_registry_acl" "describe_registry" {
  cluster_id     = redpanda_cluster.test.id
  principal      = "User:${redpanda_user.test.name}"
  resource_type  = "REGISTRY"
  resource_name  = "*"
  pattern_type   = "LITERAL"
  host           = var.sr_acl_host
  operation      = "DESCRIBE"
  permission     = var.sr_acl_permission
  username       = redpanda_user.test.name
  password       = var.user_pw
  allow_deletion = var.sr_acl_allow_deletion

  depends_on = [redpanda_acl.schema_registry_admin]
}

resource "redpanda_schema_registry_acl" "alter_configs_registry" {
  cluster_id     = redpanda_cluster.test.id
  principal      = "User:${redpanda_user.test.name}"
  resource_type  = "REGISTRY"
  resource_name  = "*"
  pattern_type   = "LITERAL"
  host           = var.sr_acl_host
  operation      = "ALTER_CONFIGS"
  permission     = var.sr_acl_permission
  username       = redpanda_user.test.name
  password       = var.user_pw
  allow_deletion = var.sr_acl_allow_deletion

  depends_on = [redpanda_acl.schema_registry_admin]
}

resource "redpanda_schema" "user_schema" {
  cluster_id     = redpanda_cluster.test.id
  subject        = var.user_schema_subject != null ? var.user_schema_subject : "${var.topic_name}-value"
  schema_type    = var.schema_type
  schema         = var.user_schema_definition
  username       = redpanda_user.test.name
  password       = var.user_pw
  allow_deletion = var.schema_allow_deletion

  depends_on = [
    redpanda_acl.cluster_admin,
    redpanda_acl.schema_registry_admin,
    redpanda_acl.cluster_action,
    redpanda_acl.topic_access,
    redpanda_schema_registry_acl.all_test_topic,
    redpanda_schema_registry_acl.describe_registry,
    redpanda_schema_registry_acl.alter_configs_registry
  ]
}


resource "redpanda_schema" "user_event_schema" {
  cluster_id     = redpanda_cluster.test.id
  subject        = var.user_event_schema_subject != null ? var.user_event_schema_subject : "${var.topic_name}-events-value"
  schema_type    = var.schema_type
  schema         = var.user_event_schema_definition
  username       = redpanda_user.test.name
  password       = var.user_pw
  allow_deletion = var.schema_allow_deletion

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
    redpanda_schema_registry_acl.alter_configs_registry
  ]
}

resource "redpanda_schema" "product_schema" {
  cluster_id     = redpanda_cluster.test.id
  subject        = var.product_schema_subject != null ? var.product_schema_subject : "${var.topic_name}-product-value"
  schema_type    = var.schema_type
  schema         = var.product_schema_definition
  compatibility  = var.compatibility_level
  username       = redpanda_user.test.name
  password       = var.user_pw
  allow_deletion = var.schema_allow_deletion

  depends_on = [
    redpanda_acl.cluster_admin,
    redpanda_acl.schema_registry_admin,
    redpanda_acl.cluster_action,
    redpanda_acl.topic_access,
    redpanda_schema_registry_acl.all_test_topic,
    redpanda_schema_registry_acl.describe_registry,
    redpanda_schema_registry_acl.alter_configs_registry
  ]
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
