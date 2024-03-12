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

resource "redpanda_cluster" "test" {
  name            = var.cluster_name
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
    "key" = "value"
  }
}
variable "cluster_name" {
  default = ""
}
variable "namespace_name" {
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