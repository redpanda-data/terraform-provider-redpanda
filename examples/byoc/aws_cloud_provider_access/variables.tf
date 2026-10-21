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
  description = "Name prefix of the IAM role Redpanda assumes and of its permissions policies"
  type        = string
  default     = "redpanda-cross-account-"

  # The agent policy lets the role rewrite IAM roles and policies with these
  # names (its AgentRole and AgentAndClusterPolicies statements). Every role
  # and policy name starts with this prefix, so it must neither start with a
  # reserved prefix nor be the start of one.
  validation {
    condition = !anytrue([
      for r in ["redpanda-agent-", "redpanda-cloud-storage-manager-", "redpanda-glue-manager-"] :
      startswith(var.role_name_prefix, r) || startswith(r, var.role_name_prefix)
    ])
    error_message = "role_name_prefix must not overlap redpanda-agent-, redpanda-cloud-storage-manager- or redpanda-glue-manager-. A prefix such as redpanda- names the agent policy redpanda-agent-*, which the role may rewrite."
  }
}

variable "enable_vpc_peering" {
  description = "Grant the permissions Redpanda needs to manage VPC peering connections. Keep it true until the peering connections are deleted."
  type        = bool
  default     = false
}

variable "enable_transit_gateway" {
  description = "Grant the permissions Redpanda needs to manage transit gateway attachments. Keep it true until the attachments are deleted."
  type        = bool
  default     = false
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
