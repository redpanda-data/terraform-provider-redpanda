provider "redpanda" {
  client_id     = var.client_id
  client_secret = var.client_secret
}

resource "redpanda_namespace" "test" {
  name = "testname"
}

resource "redpanda_network" "test" {
  name           = "testname"
  namespace_id   = redpanda_namespace.test.id
  cloud_provider = "gcp"
  region         = "us-central1"
  cluster_type   = "dedicated"
  cidr_block     = "10.0.0.0/20"
}

variable "client_id" {
  type        = string
  description = "client_id"
}

variable "client_secret" {
  type        = string
  description = "client_secret"
}

resource "redpanda_cluster" "test" {
  name            = "testname"
  namespace_id    = redpanda_namespace.test.id
  network_id      = redpanda_network.test.id
  cloud_provider  = "gcp"
  region          = "us-central1"
  cluster_type    = "dedicated"
  connection_type = "public"
  throughput_tier = "tier-2-gcp-um50"
  zones           = [
    "us-central1-a",
  ]
  allow_deletion = true
  tags           = {
    "key" = "value"
  }
}