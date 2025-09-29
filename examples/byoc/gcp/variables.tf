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
  default = "tier-1-gcp-um4g"
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