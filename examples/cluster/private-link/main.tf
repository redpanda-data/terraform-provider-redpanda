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

resource "redpanda_cluster" "test" {
  name              = var.cluster_name
  resource_group_id = redpanda_resource_group.test.id
  network_id        = redpanda_network.test.id
  cloud_provider    = var.cloud_provider
  region            = var.region
  cluster_type      = "dedicated"
  connection_type   = "public"
  throughput_tier   = var.throughput_tier
  zones             = var.zones
  allow_deletion    = true
  tags = {
    "key" = "value"
  }
  aws_private_link = {
    enabled         = true
    connect_console = true
    allowed_principals = ["arn:aws:iam::123456789024:root"]
  }
  azure_private_link = {
    enabled         = true
    connect_console = true
    allowed_principals = ["12345678-1234-1234-1234-123456789012"]
  }
  gcp_private_link = {
    enabled         = true
    connect_console = true
    allowed_principals = ["projects/123456789012"]
  }
}

variable "resource_group_name" {
  default = "testname"
}

variable "network_name" {
  default = "testname"
}

variable "cluster_name" {
  default = "testname"
}

variable "region" {
  default = "us-east-1"
}

variable "zones" {
  default = ["use1-az2", "use1-az4", "use1-az6"]
}

variable "cloud_provider" {
  default = "aws"
}

variable "throughput_tier" {
  default = "tier-1-aws-v2-arm"
}