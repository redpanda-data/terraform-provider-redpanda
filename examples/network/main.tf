provider "redpanda" {}

resource "redpanda_namespace" "test" {
  name = var.namespace_name
}

resource "redpanda_network" "test" {
  name           = var.network_name
  namespace_id   = redpanda_namespace.test.id
  cloud_provider = var.cloud_provider
  region         = var.region
  cluster_type   = "dedicated"
  cidr_block     = "10.0.0.0/20"
}

variable "namespace_name" {
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