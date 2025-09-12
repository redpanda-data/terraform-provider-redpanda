provider "redpanda" {}

resource "redpanda_resource_group" "example" {
  name = "example-resource-group"
}

resource "redpanda_serverless_cluster" "example" {
  name              = "example-serverless-cluster"
  resource_group_id = redpanda_resource_group.example.id
  region            = "us-west-2"
}