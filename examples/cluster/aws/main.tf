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
}

variable "resource_group_name" {
  default = "testname"
}

variable "network_name" {
  default = "testname"
}

variable "cluster_name" {
  default = "testname"
}

variable "region" {
  default = "us-east-2"
}

variable "zones" {
  default = ["use2-az1", "use2-az2", "use2-az3"]
}

variable "cloud_provider" {
  default = "aws"
}

variable "throughput_tier" {
  default = "tier-1-aws-v2-arm"
}

resource "redpanda_user" "test" {
  name            = var.user_name
  password        = var.user_pw
  mechanism       = var.mechanism
  cluster_api_url = redpanda_cluster.test.cluster_api_url
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
    redpanda_acl.schema_registry_admin,
    redpanda_schema_registry_acl.all_test_topic,
    redpanda_schema_registry_acl.describe_registry,
    redpanda_schema_registry_acl.alter_configs_registry
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
    redpanda_acl.schema_registry_admin,
    redpanda_schema_registry_acl.all_test_topic,
    redpanda_schema_registry_acl.describe_registry,
    redpanda_schema_registry_acl.alter_configs_registry
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
    redpanda_acl.schema_registry_admin,
    redpanda_schema_registry_acl.all_test_topic,
    redpanda_schema_registry_acl.describe_registry,
    redpanda_schema_registry_acl.alter_configs_registry
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
  allow_deletion        = true
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
  allow_deletion        = true
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
  allow_deletion = true
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


variable "user_name" {
  default = "test-username"
}

variable "user_pw" {
  default = "password"
}

variable "mechanism" {
  default = "scram-sha-256"
}

variable "topic_name" {
  default = "test-topic"
}

variable "partition_count" {
  default = 3
}

variable "replication_factor" {
  default = 3
}

variable "schema_type" {
  description = "The type of schema (AVRO, JSON, PROTOBUF)"
  default     = "AVRO"
}

variable "user_schema_definition" {
  description = "The AVRO schema definition for user data"
  default     = <<EOF
{
  "type": "record",
  "name": "User",
  "fields": [
    {
      "name": "id",
      "type": "int"
    },
    {
      "name": "name",
      "type": "string"
    },
    {
      "name": "email",
      "type": "string"
    },
    {
      "name": "created_at",
      "type": "long",
      "logicalType": "timestamp-millis"
    }
  ]
}
EOF
}

variable "user_event_schema_definition" {
  description = "The AVRO schema definition for user events that references the User schema"
  default     = <<EOF
{
  "type": "record",
  "name": "UserEvent",
  "fields": [
    {
      "name": "event_id",
      "type": "string"
    },
    {
      "name": "event_type",
      "type": {
        "type": "enum",
        "name": "EventType",
        "symbols": ["CREATED", "UPDATED", "DELETED"]
      }
    },
    {
      "name": "user",
      "type": "User"
    },
    {
      "name": "timestamp",
      "type": "long",
      "logicalType": "timestamp-millis"
    },
    {
      "name": "metadata",
      "type": ["null", {
        "type": "map",
        "values": "string"
      }],
      "default": null
    }
  ]
}
EOF
}

variable "product_schema_definition" {
  description = "The AVRO schema definition for product data with strict compatibility"
  default     = <<EOF
{
  "type": "record",
  "name": "Product",
  "fields": [
    {
      "name": "id",
      "type": "string"
    },
    {
      "name": "name",
      "type": "string"
    },
    {
      "name": "price",
      "type": {
        "type": "bytes",
        "logicalType": "decimal",
        "precision": 10,
        "scale": 2
      }
    },
    {
      "name": "category",
      "type": {
        "type": "enum",
        "name": "Category",
        "symbols": ["ELECTRONICS", "CLOTHING", "BOOKS", "HOME"]
      }
    },
    {
      "name": "description",
      "type": ["null", "string"],
      "default": null
    },
    {
      "name": "created_at",
      "type": "long",
      "logicalType": "timestamp-millis"
    }
  ]
}
EOF
}

variable "compatibility_level" {
  description = "The compatibility level for schema evolution (BACKWARD, BACKWARD_TRANSITIVE, FORWARD, FORWARD_TRANSITIVE, FULL, FULL_TRANSITIVE, NONE)"
  default     = "FULL"
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
