provider "redpanda" {
}

resource "redpanda_namespace" "test" {
  name = var.name
}

variable "name" {
  default = "testname"
}
