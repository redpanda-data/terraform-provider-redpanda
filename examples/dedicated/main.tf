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
  region         = "us-west1"
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

