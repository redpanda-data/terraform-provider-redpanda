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

variable "cluster_enable_shadow_linking" {
  description = "Enable shadowlinking"
  type        = bool
  default     = true
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

variable "cluster_allow_deletion" {
  description = "Allow deletion of cluster resource"
  type        = bool
  default     = false
}

variable "user_allow_deletion" {
  description = "Allow deletion of user resource"
  type        = bool
  default     = true
}

variable "acl_allow_deletion" {
  description = "Allow deletion of ACL resources"
  type        = bool
  default     = true
}

variable "sr_acl_allow_deletion" {
  description = "Allow deletion of Schema Registry ACL resources"
  type        = bool
  default     = true
}

variable "role_name" {
  description = "Name of the role to create"
  default     = "developer"
}

variable "role_allow_deletion" {
  description = "Allow deletion of role resource"
  type        = bool
  default     = true
}

variable "pipeline_name" {
  description = "Name of the pipeline"
  default     = "test-pipeline"
}

variable "pipeline_description" {
  description = "Description of the pipeline"
  default     = "Test pipeline for acceptance testing"
}

variable "pipeline_config_yaml" {
  description = "Pipeline configuration in YAML format"
  default     = <<EOF
input:
  generate:
    interval: "1s"
    mapping: |
      root.message = "hello world"
      root.timestamp = now()

output:
  drop: {}
EOF
}

variable "pipeline_state" {
  description = "Desired state of the pipeline: 'running' or 'stopped'"
  type        = string
  default     = "stopped"
}

variable "pipeline_memory_shares" {
  description = "Memory allocation for the pipeline"
  default     = "256Mi"
}

variable "pipeline_cpu_shares" {
  description = "CPU allocation for the pipeline"
  default     = "200m"
}

variable "pipeline_allow_deletion" {
  description = "Allow deletion of pipeline resource"
  type        = bool
  default     = false
}
