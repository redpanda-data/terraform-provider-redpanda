provider "redpanda" {
  client_id     = var.client_id
  client_secret = var.client_secret
}

resource "redpanda_namespace" "test" {
  name = "testname"
}

variable "client_id" {
  type        = string
  description = "client_id"
}

variable "client_secret" {
  type        = string
  description = "client_secret"
}

