variable "resource_group_name" {
  default = "testgroup"
}

variable "cluster_name" {
  default = "testname"
}

variable "region" {
  default = "eu-west-1"
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

variable "user_name" {
  default = "test-username"
}

variable "user_pw" {
  default = "password"
}

variable "mechanism" {
  default = "scram-sha-256"
}

variable "user_allow_deletion" {
  description = "Allow deletion of user resource"
  type        = bool
  default     = true
}

variable "public_networking" {
  description = "Value for public networking config (one of STATE_UNSPECIFIED, STATE_ENABLED, or STATE_DISABLED)"
  type        = string
  default     = "STATE_ENABLED"
}

variable "private_networking" {
  description = "Value for private networking config (one of STATE_UNSPECIFIED, STATE_ENABLED, or STATE_DISABLED)"
  type        = string
  default     = "STATE_DISABLED"
}

variable "allowed_principals" {
  description = "AWS principal ARNs allowed to connect via private link. Required when private_networking=STATE_ENABLED."
  type        = list(string)
  default     = []
}

variable "allow_private_link_deletion" {
  type        = bool
  description = "Allow deletion of the serverless private link. Set to true to allow terraform destroy."
  default     = false
}

variable "cluster_allow_deletion" {
  description = "Allow deletion of serverless cluster resource. Set to true to allow terraform destroy."
  type        = bool
  default     = false
}

variable "topic_configuration" {
  description = "Full override of the redpanda_topic configuration map. When non-null, replaces the default {cleanup.policy, retention.ms} pair."
  type        = map(string)
  default     = null
}

variable "topic_retention_ms" {
  description = "retention.ms entry on the redpanda_topic configuration map. Mutated by acceptance tests to exercise the Update path on topic configuration."
  type        = string
  default     = "604800000"
}

variable "acl_allow_deletion" {
  description = "Allow deletion of the ACL resource"
  type        = bool
  default     = true
}

variable "secret_value" {
  description = "Value stored in the cluster's secret store"
  type        = string
  sensitive   = true
  default     = "acc-test-secret-value"
}

variable "secret_allow_deletion" {
  description = "Allow deletion of the secret resource"
  type        = bool
  default     = true
}

variable "pipeline_state" {
  description = "Desired pipeline state. Acceptance tests flip this to running."
  type        = string
  default     = "stopped"
}

variable "pipeline_allow_deletion" {
  description = "Allow deletion of the pipeline resource"
  type        = bool
  default     = true
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

variable "protobuf_schema_definition" {
  description = "The PROTOBUF schema definition used to exercise canonicalization round-trip"
  default     = <<EOF
syntax = "proto3";
package tfrp.acc.v1;

message Event {
  Status status = 1;
  string id = 2;
}

enum Status {
  STATUS_UNSPECIFIED = 0;
  STATUS_ACTIVE = 1;
}
EOF
}

variable "compatibility_level" {
  description = "The compatibility level for schema evolution (BACKWARD, BACKWARD_TRANSITIVE, FORWARD, FORWARD_TRANSITIVE, FULL, FULL_TRANSITIVE, NONE)"
  default     = "FULL"
}

variable "sr_acl_allow_deletion" {
  description = "Allow deletion of Schema Registry ACL resources"
  type        = bool
  default     = true
}

variable "user_password_wo" {
  description = "Write-only password for user (not stored in state)"
  type        = string
  default     = null
  sensitive   = true
}

variable "user_password_wo_version" {
  description = "Version number to trigger password update when using password_wo"
  type        = number
  default     = null
}
