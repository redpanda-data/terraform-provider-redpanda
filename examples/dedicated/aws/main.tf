provider "redpanda" {
}

resource "redpanda_namespace" "test" {
  name = "testname-aws"
}

resource "redpanda_network" "test" {
  name           = "testname-aws"
  namespace_id   = redpanda_namespace.test.id
  cloud_provider = var.cloud_provider
  region         = var.region
  cluster_type   = "dedicated"
  cidr_block     = "10.0.0.0/20"
}

resource "redpanda_cluster" "test" {
  name            = "testname-aws"
  namespace_id    = redpanda_namespace.test.id
  network_id      = redpanda_network.test.id
  cloud_provider  = var.cloud_provider
  region          = var.region
  cluster_type    = "dedicated"
  connection_type = "public"
  throughput_tier = var.throughput_tier
  zones           = var.zones
  allow_deletion  = true
  tags            = {
    // not actually used as API does not consume it yet but we keep it in state for when it does
    "key" = "value"
  }
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