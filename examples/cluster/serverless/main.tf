provider "redpanda" {
}
resource "redpanda_resource_group" "test" {
  name = var.resource_group_name
}

resource "redpanda_serverless_cluster" "test" {
  name              = var.cluster_name
  resource_group_id = redpanda_resource_group.test.id
  serverless_region = var.region
}

variable "resource_group_name" {
  default = "testgroup"
}

variable "cluster_name" {
  default = "testname"
}

variable "region" {
  default = "eu-west-1"
}