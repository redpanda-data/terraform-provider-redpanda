// this is only included for acceptance testing purposes, you will almost never want to define just namespace
provider "redpanda" {}

resource "redpanda_namespace" "test" {
  name = var.name
}

variable "name" {
  default = "testname"
}
