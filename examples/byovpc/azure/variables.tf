# Variables
variable "azure_subscription_id" {
  description = "Azure subscription the BYOVNet infrastructure and the cluster are created in"
  type        = string
}

variable "azure_tenant_id" {
  description = "Azure tenant of the subscription"
  type        = string
}

variable "region" {
  description = "Azure region for resources"
  type        = string
  default     = "eastus"
}

variable "resource_name_prefix" {
  description = "Prefix the module puts on every Azure resource name. Storage accounts take it with dashes removed, so keep it short: prefix plus 14 characters must fit the 24-character storage-account limit."
  type        = string
  default     = "rp-"
}

variable "resource_group_name_prefix" {
  description = "Prefix the module puts on every resource group name"
  type        = string
  default     = "rp-"
}

variable "resource_group_name" {
  description = "Redpanda resource group name"
  type        = string
  default     = "testname"
}

variable "network_name" {
  description = "Name for the Redpanda network"
  type        = string
  default     = "testname"
}

variable "cluster_name" {
  description = "Name for the Redpanda cluster"
  type        = string
  default     = "testname"
}

variable "throughput_tier" {
  description = "Throughput tier for the Redpanda cluster"
  type        = string
  default     = "tier-1-azure-v3-x86"
}

variable "zones" {
  description = "Azure zones for the Redpanda cluster"
  type        = list(string)
  default     = ["eastus-az1", "eastus-az2", "eastus-az3"]
}

variable "environment" {
  description = "Environment name (dev, staging, prod)"
  type        = string
  default     = "dev"
}
