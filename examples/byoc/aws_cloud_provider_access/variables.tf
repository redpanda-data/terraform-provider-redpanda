variable "resource_group_name" {
  default = "testname"
}

variable "network_name" {
  default = "testname"
}

variable "cluster_name" {
  default = "testname"
}

variable "role_name_prefix" {
  description = "Name prefix of the IAM role Redpanda assumes"
  default     = "redpanda-cross-account-"
}

variable "permissions_policy_arn" {
  description = "ARN of the IAM policy granting the permissions Redpanda needs to provision BYOC infrastructure"
  type        = string
}

variable "region" {
  default = "us-east-2"
}

variable "zones" {
  default = ["use2-az1", "use2-az2", "use2-az3"]
}

variable "throughput_tier" {
  default = "tier-1-aws-v2-x86"
}

variable "cluster_allow_deletion" {
  description = "Allow deletion of cluster resource"
  type        = bool
  default     = false
}
