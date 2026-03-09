variable "resource_group_name" {
  description = "Name of the resource group"
  default     = "role-example-rg"
}

variable "network_name" {
  description = "Name of the network"
  default     = "role-example-network"
}

variable "cluster_name" {
  description = "Name of the cluster"
  default     = "role-example-cluster"
}

variable "cloud_provider" {
  description = "Cloud provider (aws or gcp)"
  default     = "aws"
}

variable "region" {
  description = "Cloud region"
  default     = "us-east-2"
}

variable "zones" {
  description = "Availability zones"
  default     = ["use2-az1", "use2-az2", "use2-az3"]
}

variable "throughput_tier" {
  description = "Cluster throughput tier"
  default     = "tier-1-aws-v2-arm"
}

variable "user_name" {
  description = "Name of the user to create"
  default     = "example-user"
}

variable "topic_name" {
  description = "Name of the topic to create"
  default     = "example-topic"
}

variable "role_name" {
  description = "Name of the role to create"
  default     = "developer"
}

variable "existing_user_principal" {
  description = "OIDC principal of an existing external user to assign the role to (e.g. an email from your identity provider). This user is not managed by Terraform."
  default     = "fulano@redpanda.com"
}
