variable "resource_group_name" {
  default = "testname"
}

variable "network_name" {
  default = "testname"
}

variable "cluster_name" {
  default = "testname"
}

variable "cloud_provider" {
  default = "azure"
}

variable "region" {
  default = "westus2"
}

variable "zones" {
  default = ["westus2-az1", "westus2-az2", "westus2-az3"]
}

variable "throughput_tier" {
  default = "tier-1-azure-v3-x86"
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