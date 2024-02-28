provider "redpanda" {}

resource "redpanda_namespace" "test" {
  name = var.namespace_name
}

variable "namespace_name" {
  default = "testname"
}
