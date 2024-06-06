provider "redpanda" {}

resource "redpanda_resource_group" "test" {
  name = var.resource_group_name
}

variable "resource_group_name" {
  default = "testname"
}
