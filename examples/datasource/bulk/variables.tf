variable "cluster_id" {
  default = "testname"
}

variable "resource_group_name" {
  default = "tfrp-acc-testbulk-aKtA"
}

variable "network_name" {
  default = "public-network-rate-limit-test"
}

variable "cluster_name" {
  default = "rate-limit-test"
}

variable "region" {
  default = "us-east-2"
}

variable "zones" {
  default = ["use1-az2", "use1-az4", "use1-az6"]
}

variable "cloud_provider" {
  default = "aws"
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