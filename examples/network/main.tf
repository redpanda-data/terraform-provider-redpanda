provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = var.resource_group_name
}

resource "redpanda_network" "test" {
  name              = var.network_name
  resource_group_id = redpanda_resource_group.test.id
  cloud_provider    = var.cloud_provider
  region            = var.region
  cluster_type      = "dedicated"
  cidr_block        = "10.0.0.0/20"
}

variable "resource_group_name" {
  default = "testname"
}

variable "network_name" {
  default = "testname"
}

variable "region" {
  default = "us-east-1"
}

variable "cloud_provider" {
  default = "aws"
}