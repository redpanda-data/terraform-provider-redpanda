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
  default = "tier-1-gcp-v2-x86"
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

variable "compatibility_level" {
  description = "The compatibility level for schema evolution (BACKWARD, BACKWARD_TRANSITIVE, FORWARD, FORWARD_TRANSITIVE, FULL, FULL_TRANSITIVE, NONE)"
  default     = "FULL"
}

variable "cluster_tags" {
  description = "Tags applied to the redpanda_cluster resource. Mutated by acceptance tests to exercise the Update path on tags."
  type        = map(string)
  default = {
    "key" = "value"
  }
}

variable "topic_retention_ms" {
  description = "retention.ms entry on the redpanda_topic configuration map. Mutated by acceptance tests to exercise the Update path on topic configuration."
  type        = string
  default     = "604800000"
}

variable "topic_configuration" {
  description = "Full override of the redpanda_topic configuration map. When non-null, replaces the default {cleanup.policy, retention.ms} pair."
  type        = map(string)
  default     = null
}