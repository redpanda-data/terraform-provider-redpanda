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
  ## This is a reference for GCP tags
  #   tags = {
  #     "key" = "value"
  #   }
  ## This is a reference for GCP Private Service Connect
  #   gcp_private_service_connect = {
  #     enabled               = true
  #     global_access_enabled = true
  #     consumer_accept_list = [
  #       {
  #         source = "projects/123456789012"
  #       }
  #     ]
  #   }
}

variable "cluster_name" {
  default = ""
}

variable "resource_group_name" {
  default = ""
}

variable "network_name" {
  default = ""
}

variable "region" {
  default = "us-central1"
}

variable "zones" {
  default = ["us-central1-a", "us-central1-b", "us-central1-c"]
}

variable "cloud_provider" {
  default = "gcp"
}

variable "throughput_tier" {
  default = "tier-1-gcp-um4g"
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
  cluster_id  = redpanda_cluster.test.id
  subject     = "${var.topic_name}-value"
  schema_type = var.schema_type
  schema      = var.user_schema_definition
  username    = redpanda_user.test.name
  password    = var.user_pw
}

resource "redpanda_schema" "user_event_schema" {
  cluster_id  = redpanda_cluster.test.id
  subject     = "${var.topic_name}-events-value"
  schema_type = var.schema_type
  schema      = var.user_event_schema_definition
  username    = redpanda_user.test.name
  password    = var.user_pw
  
  # This schema references the user schema
  references = [
    {
      name    = "User"
      subject = redpanda_schema.user_schema.subject
      version = redpanda_schema.user_schema.version
    }
  ]
}

resource "redpanda_schema" "product_schema" {
  cluster_id    = redpanda_cluster.test.id
  subject       = "${var.topic_name}-product-value"
  schema_type   = var.schema_type
  schema        = var.product_schema_definition
  compatibility = var.compatibility_level
  username      = redpanda_user.test.name
  password      = var.user_pw
}


resource "redpanda_acl" "test" {
  resource_type         = "TOPIC"
  resource_name         = redpanda_topic.test.name
  resource_pattern_type = "LITERAL"
  principal             = "User:${redpanda_user.test.name}"
  host                  = "*"
  operation             = "READ"
  permission_type       = "ALLOW"
  cluster_api_url       = redpanda_cluster.test.cluster_api_url
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
  default = <<EOF
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
  default = <<EOF
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
  default = <<EOF
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
