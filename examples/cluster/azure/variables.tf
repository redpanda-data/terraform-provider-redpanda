variable "resource_group_name" {
  default = "testname"
}

variable "network_name" {
  default = "testname"
}

variable "cluster_name" {
  default = "testname"
}

variable "cloud_provider" {
  default = "azure"
}

variable "region" {
  default = "westus2"
}

variable "zones" {
  default = ["westus2-az1", "westus2-az2", "westus2-az3"]
}

variable "throughput_tier" {
  default = "tier-1-azure-v3-x86"
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