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
  default     = "tier-1-gcp-v2-x86"
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

variable "rpsql_enabled" {
  default     = false
  description = "Enable Redpanda SQL. Requires the rpsql_* customer-managed resources to be set."
  type        = bool
}

variable "rpsql_secret_manager_prefix" {
  default     = "redpanda-rpsql"
  description = "GCP Secret Manager prefix for the Redpanda SQL Iceberg catalog credentials. Must match ^[A-Za-z0-9_-]+$ — a bare prefix, not a full resource path."
  type        = string
}

variable "rpsql_api_service_account_email" {
  default     = ""
  description = "Email of the Redpanda SQL API service account. Required when rpsql_enabled."
  type        = string
}

variable "rpsql_service_account_email" {
  default     = ""
  description = "Email of the Redpanda SQL service account. Required when rpsql_enabled."
  type        = string
}

variable "rpsql_cloud_storage_bucket_name" {
  default     = ""
  description = "Name of the GCS bucket backing Redpanda SQL. Required when rpsql_enabled."
  type        = string
}
