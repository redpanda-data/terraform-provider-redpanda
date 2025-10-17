# Variables
variable "project_id" {
  description = "The Google Cloud project ID"
  type        = string
}

variable "google_credentials_base64" {
  description = "Base64 encoded Google Cloud credentials"
  type        = string
}

variable "region" {
  description = "GCP region for resources"
  type        = string
  default     = "us-central1"
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
  default     = "tier-1-gcp-um4g"
}

variable "zones" {
  description = "GCP zones for the Redpanda cluster"
  type        = list(string)
  default     = ["us-central1-a", "us-central1-b", "us-central1-c"]
}

variable "environment" {
  description = "Environment name (dev, staging, prod)"
  type        = string
  default     = "dev"
}

variable "user_name" {
  description = "Kafka user name"
  type        = string
  default     = "test-username"
}

variable "user_pw" {
  description = "Kafka user password"
  type        = string
  sensitive   = true
  default     = "password"
}

variable "mechanism" {
  description = "Kafka authentication mechanism"
  type        = string
  default     = "scram-sha-256"
}

variable "topic_name" {
  description = "Kafka topic name"
  type        = string
  default     = "test-topic"
}

variable "partition_count" {
  description = "Number of partitions for the Kafka topic"
  type        = number
  default     = 3
}

variable "replication_factor" {
  description = "Replication factor for the Kafka topic"
  type        = number
  default     = 3
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