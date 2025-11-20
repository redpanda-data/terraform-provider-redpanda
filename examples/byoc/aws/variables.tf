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
  default = "tier-1-aws-v2-x86"
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