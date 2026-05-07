variable "resource_group_name" {
  type        = string
  description = "Resource group containing both clusters"
}

variable "source_network_name" {
  type        = string
  description = "Network for the source cluster"
}

variable "shadow_network_name" {
  type        = string
  description = "Network for the shadow cluster"
}

variable "source_cluster_name" {
  type        = string
  description = "Source cluster name (data being replicated FROM)"
}

variable "shadow_cluster_name" {
  type        = string
  description = "Shadow cluster name (data being replicated TO; cluster_configuration enables shadow linking)"
}

variable "region" {
  type    = string
  default = "us-east-2"
}

variable "zones" {
  type    = list(string)
  default = ["use2-az1", "use2-az2", "use2-az3"]
}

variable "cloud_provider" {
  type    = string
  default = "aws"
}

variable "throughput_tier" {
  type    = string
  default = "tier-1-aws-v2-arm"
}

variable "cluster_allow_deletion" {
  type        = bool
  default     = false
  description = "allow_deletion=false acts as a canary for cluster-teardown failures (see CLAUDE.md)."
}

variable "user_name" {
  type        = string
  description = "SASL username to create on the source cluster for the shadow link"
}

variable "user_password" {
  type        = string
  sensitive   = true
  description = "SASL password — also stored as the shadow cluster secret"
}

variable "secret_name" {
  type        = string
  description = "Name of the secret in the shadow cluster's secret store (uppercase, A-Z0-9_)"
}

variable "link_name" {
  type        = string
  description = "Name of the shadow link (DNS-1123 subdomain)"
}

variable "metadata_max_age_ms" {
  type        = number
  default     = 10000
  description = "client_options.metadata_max_age_ms; toggled by the partial-update acceptance test step to verify only the changed field-mask path is sent."
}
